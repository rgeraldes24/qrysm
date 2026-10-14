package main

import (
	"os/exec"
	"testing"

	"github.com/theQRL/qrysm/testing/require"
)

func TestSelectScenarios(t *testing.T) {
	got, err := selectScenarios(nil)
	require.NoError(t, err)
	require.DeepEqual(t, defaultScenarios, got)

	got, err = selectScenarios([]string{"slasher", "minimal", "slasher"})
	require.NoError(t, err)
	require.DeepEqual(t, []string{"slasher", "minimal"}, got)

	_, err = selectScenarios([]string{"builder"})
	require.ErrorContains(t, `unknown scenario "builder"`, err)
}

func TestScenarioTestsExist(t *testing.T) {
	// Every -run pattern names a test of testing/endtoend.
	for name, sc := range scenarios {
		out, err := exec.Command("go", "test", "-tags=develop,minimal", "-list", sc.run, "../../testing/endtoend").Output()
		require.NoError(t, err, name)
		require.StringContains(t, sc.run[1:len(sc.run)-1], string(out), name)
	}
}
