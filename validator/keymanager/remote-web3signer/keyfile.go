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
	"github.com/theQRL/qrysm/io/file"
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
		line = keyFileLineContent(line)
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

// keyFileLineContent returns a key file line without its comment and
// surrounding whitespace: the key on that line, or "".
func keyFileLineContent(line string) string {
	if hash := strings.IndexByte(line, '#'); hash >= 0 {
		line = line[:hash]
	}
	return strings.TrimSpace(line)
}

// appendKeyFileLines returns the contents of a key file with one line per
// key appended, ending data's last line first if it has no newline.
func appendKeyFileLines(data []byte, keys [][field_params.MLDSA87PubkeyLength]byte) []byte {
	out := make([]byte, 0, len(data)+1+len(keys)*(2+2*field_params.MLDSA87PubkeyLength+1))
	out = append(out, data...)
	if len(out) > 0 && out[len(out)-1] != '\n' {
		out = append(out, '\n')
	}
	for _, key := range keys {
		out = append(out, hexutil.Encode(key[:])...)
		out = append(out, '\n')
	}
	return out
}

// removeKeyFileLines returns the contents of a key file without the lines
// that hold one of keys. Other lines, comments and blank lines included, are
// kept as they are. data must be a valid key file.
func removeKeyFileLines(data []byte, keys map[[field_params.MLDSA87PubkeyLength]byte]bool) []byte {
	out := make([]byte, 0, len(data))
	for _, line := range strings.SplitAfter(string(data), "\n") {
		if content := keyFileLineContent(line); content != "" {
			if key, err := DecodePublicKey(content); err == nil && keys[key] {
				continue
			}
		}
		out = append(out, line...)
	}
	return out
}

// renameFile is os.Rename, a variable so that tests can make the last step
// of a key file write fail.
var renameFile = os.Rename

// replaceKeyFile replaces the key file at path with data, so that a reader
// sees either the old or the new contents, never a part: data goes to a
// temporary file in the same directory, which is synced and then renamed over
// the key file. A symbolic link is followed, so that the link stays in place.
// The new file keeps the old one's permissions, and its owner and group as far
// as the validator client may change them (see keepOwner). On error the key
// file is left as it was.
func replaceKeyFile(path string, data []byte) (err error) {
	target, err := filepath.EvalSymlinks(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return err
	}
	dir, base := filepath.Split(target)
	tmp, err := os.CreateTemp(dir, "."+base+".tmp-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tmp.Close() // Already closed unless an earlier step failed.
			_ = os.Remove(tmp.Name())
		}
	}()
	if err = tmp.Chmod(info.Mode().Perm()); err != nil {
		return err
	}
	if err := keepOwner(tmp, info); err != nil {
		log.WithError(err).Warn("Could not give the new remote signer key file the old one's owner")
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = renameFile(tmp.Name(), target); err != nil {
		return err
	}
	// The new contents are in place; a failed directory sync only means the
	// rename may not survive a crash.
	if err := file.SyncDir(dir); err != nil {
		log.WithError(err).Warn("Could not sync the remote signer key file's directory")
	}
	return nil
}

// ReadKeyFile reads and parses the key file at path.
func ReadKeyFile(path string) ([][field_params.MLDSA87PubkeyLength]byte, error) {
	_, keys, err := readKeyFile(path)
	return keys, err
}

// readKeyFile reads and parses the key file at path, and also returns its
// contents.
func readKeyFile(path string) ([]byte, [][field_params.MLDSA87PubkeyLength]byte, error) {
	data, err := os.ReadFile(path) // #nosec G304 -- operator-supplied path
	if err != nil {
		return nil, nil, errors.Wrap(err, "could not read remote signer key file")
	}
	keys, err := ParseKeyFile(data)
	if err != nil {
		return nil, nil, errors.Wrapf(err, "invalid remote signer key file %s", path)
	}
	return data, keys, nil
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
	km.updateLock.Lock()
	defer km.updateLock.Unlock()
	keys, err := ReadKeyFile(km.keyFile)
	if err != nil {
		log.WithError(err).Error("Could not reload the remote signer key file, keeping the current keys")
		return
	}
	if km.setKeys(keys) {
		log.WithField("keys", len(keys)).Info(keymanager.KeysReloaded)
	}
}

// setKeys replaces the validating keys, which Sign and RandaoReveal then
// check against, and, if they changed, notifies subscribers and reports true.
// The caller holds km.updateLock, which keeps notifications in order; km.lock
// is released before notifying, since a subscriber may fetch the keys before
// it receives the next change.
func (km *Keymanager) setKeys(keys [][field_params.MLDSA87PubkeyLength]byte) bool {
	km.lock.Lock()
	changed := !slices.Equal(km.providedPublicKeys, keys)
	if changed {
		km.providedPublicKeys = keys
		km.keySet = keySetOf(keys)
	}
	km.lock.Unlock()
	if changed {
		km.accountsChangedFeed.Send(slices.Clone(keys))
	}
	return changed
}

func closeWatcher(watcher *fsnotify.Watcher) {
	if err := watcher.Close(); err != nil {
		log.WithError(err).Error("Could not close the key file watcher")
	}
}
