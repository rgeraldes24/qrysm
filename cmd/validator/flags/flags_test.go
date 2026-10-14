package flags

import (
	"flag"
	"os"
	"testing"

	"github.com/theQRL/qrysm/cmd"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/urfave/cli/v2"
)

func TestLoadFlagsFromConfig_EnableBuilderHasDefaultValue(t *testing.T) {
	app := cli.App{}
	set := flag.NewFlagSet("test", 0)
	context := cli.NewContext(&app, set, nil)

	require.NoError(t, os.WriteFile("flags_test.yaml", []byte("---\nenable-builder: true"), 0666))

	require.NoError(t, set.Parse([]string{"test-command", "--" + cmd.ConfigFileFlag.Name, "flags_test.yaml"}))
	comFlags := cmd.WrapFlags([]cli.Flag{
		&cli.StringFlag{
			Name: cmd.ConfigFileFlag.Name,
		},
		&cli.BoolFlag{
			Name:  EnableBuilderFlag.Name,
			Value: false,
		},
	})
	command := &cli.Command{
		Name:  "test-command",
		Flags: comFlags,
		Before: func(cliCtx *cli.Context) error {
			return cmd.LoadFlagsFromConfig(cliCtx, comFlags)
		},
		Action: func(cliCtx *cli.Context) error {

			require.Equal(t, true,
				cliCtx.Bool(EnableBuilderFlag.Name))
			return nil
		},
	}
	require.NoError(t, command.Run(context, context.Args().Slice()...))
	require.NoError(t, os.Remove("flags_test.yaml"))
}
