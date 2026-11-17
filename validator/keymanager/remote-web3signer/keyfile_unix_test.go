//go:build unix

package remote_web3signer

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

func ownerOf(t *testing.T, path string) (uid, gid int) {
	t.Helper()
	info, err := os.Stat(path)
	require.NoError(t, err)
	st, ok := info.Sys().(*syscall.Stat_t)
	require.Equal(t, true, ok)
	return int(st.Uid), int(st.Gid)
}

// A key file whose group is not the one a new file gets keeps its group when
// the validator client is a member of it.
func TestReplaceKeyFile_KeepsGroup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "remote-keys.txt")
	writeKeyFile(t, path, "# keys")
	uid, gid := ownerOf(t, path)
	groups, err := os.Getgroups()
	require.NoError(t, err)
	other := -1
	for _, g := range groups {
		if g != gid {
			other = g
			break
		}
	}
	if other < 0 {
		t.Skip("the user is a member of a single group")
	}
	require.NoError(t, os.Chown(path, -1, other))

	require.NoError(t, replaceKeyFile(path, []byte("# new keys\n")))
	gotUID, gotGID := ownerOf(t, path)
	assert.Equal(t, uid, gotUID)
	assert.Equal(t, other, gotGID)
}

// Run as root, the validator client keeps the key file's owner as well.
func TestReplaceKeyFile_KeepsOwnerAsRoot(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("only root can give a file another owner")
	}
	path := filepath.Join(t.TempDir(), "remote-keys.txt")
	writeKeyFile(t, path, "# keys")
	require.NoError(t, os.Chown(path, 4242, 4343))

	require.NoError(t, replaceKeyFile(path, []byte("# new keys\n")))
	uid, gid := ownerOf(t, path)
	assert.Equal(t, 4242, uid)
	assert.Equal(t, 4343, gid)
}
