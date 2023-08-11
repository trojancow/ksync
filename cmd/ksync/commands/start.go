package commands

import (
	"fmt"
	"github.com/KYVENetwork/ksync/executor/auto"
	"github.com/KYVENetwork/ksync/executor/db"
	"github.com/KYVENetwork/ksync/executor/p2p"
	"github.com/KYVENetwork/ksync/utils"
	"github.com/spf13/cobra"
	"strings"
)

var (
	daemonPath   string
	flags        string
	mode         string
	home         string
	poolId       int64
	seeds        string
	targetHeight int64
	chainId      string
	restEndpoint string

	quitCh = make(chan int)
)

func init() {
	startCmd.Flags().StringVar(&mode, "mode", utils.DefaultMode, fmt.Sprintf("sync mode (\"auto\",\"db\",\"p2p\"), [default = %s]", utils.DefaultMode))

	startCmd.Flags().StringVar(&home, "home", "", "home directory")
	if err := startCmd.MarkFlagRequired("home"); err != nil {
		panic(fmt.Errorf("flag 'home' should be required: %w", err))
	}

	// Optional AUTO-MODE flags.
	startCmd.Flags().StringVar(&daemonPath, "daemon-path", "", "daemon path of node to be synced")

	startCmd.Flags().StringVar(&chainId, "chain-id", utils.DefaultChainId, fmt.Sprintf("kyve chain id (\"kyve-1\",\"kaon-1\",\"korellia\"), [default = %s]", utils.DefaultChainId))

	startCmd.Flags().Int64Var(&poolId, "pool-id", 0, "pool id")
	if err := startCmd.MarkFlagRequired("pool-id"); err != nil {
		panic(fmt.Errorf("flag 'pool-id' should be required: %w", err))
	}

	startCmd.Flags().StringVar(&restEndpoint, "rest-endpoint", "", "Overwrite default rest endpoint from chain")

	startCmd.Flags().Int64Var(&targetHeight, "target-height", 0, "target height (including)")

	startCmd.Flags().StringVar(&seeds, "seeds", "", "P2P seeds to continue syncing process after KSYNC")

	startCmd.Flags().StringVar(&flags, "flags", "", "Flags for starting the node to be synced; excluding --home and --with-tendermint")

	rootCmd.AddCommand(startCmd)
}

var startCmd = &cobra.Command{
	Use:   "start",
	Short: "Start fast syncing blocks with KSYNC",
	Run: func(cmd *cobra.Command, args []string) {
		// if no custom rest endpoint was given we take it from the chainId
		if restEndpoint == "" {
			switch chainId {
			case "kyve-1":
				restEndpoint = utils.RestEndpointMainnet
			case "kaon-1":
				restEndpoint = utils.RestEndpointKaon
			case "korellia":
				restEndpoint = utils.RestEndpointKorellia
			default:
				panic("flag --chain-id has to be either \"kyve-1\", \"kaon-1\" or \"korellia\"")
			}
		}

		// trim trailing slash
		restEndpoint = strings.TrimSuffix(restEndpoint, "/")

		// start block executor based on sync mode
		switch mode {
		case "auto":
			if daemonPath == "" {
				panic("flag --daemon-path is required for mode \"auto\"")
			}
			auto.StartAutoExecutor(quitCh, home, daemonPath, seeds, flags, poolId, restEndpoint, targetHeight)
		case "db":
			go db.StartDBExecutor(quitCh, home, poolId, restEndpoint, targetHeight)
		case "p2p":
			go p2p.StartP2PExecutor(quitCh, home, poolId, restEndpoint, targetHeight)
		default:
			panic("flag --mode has to be either \"auto\", \"db\" or \"p2p\"")
		}

		// only exit process if executor has finished
		<-quitCh
	},
}
