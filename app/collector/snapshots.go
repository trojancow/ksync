package collector

import (
	"encoding/json"
	"fmt"
	"github.com/KYVENetwork/ksync/types"
	"github.com/KYVENetwork/ksync/utils"
	"strconv"
	"strings"
	"time"
)

type KyveSnapshotCollector struct {
	poolId    int64
	chainRest string

	earliestAvailableHeight int64
	latestAvailableHeight   int64
	interval                int64
	totalBundles            int64
}

func NewKyveSnapshotCollector(poolId int64, chainRest string) (*KyveSnapshotCollector, error) {
	poolResponse, err := utils.GetPool(chainRest, poolId)
	if err != nil {
		return nil, fmt.Errorf("fail to get pool with id %d: %w", poolId, err)
	}

	if poolResponse.Pool.Data.Runtime != utils.RuntimeTendermintSsync {
		return nil, fmt.Errorf("found invalid runtime on snapshot pool %d: Expected = %s Found = %s", poolId, utils.RuntimeTendermintSsync, poolResponse.Pool.Data.Runtime)
	}

	var config types.TendermintSSyncConfig
	if err := json.Unmarshal([]byte(poolResponse.Pool.Data.Config), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot pool config %s: %w", poolResponse.Pool.Data.Config, err)
	}

	startHeight, _, err := utils.ParseSnapshotFromKey(poolResponse.Pool.Data.StartKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse height %s from start key: %w", poolResponse.Pool.Data.StartKey, err)
	}

	currentHeight, chunkIndex, err := utils.ParseSnapshotFromKey(poolResponse.Pool.Data.CurrentKey)
	if err != nil {
		return nil, fmt.Errorf("failed to parse height %s from current key: %w", poolResponse.Pool.Data.CurrentKey, err)
	}

	// we expect that the current snapshot is not complete yet and that the latest
	// available snapshot is the one before it, to get the height of that we simply
	// go back the snapshot interval from the current height
	latestAvailableHeight := currentHeight - config.Interval

	// if however the last chunk of the current snapshot has already been archived the latest available
	// snapshot height is indeed the current one. We can check this if the new bundle summary
	// format "height/format/chunkIndex/totalChunks" is available and by comparing the total number of chunks
	// in the snapshot with our current chunk index. We just skip if parsing the number of total chunks
	// fails
	if summary := strings.Split(poolResponse.Pool.Data.CurrentSummary, "/"); len(summary) == 4 {
		if totalChunks, err := strconv.ParseInt(summary[3], 10, 64); err == nil && totalChunks == chunkIndex+1 {
			latestAvailableHeight = currentHeight
		}
	}

	return &KyveSnapshotCollector{
		poolId:                  poolId,
		chainRest:               chainRest,
		earliestAvailableHeight: startHeight,
		latestAvailableHeight:   latestAvailableHeight,
		interval:                config.Interval,
		totalBundles:            poolResponse.Pool.Data.TotalBundles,
	}, nil
}

func (collector *KyveSnapshotCollector) GetEarliestAvailableHeight() int64 {
	return collector.earliestAvailableHeight
}

func (collector *KyveSnapshotCollector) GetLatestAvailableHeight() int64 {
	return collector.latestAvailableHeight
}

func (collector *KyveSnapshotCollector) GetInterval() int64 {
	return collector.interval
}

func (collector *KyveSnapshotCollector) GetSnapshotHeight(targetHeight int64, isServeSnapshot bool) int64 {
	// if no target height was given the snapshot height is the latest available,
	// also if the target height is greater than the latest available height
	if targetHeight == 0 || targetHeight > collector.latestAvailableHeight {
		// if we run the serve-snapshot command we actually do not want to sync to the latest available height
		// or else the node operator has to wait until the next snapshot is created in order to join the pool.
		if isServeSnapshot && collector.latestAvailableHeight > collector.interval {
			return collector.latestAvailableHeight - collector.interval
		}
		return collector.latestAvailableHeight
	}

	// get the nearest snapshot height on the interval below the given target height
	// by subtracting the modulo remainder
	return targetHeight - (targetHeight % collector.interval)
}

