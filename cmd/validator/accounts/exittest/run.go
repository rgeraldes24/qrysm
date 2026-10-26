package exittest

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/config/features"
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/testing/require"
	"github.com/urfave/cli/v2"
)

// WriteChainConfig writes a chain config file, in qrysm's format, for a
// network with mainnet's values except its name and genesis fork version, and
// returns its path.
func WriteChainConfig(t testing.TB, forkVersion []byte) string {
	t.Helper()
	cfg := params.MainnetConfig().Copy()
	cfg.ConfigName = "exittest"
	cfg.GenesisForkVersion = forkVersion
	path := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(path, params.ConfigToYaml(cfg), 0o600))
	return path
}

// RunCommand runs app with args, before hooks included, and returns the error
// app.Run returned or, when the command's action logged one at fatal level,
// that error. The fatal log does not exit the test binary. The beacon config
// and feature flags the command sets are restored when the test ends, and the
// command runs in a temporary working directory, where it may record that the
// terms of use were accepted.
func RunCommand(t *testing.T, app *cli.App, args []string) error {
	t.Helper()
	params.SetupTestConfigCleanup(t)
	t.Cleanup(features.InitWithReset(&features.Flags{}))
	t.Chdir(t.TempDir())

	var fatal error
	logger := logrus.StandardLogger()
	previous := logger.ReplaceHooks(make(logrus.LevelHooks))
	hooks := make(logrus.LevelHooks)
	for level, h := range previous {
		hooks[level] = append([]logrus.Hook(nil), h...)
	}
	hooks.Add(fatalHook(func(entry *logrus.Entry) {
		if err, ok := entry.Data[logrus.ErrorKey].(error); ok && fatal == nil {
			fatal = err
		}
	}))
	logger.ReplaceHooks(hooks)
	exit := logger.ExitFunc
	logger.ExitFunc = func(int) {}
	defer func() {
		logger.ExitFunc = exit
		logger.ReplaceHooks(previous)
	}()

	if err := app.Run(args); err != nil {
		return err
	}
	return fatal
}

// fatalHook calls itself with every entry logged at fatal level.
type fatalHook func(*logrus.Entry)

func (fatalHook) Levels() []logrus.Level {
	return []logrus.Level{logrus.FatalLevel}
}

func (h fatalHook) Fire(entry *logrus.Entry) error {
	h(entry)
	return nil
}
