package remote_web3signer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/pkg/errors"
	"github.com/theQRL/go-qrl/common/hexutil"
	"github.com/theQRL/qrysm/async"
	field_params "github.com/theQRL/qrysm/config/fieldparams"
	"github.com/theQRL/qrysm/validator/keymanager"
)

// keyFileReloadDebounce is how long the key file must stay unchanged before
// it is re-read, so that a burst of writes causes one reload. A variable for
// tests.
var keyFileReloadDebounce = time.Second

// DecodePublicKey decodes a 0x-prefixed hex ML-DSA-87 public key.
func DecodePublicKey(s string) ([field_params.MLDSA87PubkeyLength]byte, error) {
	var key [field_params.MLDSA87PubkeyLength]byte
	decoded, err := hexutil.Decode(s)
	if err != nil {
		return key, err
	}
	if len(decoded) != field_params.MLDSA87PubkeyLength {
		return key, fmt.Errorf("public key is %d bytes, want %d", len(decoded), field_params.MLDSA87PubkeyLength)
	}
	copy(key[:], decoded)
	return key, nil
}

// ParseKeyFile parses the contents of a key file: one 0x-prefixed public key
// per line. Surrounding whitespace, blank lines and everything from a '#' to
// the end of its line are ignored. A malformed or repeated key is an error
// that names its line, never the key.
func ParseKeyFile(data []byte) ([][field_params.MLDSA87PubkeyLength]byte, error) {
	keys := make([][field_params.MLDSA87PubkeyLength]byte, 0)
	seen := make(map[[field_params.MLDSA87PubkeyLength]byte]int)
	for i, line := range strings.Split(string(data), "\n") {
		lineNumber := i + 1
		if hash := strings.IndexByte(line, '#'); hash >= 0 {
			line = line[:hash]
		}
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		key, err := DecodePublicKey(line)
		if err != nil {
			return nil, errors.Wrapf(err, "line %d is not a 0x-prefixed public key", lineNumber)
		}
		if first, ok := seen[key]; ok {
			return nil, fmt.Errorf("line %d repeats the public key on line %d", lineNumber, first)
		}
		seen[key] = lineNumber
		keys = append(keys, key)
	}
	return keys, nil
}

// ReadKeyFile reads and parses the key file at path.
func ReadKeyFile(path string) ([][field_params.MLDSA87PubkeyLength]byte, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- operator-supplied path
	if err != nil {
		return nil, errors.Wrap(err, "could not read remote signer key file")
	}
	keys, err := ParseKeyFile(data)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid remote signer key file %s", path)
	}
	return keys, nil
}

// watchKeyFile starts reloading the key file whenever it changes, until ctx
// is done or km.stopWatching is called. It watches the file's directory
// rather than the file, so that a file replaced by a rename, as editors and
// atomic writers do, stays watched.
func (km *Keymanager) watchKeyFile(ctx context.Context) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return errors.Wrap(err, "could not create a file watcher")
	}
	if err := watcher.Add(filepath.Dir(km.keyFile)); err != nil {
		closeWatcher(watcher)
		return errors.Wrap(err, "could not watch the remote signer key file's directory")
	}
	ctx, cancel := context.WithCancel(ctx)
	done := make(chan struct{})
	go func() {
		defer close(done)
		km.handleKeyFileEvents(ctx, watcher)
	}()
	km.stopWatching = func() {
		cancel()
		<-done
	}
	return nil
}

func (km *Keymanager) handleKeyFileEvents(ctx context.Context, watcher *fsnotify.Watcher) {
	defer closeWatcher(watcher)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	changes := make(chan any, 100)
	go async.Debounce(ctx, keyFileReloadDebounce, changes, func(any) { km.reloadKeyFile() })
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return
			}
			if filepath.Clean(event.Name) != km.keyFile {
				continue
			}
			select {
			case changes <- event:
			default: // A reload is already pending.
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return
			}
			log.WithError(err).Error("Could not watch the remote signer key file")
		case <-ctx.Done():
			return
		}
	}
}

// reloadKeyFile re-reads the key file and, if its keys changed, replaces the
// validating keys and notifies subscribers. An unreadable or invalid file
// keeps the current keys.
func (km *Keymanager) reloadKeyFile() {
	keys, err := ReadKeyFile(km.keyFile)
	if err != nil {
		log.WithError(err).Error("Could not reload the remote signer key file, keeping the current keys")
		return
	}
	km.lock.Lock()
	changed := !slices.Equal(km.providedPublicKeys, keys)
	if changed {
		km.providedPublicKeys = keys
	}
	km.lock.Unlock()
	if !changed {
		return
	}
	log.WithField("keys", len(keys)).Info(keymanager.KeysReloaded)
	km.accountsChangedFeed.Send(slices.Clone(keys))
}

func closeWatcher(watcher *fsnotify.Watcher) {
	if err := watcher.Close(); err != nil {
		log.WithError(err).Error("Could not close the key file watcher")
	}
}
