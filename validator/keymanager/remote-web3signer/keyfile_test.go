package remote_web3signer

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common/hexutil"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/runtime/interop"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

// testKeys returns n distinct public keys, as raw bytes and in canonical hex.
func testKeys(t *testing.T, n int) ([][field_params.MLDSA87PubkeyLength]byte, []string) {
	t.Helper()
	keys := make([][field_params.MLDSA87PubkeyLength]byte, n)
	hexKeys := make([]string, n)
	for i := range keys {
		for j := range keys[i] {
			keys[i][j] = byte(i + j)
		}
		hexKeys[i] = hexutil.Encode(keys[i][:])
	}
	return keys, hexKeys
}

func writeKeyFile(t *testing.T, path string, lines ...string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o600))
}

func TestParseKeyFile(t *testing.T) {
	keys, hexKeys := testKeys(t, 2)
	tests := []struct {
		name    string
		content string
		want    [][field_params.MLDSA87PubkeyLength]byte
		wantErr string
	}{
		{
			name:    "empty file",
			content: "",
			want:    [][field_params.MLDSA87PubkeyLength]byte{},
		},
		{
			name:    "one key per line",
			content: hexKeys[0] + "\n" + hexKeys[1] + "\n",
			want:    keys,
		},
		{
			name: "comments, blank lines, whitespace and CRLF",
			content: "# validators of node 0\r\n\r\n  " + hexKeys[0] + "  \r\n" +
				hexKeys[1] + " # second key\r\n# trailing comment",
			want: keys,
		},
		{
			name:    "upper-case hex",
			content: "0x" + strings.ToUpper(hexKeys[0][2:]),
			want:    keys[:1],
		},
		{
			name:    "missing 0x prefix",
			content: hexKeys[0][2:],
			wantErr: "line 1 is not a 0x-prefixed public key",
		},
		{
			name:    "not hex",
			content: hexKeys[0] + "\n0xzz",
			wantErr: "line 2 is not a 0x-prefixed public key",
		},
		{
			name:    "wrong length",
			content: "\n\n" + hexKeys[0][:len(hexKeys[0])-2],
			wantErr: "line 3 is not a 0x-prefixed public key: public key is 2591 bytes, want 2592",
		},
		{
			name:    "duplicate key in another case",
			content: hexKeys[0] + "\n" + hexKeys[1] + "\n" + "0x" + strings.ToUpper(hexKeys[0][2:]),
			wantErr: "line 3 repeats the public key on line 1",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseKeyFile([]byte(tt.content))
			if tt.wantErr != "" {
				require.ErrorContains(t, tt.wantErr, err)
				assert.Equal(t, false, strings.Contains(err.Error(), hexKeys[0][2:40]), "error must not quote the key")
				return
			}
			require.NoError(t, err)
			require.DeepEqual(t, tt.want, got)
		})
	}
}

func TestKeyFileLineEdits(t *testing.T) {
	keys, hexKeys := testKeys(t, 4)
	original := "# node 0\n" + hexKeys[0] + "\n\n" + hexKeys[1] + " # second key\r\n" + hexKeys[2]

	t.Run("append", func(t *testing.T) {
		got := appendKeyFileLines([]byte(original), keys[3:])
		assert.Equal(t, original+"\n"+hexKeys[3]+"\n", string(got))
		parsed, err := ParseKeyFile(got)
		require.NoError(t, err)
		require.DeepEqual(t, keys, parsed)

		got = appendKeyFileLines(nil, keys[:2])
		assert.Equal(t, hexKeys[0]+"\n"+hexKeys[1]+"\n", string(got))
	})
	t.Run("remove", func(t *testing.T) {
		got := removeKeyFileLines([]byte(original), map[[field_params.MLDSA87PubkeyLength]byte]bool{
			keys[1]: true,
			keys[2]: true,
			keys[3]: true, // Not in the file.
		})
		assert.Equal(t, "# node 0\n"+hexKeys[0]+"\n\n", string(got))

		upper := "0x" + strings.ToUpper(hexKeys[0][2:])
		got = removeKeyFileLines([]byte(upper+"\n"+hexKeys[1]), map[[field_params.MLDSA87PubkeyLength]byte]bool{keys[0]: true})
		assert.Equal(t, hexKeys[1], string(got))
	})
}

