package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/theQRL/qrysm/testing/require"
)

func TestParseBinary(t *testing.T) {
	b, err := parseBinary("qrysmctl=./cmd/qrysmctl:noMainnetGenesis,other")
	require.NoError(t, err)
	require.DeepEqual(t, binary{name: "qrysmctl", pkg: "./cmd/qrysmctl", tags: []string{"noMainnetGenesis", "other"}}, b)

	b, err = parseBinary("deposit=./cmd/staking-deposit-cli/deposit")
	require.NoError(t, err)
	require.DeepEqual(t, binary{name: "deposit", pkg: "./cmd/staking-deposit-cli/deposit"}, b)

	for _, bad := range []string{"deposit", "=./cmd/x", "x="} {
		_, err := parseBinary(bad)
		require.ErrorContains(t, "want name=package", err)
	}
}

func TestParsePlatforms(t *testing.T) {
	got, err := parsePlatforms(" linux/amd64,darwin/arm64,linux/amd64, ")
	require.NoError(t, err)
	require.DeepEqual(t, []string{"linux/amd64", "darwin/arm64"}, got)

	_, err = parsePlatforms("linux")
	require.ErrorContains(t, "want <os>/<arch>", err)
	_, err = parsePlatforms(",")
	require.ErrorContains(t, "no platform", err)
}

func TestArtifactName(t *testing.T) {
	require.Equal(t, "beacon-chain-v1.2.3-linux-amd64", artifactName("beacon-chain", "v1.2.3", "linux", "amd64"))
	require.Equal(t, "validator-v1.2.3-windows-amd64.exe", artifactName("validator", "v1.2.3", "windows", "amd64"))
}

func TestStamp(t *testing.T) {
	st := stamp{commit: "abc", tag: "v1.2.3", epoch: 1700000000}
	// As hack/workspace_status.sh: date --rfc-3339=seconds --utc.
	require.Equal(t, "2023-11-14 22:13:20+00:00", st.date())
	require.Equal(t, "-s -w"+
		" -X 'github.com/theQRL/qrysm/runtime/version.gitCommit=abc'"+
		" -X 'github.com/theQRL/qrysm/runtime/version.gitTag=v1.2.3'"+
		" -X 'github.com/theQRL/qrysm/runtime/version.buildDate=2023-11-14 22:13:20+00:00'"+
		" -X 'github.com/theQRL/qrysm/runtime/version.buildDateUnix=1700000000'", st.ldflags())
}

func TestWriteSHA256(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bin-v1-linux-amd64")
	require.NoError(t, os.WriteFile(path, []byte("hello\n"), 0o600))
	require.NoError(t, writeSHA256(path))

	got, err := os.ReadFile(path + ".sha256")
	require.NoError(t, err)
	// `sha256sum bin-v1-linux-amd64`
	require.Equal(t, "5891b5b522d5df086d0ff0b110fbd9d21bb4fc7163af34d08286a2e846f6be03  bin-v1-linux-amd64\n", string(got))
}

func TestGitStamp(t *testing.T) {
	t.Setenv("SOURCE_DATE_EPOCH", "1700000000")
	st, err := gitStamp("v9.9.9")
	require.NoError(t, err)
	require.Equal(t, "v9.9.9", st.tag)
	require.Equal(t, int64(1700000000), st.epoch)
	require.Equal(t, 40, len(st.commit))

	t.Setenv("SOURCE_DATE_EPOCH", "soon")
	_, err = gitStamp("v9.9.9")
	require.ErrorContains(t, "SOURCE_DATE_EPOCH", err)
}
