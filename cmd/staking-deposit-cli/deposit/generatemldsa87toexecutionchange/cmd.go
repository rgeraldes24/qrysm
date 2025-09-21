package generatemldsa87toexecutionchange

import (
	"github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/cmd/staking-deposit-cli/stakingdeposit"
	"github.com/urfave/cli/v2"
)

var (
	generateMLDSA87ToExecutionChangeFlags = struct {
		MLDSA87ToExecutionChangesFolder  string
		Chain                            string
		Seed                             string
		SeedPassword                     string
		ValidatorStartIndex              uint64
		ValidatorIndices                 *cli.Uint64Slice
		MLDSA87WithdrawalCredentialsList *cli.StringSlice
		ExecutionAddress                 string
		DevnetChainSetting               string
	}{
		ValidatorIndices:                 cli.NewUint64Slice(),
		MLDSA87WithdrawalCredentialsList: cli.NewStringSlice(),
	}
	log = logrus.WithField("prefix", "deposit")
)

var Commands = []*cli.Command{
	{
		Name:    "generate-ml-dsa-87-to-execution-change",
		Aliases: []string{"generate-execution-change"},
		Usage:   "",
		Action: func(cliCtx *cli.Context) error {
			if err := cliActionGenerateMLDSA87ToExecutionChange(cliCtx); err != nil {
				log.WithError(err).Fatal("Could not generate using an existing seed")
			}
			return nil
		},
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:        "ml-dsa-87-to-execution-changes-folder",
				Usage:       "Folder where the ml-dsa-87 to execution changes files will be created",
				Destination: &generateMLDSA87ToExecutionChangeFlags.MLDSA87ToExecutionChangesFolder,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "chain",
				Usage:       "Name of the chain should be one of these mainnet, betanet",
				Destination: &generateMLDSA87ToExecutionChangeFlags.Chain,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "seed",
				Usage:       "",
				Destination: &generateMLDSA87ToExecutionChangeFlags.Seed,
				Required:    true,
			},
			&cli.Uint64Flag{
				Name:        "validator-start-index",
				Usage:       "",
				Destination: &generateMLDSA87ToExecutionChangeFlags.ValidatorStartIndex,
				Required:    true,
			},
			&cli.Uint64SliceFlag{
				Name:        "validator-indices",
				Usage:       "",
				Destination: generateMLDSA87ToExecutionChangeFlags.ValidatorIndices,
				Required:    true,
			},
			&cli.StringSliceFlag{
				Name:        "ml-dsa-87-withdrawal-credentials-list",
				Usage:       "",
				Destination: generateMLDSA87ToExecutionChangeFlags.MLDSA87WithdrawalCredentialsList,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "execution-address",
				Usage:       "",
				Destination: &generateMLDSA87ToExecutionChangeFlags.ExecutionAddress,
				Required:    true,
			},
			&cli.StringFlag{
				Name:        "devnet-chain-setting",
				Usage:       "Use for devnet only, to set the custom network_name, genesis_fork_name, genesis_validator_root. Input should be in JSON format.",
				Destination: &generateMLDSA87ToExecutionChangeFlags.DevnetChainSetting,
				Value:       "",
			},
		},
	},
}

func cliActionGenerateMLDSA87ToExecutionChange(cliCtx *cli.Context) error {
	// TODO (cyyber): Add flag value validation
	stakingdeposit.GenerateMLDSA87ToExecutionChange(
		generateMLDSA87ToExecutionChangeFlags.MLDSA87ToExecutionChangesFolder,
		generateMLDSA87ToExecutionChangeFlags.Chain,
		generateMLDSA87ToExecutionChangeFlags.Seed,
		generateMLDSA87ToExecutionChangeFlags.ValidatorStartIndex,
		generateMLDSA87ToExecutionChangeFlags.ValidatorIndices.Value(),
		generateMLDSA87ToExecutionChangeFlags.MLDSA87WithdrawalCredentialsList.Value(),
		generateMLDSA87ToExecutionChangeFlags.ExecutionAddress,
		generateMLDSA87ToExecutionChangeFlags.DevnetChainSetting,
	)
	return nil
}
