package cmd

import (
	"flag"
	"testing"

	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/urfave/cli/v2"
)

func TestOverrideConfig(t *testing.T) {
	cfg := &Flags{
		MaxRPCPageSize: 123,
	}
	reset := InitWithReset(cfg)
	defer reset()
	c := Get()
	assert.Equal(t, 123, c.MaxRPCPageSize)
}

func TestDefaultConfig(t *testing.T) {
	cfg := &Flags{
		MaxRPCPageSize: params.BeaconConfig().DefaultPageSize,
	}
	c := Get()
	assert.DeepEqual(t, c, cfg)

	reset := InitWithReset(cfg)
	defer reset()
	c = Get()
	assert.DeepEqual(t, c, cfg)
}

func TestConfigureBeaconConfig(t *testing.T) {
	params.SetupTestConfigCleanup(t)
	app := cli.App{}
	set := flag.NewFlagSet("test", 0)
	set.Bool(MinimalConfigFlag.Name, true, "test")
	context := cli.NewContext(&app, set, nil)
	require.NoError(t, ConfigureBeaconChain(context))
	assert.Equal(t, params.MinimalName, params.BeaconConfig().ConfigName)
}
