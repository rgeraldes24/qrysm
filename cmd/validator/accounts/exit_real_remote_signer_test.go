package accounts

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
	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/crypto/ml_dsa_87"
	"github.com/theQRL/qrysm/testing/require"
)

// TestAccountsExit_RealRemoteSigner runs the same exits against a real
// go-remotesigner, started once with and once without
// --voluntary-exit-signing-enabled. It runs only when REMOTESIGNER_BINARY
// names the signer binary, for example the one `make build-consensus` builds
// in a go-remotesigner checkout:
//
//	REMOTESIGNER_BINARY=/path/to/go-remotesigner/bin/remotesigner-consensus \
//		go test ./cmd/validator/accounts -run TestAccountsExit_RealRemoteSigner
func TestAccountsExit_RealRemoteSigner(t *testing.T) {
	binary := os.Getenv("REMOTESIGNER_BINARY")
	if binary == "" {
		t.Skip("REMOTESIGNER_BINARY is not set")
	}
	seed := make([]byte, field_params.MLDSA87SeedLength)
	_, err := rand.Read(seed)
	require.NoError(t, err)
	key, err := ml_dsa_87.SecretKeyFromSeed(seed)
	require.NoError(t, err)
	genesisTime := time.Unix(time.Now().Add(-time.Hour).Unix(), 0)
	// The signer runs on this test's chain: the beacon config written in
	// qrysm's own format, with its genesis time and validators root.
	network := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(network, params.ConfigToYaml(params.BeaconConfig()), 0o600))
	for _, exitsEnabled := range []bool{true, false} {
		t.Run(fmt.Sprintf("exit signing enabled %v", exitsEnabled), func(t *testing.T) {
			beacon := startFakeBeaconNode(t, key.PublicKey().Marshal(), genesisTime)
			args := []string{
				"--network=" + network,
				fmt.Sprintf("--network-genesis-time=%d", genesisTime.Unix()),
				"--network-genesis-validators-root=" + hexutil.Encode(exitTestGVR),
			}
			if exitsEnabled {
				args = append(args, "--voluntary-exit-signing-enabled")
			}
			signerURL, tokenFile := startRealRemoteSigner(t, binary, seed, args)
			err := runRemoteSignerExit(t, beacon, signerURL, tokenFile)
			assertRemoteSignerExit(t, beacon, err, exitsEnabled)
		})
	}
}

// startRealRemoteSigner starts the signer binary on a loopback port with one
// file-raw key holding seed, passing consensusArgs after the consensus
// command, and waits until it is ready. It returns the signer URL and the
// sign-scope token file the signer generated.
func startRealRemoteSigner(t *testing.T, binary string, seed []byte, consensusArgs []string) (string, string) {
	t.Helper()
	data, keys := t.TempDir(), t.TempDir()
	keyFile := fmt.Sprintf("type: file-raw\nkeyType: ML_DSA_87\nprivateKey: \"%s\"\n", hexutil.Encode(seed))
	require.NoError(t, os.WriteFile(filepath.Join(keys, "validator.yaml"), []byte(keyFile), 0o600))
	args := append([]string{
		"--data-path=" + data,
		"--key-store-path=" + keys,
		"--http-listen-host=127.0.0.1",
		"--http-listen-port=0",
		"consensus",
	}, consensusArgs...)
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