// TestReplaceKeyFile_ReaderNeverSeesPartialFile reads the key file while it
// is replaced over and over: every read must return one of the two complete
// versions.
func TestReplaceKeyFile_ReaderNeverSeesPartialFile(t *testing.T) {
	_, hexKeys := testKeys(t, 200)
	versions := [][]byte{
		[]byte(strings.Join(hexKeys[:100], "\n") + "\n"),
		[]byte(strings.Join(hexKeys[100:], "\n") + "\n"),
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "remote-keys.txt")
	require.NoError(t, os.WriteFile(path, versions[0], 0o600))

	done := make(chan struct{})
	readerErr := make(chan error, 1)
	reads := 0
	go func() {
		defer close(readerErr)
		for {
			select {
			case <-done:
				return
			default:
			}
			data, err := os.ReadFile(path)
			if err != nil {
				readerErr <- err
				return
			}
			if !bytes.Equal(data, versions[0]) && !bytes.Equal(data, versions[1]) {
				readerErr <- fmt.Errorf("read a partial key file of %d bytes", len(data))
				return
			}
			reads++
		}
	}()
	for i := range 20 {
		require.NoError(t, replaceKeyFile(path, versions[(i+1)%2]))
	}
	close(done)
	require.NoError(t, <-readerErr)
	assert.Equal(t, true, reads > 0)

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Equal(t, 1, len(entries), "no temporary file is left behind")
}

func TestReplaceKeyFile_FailureKeepsOldFile(t *testing.T) {
	_, hexKeys := testKeys(t, 2)
	old := []byte(hexKeys[0] + "\n")

	t.Run("rename fails", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, "remote-keys.txt")
		require.NoError(t, os.WriteFile(path, old, 0o600))
		renameFile = func(string, string) error { return errors.New("injected rename failure") }
		t.Cleanup(func() { renameFile = os.Rename })

		err := replaceKeyFile(path, []byte(hexKeys[1]+"\n"))
		require.ErrorContains(t, "injected rename failure", err)
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.DeepEqual(t, old, got)
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Equal(t, 1, len(entries), "the temporary file is removed")
	})
	t.Run("directory not writable", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root can write to a read-only directory")
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "remote-keys.txt")
		require.NoError(t, os.WriteFile(path, old, 0o600))
		require.NoError(t, os.Chmod(dir, 0o500))
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

		require.NotNil(t, replaceKeyFile(path, []byte(hexKeys[1]+"\n")))
		got, err := os.ReadFile(path)
		require.NoError(t, err)
		require.DeepEqual(t, old, got)
	})
	t.Run("missing file", func(t *testing.T) {
		dir := t.TempDir()
		require.NotNil(t, replaceKeyFile(filepath.Join(dir, "remote-keys.txt"), old))
		entries, err := os.ReadDir(dir)
		require.NoError(t, err)
		assert.Equal(t, 0, len(entries))
	})
}

func TestReplaceKeyFile_KeepsModeAndSymlink(t *testing.T) {
	_, hexKeys := testKeys(t, 2)
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "keys.txt")
	require.NoError(t, os.WriteFile(target, []byte(hexKeys[0]+"\n"), 0o600))
	require.NoError(t, os.Chmod(target, 0o640))
	link := filepath.Join(t.TempDir(), "remote-keys.txt")
	require.NoError(t, os.Symlink(target, link))

	require.NoError(t, replaceKeyFile(link, []byte(hexKeys[1]+"\n")))
	linkInfo, err := os.Lstat(link)
	require.NoError(t, err)
	assert.Equal(t, os.ModeSymlink, linkInfo.Mode()&os.ModeSymlink, "the link stays a link")
	info, err := os.Stat(target)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0o640), info.Mode().Perm())
	got, err := os.ReadFile(link)
	require.NoError(t, err)
	assert.Equal(t, hexKeys[1]+"\n", string(got))
}

// The e2e gives each validator client 128 keys through a key file, which is
// beyond what a comma list on the command line can carry.
func TestKeymanager_KeyFileWith128CanonicalKeys(t *testing.T) {
	_, pubKeys, err := interop.DeterministicallyGenerateKeys(0, 128)
	require.NoError(t, err)
	want := make([][field_params.MLDSA87PubkeyLength]byte, len(pubKeys))
	lines := make([]string, len(pubKeys))
	for i, pk := range pubKeys {
		copy(want[i][:], pk.Marshal())
		lines[i] = hexutil.Encode(pk.Marshal())
	}
	path := filepath.Join(t.TempDir(), "remote-keys.txt")
	writeKeyFile(t, path, lines...)

	km, err := NewKeymanager(context.Background(), &SetupConfig{
		BaseEndpoint:          "http://localhost:9000",
		GenesisValidatorsRoot: bytes.Repeat([]byte{0x01}, 32),
		KeyFile:               path,
	})
	require.NoError(t, err)
	got, err := km.FetchValidatingPublicKeys(context.Background())
	require.NoError(t, err)
	require.Equal(t, 128, len(got))
	require.DeepEqual(t, want, got)
}