func (collector *KyveSnapshotCollector) GetCurrentHeight() (int64, error) {
	poolResponse, err := utils.GetPool(collector.chainRest, collector.poolId)
	if err != nil {
		return 0, fmt.Errorf("fail to get pool with id %d: %w", collector.poolId, err)
	}

	currentHeight, _, err := utils.ParseSnapshotFromKey(poolResponse.Pool.Data.CurrentKey)
	if err != nil {
		return 0, fmt.Errorf("failed to parse height %s from current key: %w", poolResponse.Pool.Data.CurrentKey, err)
	}

	return currentHeight, nil
}

func (collector *KyveSnapshotCollector) GetSnapshotFromBundleId(bundleId int64) (*types.SnapshotDataItem, error) {
	chunkBundleFinalized, err := utils.GetFinalizedBundleById(collector.chainRest, collector.poolId, bundleId)
	if err != nil {
		return nil, fmt.Errorf("failed getting finalized bundle by id %d: %w", bundleId, err)
	}

	data, err := utils.GetDataFromFinalizedBundle(*chunkBundleFinalized)
	if err != nil {
		return nil, fmt.Errorf("failed getting data from finalized bundle: %w", err)
	}

	var bundle types.SnapshotBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot bundle: %w", err)
	}

	if len(bundle) != 1 {
		return nil, fmt.Errorf("found multiple bundles in snapshot bundle")
	}

	return &bundle[0], nil
}

func (collector *KyveSnapshotCollector) DownloadChunkFromBundleId(bundleId int64) ([]byte, error) {
	chunkBundleFinalized, err := utils.GetFinalizedBundleById(collector.chainRest, collector.poolId, bundleId)
	if err != nil {
		return nil, fmt.Errorf("failed getting finalized bundle by id %d: %w", bundleId, err)
	}

	data, err := utils.GetDataFromFinalizedBundle(*chunkBundleFinalized)
	if err != nil {
		return nil, fmt.Errorf("failed getting data from finalized bundle: %w", err)
	}

	var bundle types.SnapshotBundle
	if err := json.Unmarshal(data, &bundle); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snapshot bundle: %w", err)
	}

	if len(bundle) != 1 {
		return nil, fmt.Errorf("found multiple bundles in snapshot bundle")
	}

	return bundle[0].Value.Chunk, nil
}

func (collector *KyveSnapshotCollector) FindSnapshotBundleIdForHeight(height int64) (int64, error) {
	latestBundleId := collector.totalBundles - 1

	// if the height is the latest height we can calculate the location of bundle id for the first
	// chunk immediately
	if height == collector.latestAvailableHeight {
		finalizedBundle, err := utils.GetFinalizedBundleById(collector.chainRest, collector.poolId, latestBundleId)
		if err != nil {
			return 0, fmt.Errorf("failed to get finalized bundle with id %d: %w", latestBundleId, err)
		}

		h, chunkIndex, err := utils.ParseSnapshotFromKey(finalizedBundle.ToKey)
		if err != nil {
			return 0, fmt.Errorf("failed to parse snapshot key %s: %w", finalizedBundle.ToKey, err)
		}

		if h == height {
			return latestBundleId - chunkIndex, nil
		}
	}

	// if the height is not the latest height we try to find it with binary search
	// TODO: consider interpolation search
	low := int64(0)
	high := latestBundleId

	// stop when low and high meet
	for low <= high {
		// check in the middle
		mid := (low + high) / 2

		finalizedBundle, err := utils.GetFinalizedBundleById(collector.chainRest, collector.poolId, mid)
		if err != nil {
			return 0, fmt.Errorf("failed to get finalized bundle with id %d: %w", mid, err)
		}

		h, chunkIndex, err := utils.ParseSnapshotFromKey(finalizedBundle.ToKey)
		if err != nil {
			return 0, fmt.Errorf("failed to parse snapshot key %s: %w", finalizedBundle.ToKey, err)
		}

		if h < height {
			// target height is in the right half
			low = mid + 1
		} else if h > height {
			// target height is in the left half
			high = mid - 1
		} else {
			// found it, now we just go back to the bundle where the first chunk index
			// is located
			return mid - chunkIndex, nil
		}

		time.Sleep(utils.RequestTimeoutMS)
	}

	return 0, fmt.Errorf("failed to find snapshot bundle id for height %d", height)
}
