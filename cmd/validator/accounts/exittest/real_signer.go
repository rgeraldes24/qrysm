package exittest

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/theQRL/go-qrl/common/hexutil"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/testing/require"
)

// RealRemoteSignerBinary returns the go-remotesigner binary that the
// REMOTESIGNER_BINARY environment variable names, for example the one
// `make build-consensus` builds in a go-remotesigner checkout, and skips the
// test when the variable is not set.
func RealRemoteSignerBinary(t *testing.T) string {
	t.Helper()
	binary := os.Getenv("REMOTESIGNER_BINARY")
	if binary == "" {
		t.Skip("REMOTESIGNER_BINARY is not set")
	}
	return binary
}

// NewSeededKey returns a random ML-DSA-87 seed and the key it derives, for a
// real signer's file-raw key.
func NewSeededKey(t testing.TB) ([]byte, ml_dsa_87.MLDSA87Key) {
	t.Helper()
	seed := make([]byte, field_params.MLDSA87SeedLength)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	key, err := ml_dsa_87.SecretKeyFromSeed(seed)
	require.NoError(t, err)
	return seed, key
}

// StartRealRemoteSigner starts the signer binary on a loopback port with one
// file-raw key holding seed, on the network of the chain config file network
// with genesisTime and GenesisValidatorsRoot, signing voluntary exits only when
// exitsEnabled, and waits until it is ready. It returns the signer URL and the
// sign-scope token file the signer generated.
func StartRealRemoteSigner(
	t *testing.T, binary string, seed []byte, network string, genesisTime time.Time, exitsEnabled bool,
) (string, string) {
	t.Helper()
	data, keys := t.TempDir(), t.TempDir()
	keyFile := fmt.Sprintf("type: file-raw\nkeyType: ML_DSA_87\nprivateKey: \"%s\"\n", hexutil.Encode(seed))
	require.NoError(t, os.WriteFile(filepath.Join(keys, "validator.yaml"), []byte(keyFile), 0o600))
	args := []string{
		"--data-path=" + data,
		"--key-store-path=" + keys,
		"--http-listen-host=127.0.0.1",
		"--http-listen-port=0",
		"consensus",
		"--network=" + network,
		fmt.Sprintf("--network-genesis-time=%d", genesisTime.Unix()),
		"--network-genesis-validators-root=" + hexutil.Encode(GenesisValidatorsRoot),
	}
	if exitsEnabled {
		args = append(args, "--voluntary-exit-signing-enabled")
	}
	output := &lockedBuffer{}
	cmd := exec.Command(binary, args...)
	cmd.Stdout, cmd.Stderr = output, output
	require.NoError(t, cmd.Start())
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	t.Cleanup(func() {
		_ = cmd.Process.Kill()
		<-done
		if t.Failed() {
			t.Logf("signer output:\n%s", output.String())
		}
	})

	// The signer writes its port to remotesigner.ports once it listens, and
	// answers /upcheck with 200 once its keys are loaded.
	var signerURL string
	deadline := time.Now().Add(30 * time.Second)
	for {
		select {
		case <-done:
			t.Fatalf("signer exited:\n%s", output.String())
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("signer not ready:\n%s", output.String())
		}
		if signerURL == "" {
			ports, err := os.ReadFile(filepath.Join(data, "remotesigner.ports"))
			if err == nil {
				for line := range strings.SplitSeq(string(ports), "\n") {
					if port, ok := strings.CutPrefix(line, "http-port="); ok {
						signerURL = "http://127.0.0.1:" + port
					}
				}
			}
		}
		if signerURL != "" {
			resp, err := http.Get(signerURL + "/upcheck")
			if err == nil {
				_ = resp.Body.Close()
				if resp.StatusCode == http.StatusOK {
					return signerURL, filepath.Join(data, "auth-token")
				}
			}
		}
		time.Sleep(20 * time.Millisecond)
	}
}

// lockedBuffer collects a process's output safely.
type lockedBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (l *lockedBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuffer) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}
