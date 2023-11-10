package flags

import (
	"github.com/urfave/cli/v2"
)

var (
	// InteropMockZond1DataVotesFlag enables mocking the zond1 proof-of-work chain data put into blocks by proposers.
	InteropMockZond1DataVotesFlag = &cli.BoolFlag{
		Name:  "interop-zond1data-votes",
		Usage: "Enable mocking of zond1 data votes for proposers to package into blocks",
	}

	// InteropGenesisTimeFlag specifies genesis time for state generation.
	InteropGenesisTimeFlag = &cli.Uint64Flag{
		Name: "interop-genesis-time",
		Usage: "Specify the genesis time for interop genesis state generation. Must be used with " +
			"--interop-num-validators",
	}
	// InteropNumValidatorsFlag specifies number of genesis validators for state generation.
	InteropNumValidatorsFlag = &cli.Uint64Flag{
		Name:  "interop-num-validators",
		Usage: "Specify number of genesis validators to generate for interop. Must be used with --interop-genesis-time",
	}
)