func TestNewKeymanager_KeyFileErrors(t *testing.T) {
	keys, hexKeys := testKeys(t, 1)
	dir := t.TempDir()
	good := filepath.Join(dir, "keys.txt")
	writeKeyFile(t, good, hexKeys[0])
	bad := filepath.Join(dir, "bad.txt")
	writeKeyFile(t, bad, hexKeys[0], hexKeys[0])

	tests := []struct {
		name    string
		cfg     SetupConfig
		wantErr string
	}{
		{
			name:    "missing file",
			cfg:     SetupConfig{KeyFile: filepath.Join(dir, "missing.txt")},
			wantErr: "could not read remote signer key file",
		},
		{
			name:    "duplicate keys",
			cfg:     SetupConfig{KeyFile: bad},
			wantErr: "line 2 repeats the public key on line 1",
		},
		{
			name:    "with a public keys url",
			cfg:     SetupConfig{KeyFile: good, PublicKeysURL: "http://localhost:9000/api/v1/consensus/publicKeys"},
			wantErr: "a key file cannot be combined with a public keys url or list",
		},
		{
			name:    "with a key list",
			cfg:     SetupConfig{KeyFile: good, ProvidedPublicKeys: keys},
			wantErr: "a key file cannot be combined with a public keys url or list",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			cfg.BaseEndpoint = "http://localhost:9000"
			cfg.GenesisValidatorsRoot = bytes.Repeat([]byte{0x01}, 32)
			_, err := NewKeymanager(context.Background(), &cfg)
			require.ErrorContains(t, tt.wantErr, err)
		})
	}
}

func TestKeymanager_KeyFileReload(t *testing.T) {
	old := keyFileReloadDebounce
	keyFileReloadDebounce = 50 * time.Millisecond
	t.Cleanup(func() { keyFileReloadDebounce = old })

	keys, hexKeys := testKeys(t, 3)
	dir := t.TempDir()
	path := filepath.Join(dir, "remote-keys.txt")
	writeKeyFile(t, path, hexKeys[0])

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	km, err := NewKeymanager(ctx, &SetupConfig{
		BaseEndpoint:          "http://localhost:9000",
		GenesisValidatorsRoot: bytes.Repeat([]byte{0x01}, 32),
		KeyFile:               path,
		ListenForChanges:      true,
	})
	require.NoError(t, err)
	changes := make(chan [][field_params.MLDSA87PubkeyLength]byte, 1)
	sub := km.SubscribeAccountChanges(changes)
	defer sub.Unsubscribe()

	expectKeys := func(want [][field_params.MLDSA87PubkeyLength]byte) {
		t.Helper()
		select {
		case got := <-changes:
			require.DeepEqual(t, want, got)
		case <-time.After(5 * time.Second):
			t.Fatal("key file change was not picked up")
		}
		got, err := km.FetchValidatingPublicKeys(ctx)
		require.NoError(t, err)
		require.DeepEqual(t, want, got)
	}
	expectNoChange := func(want [][field_params.MLDSA87PubkeyLength]byte) {
		t.Helper()
		select {
		case got := <-changes:
			t.Fatalf("unexpected key change to %d keys", len(got))
		case <-time.After(300 * time.Millisecond):
		}
		got, err := km.FetchValidatingPublicKeys(ctx)
		require.NoError(t, err)
		require.DeepEqual(t, want, got)
	}

	// Replaced atomically, as an editor or the keymanager API would.
	tmp := filepath.Join(dir, "remote-keys.txt.tmp")
	writeKeyFile(t, tmp, hexKeys[0], hexKeys[1])
	require.NoError(t, os.Rename(tmp, path))
	expectKeys(keys[:2])

	// An invalid file keeps the current keys.
	writeKeyFile(t, path, hexKeys[0], "0x1234")
	expectNoChange(keys[:2])

	// Rewritten in place.
	writeKeyFile(t, path, hexKeys[2])
	expectKeys(keys[2:])

	// Unrelated files in the directory are ignored.
	writeKeyFile(t, filepath.Join(dir, "other.txt"), hexKeys[0])
	expectNoChange(keys[2:])
}

func TestKeymanager_CloseStopsKeyFileReload(t *testing.T) {
	old := keyFileReloadDebounce
	keyFileReloadDebounce = 50 * time.Millisecond
	t.Cleanup(func() { keyFileReloadDebounce = old })

	keys, hexKeys := testKeys(t, 2)
	path := filepath.Join(t.TempDir(), "remote-keys.txt")
	writeKeyFile(t, path, hexKeys[0])
	km, err := NewKeymanager(context.Background(), &SetupConfig{
		BaseEndpoint:          "http://localhost:9000",
		GenesisValidatorsRoot: bytes.Repeat([]byte{0x01}, 32),
		KeyFile:               path,
		ListenForChanges:      true,
	})
	require.NoError(t, err)

	km.Close()
	km.Close() // A second Close is a no-op.
	writeKeyFile(t, path, hexKeys[1])
	time.Sleep(300 * time.Millisecond)
	got, err := km.FetchValidatingPublicKeys(context.Background())
	require.NoError(t, err)
	require.DeepEqual(t, keys[:1], got)
}

func TestKeymanager_CloseWithoutKeyFile(t *testing.T) {
	km, err := NewKeymanager(context.Background(), &SetupConfig{
		BaseEndpoint:          "http://localhost:9000",
		GenesisValidatorsRoot: bytes.Repeat([]byte{0x01}, 32),
	})
	require.NoError(t, err)
	km.Close()
}
