package commands

import (
	"fmt"
	"github.com/KYVENetwork/ksync/servesnapshots"
	"github.com/KYVENetwork/ksync/utils"
	"github.com/spf13/cobra"
	"strings"
)

func init() {
	serveCmd.Flags().StringVar(&binaryPath, "binary", "", "binary path of node to be synced")
	if err := serveCmd.MarkFlagRequired("binary"); err != nil {
		panic(fmt.Errorf("flag 'binary' should be required: %w", err))
	}

	serveCmd.Flags().StringVar(&homePath, "home", "", "home directory")
	if err := serveCmd.MarkFlagRequired("home"); err != nil {
		panic(fmt.Errorf("flag 'home' should be required: %w", err))
	}

	serveCmd.Flags().StringVar(&chainId, "chain-id", utils.DefaultChainId, fmt.Sprintf("KYVE chain id [\"%s\",\"%s\",\"%s\"]", utils.ChainIdMainnet, utils.ChainIdKaon, utils.ChainIdKorellia))

	serveCmd.Flags().StringVar(&chainRest, "chain-rest", "", "rest endpoint for KYVE chain")
	serveCmd.Flags().StringVar(&storageRest, "storage-rest", "", "storage endpoint for requesting bundle data")

	serveCmd.Flags().Int64Var(&blockPoolId, "block-pool-id", 0, "pool id of the block-sync pool")
	if err := serveCmd.MarkFlagRequired("block-pool-id"); err != nil {
		panic(fmt.Errorf("flag 'block-pool-id' should be required: %w", err))
	}

	serveCmd.Flags().Int64Var(&snapshotPoolId, "snapshot-pool-id", 0, "pool id of the state-sync pool")
	if err := serveCmd.MarkFlagRequired("snapshot-pool-id"); err != nil {
		panic(fmt.Errorf("flag 'snapshot-pool-id' should be required: %w", err))
	}

	serveCmd.Flags().Int64Var(&snapshotPort, "snapshot-port", utils.DefaultSnapshotServerPort, "port for snapshot server")

	serveCmd.Flags().BoolVar(&metrics, "metrics", false, "metrics server exposing sync status")
	serveCmd.Flags().Int64Var(&metricsPort, "metrics-port", utils.DefaultMetricsServerPort, "port for metrics server")

	serveCmd.Flags().Int64Var(&startHeight, "start-height", 0, "start creating snapshots at this height. note that pruning should be false when using start height")

	serveCmd.Flags().BoolVar(&pruning, "pruning", true, "prune application, state and blockstore db")

	rootCmd.AddCommand(serveCmd)
}

var serveCmd = &cobra.Command{
	Use:   "serve-snapshots",
	Short: "Serve snapshots for running KYVE state-sync pools",
	Run: func(cmd *cobra.Command, args []string) {
		chainRest = utils.GetChainRest(chainId, chainRest)
		storageRest = strings.TrimSuffix(storageRest, "/")
		servesnapshots.StartServeSnapshotsWithBinary(binaryPath, homePath, chainRest, storageRest, blockPoolId, metrics, metricsPort, snapshotPoolId, snapshotPort, startHeight, pruning)
	},
}
