//go:build unix

package remote_web3signer

import (
	"errors"
	"io/fs"
	"os"
	"syscall"
)

// keepOwner gives tmp, which replaces the key file that old describes, that
// file's owner and group, as far as the validator client may: changing the
// owner takes root, so an unprivileged validator client keeps only the group,
// when it is a member of it. tmp is otherwise left with the validator
// client's user and the group it was created with.
func keepOwner(tmp *os.File, old fs.FileInfo) error {
	want, ok := old.Sys().(*syscall.Stat_t)
	if !ok {
		return nil
	}
	info, err := tmp.Stat()
	if err != nil {
		return err
	}
	got, ok := info.Sys().(*syscall.Stat_t)
	if !ok || (got.Uid == want.Uid && got.Gid == want.Gid) {
		return nil
	}
	err = tmp.Chown(int(want.Uid), int(want.Gid))
	if errors.Is(err, fs.ErrPermission) && got.Gid != want.Gid {
		err = tmp.Chown(-1, int(want.Gid))
	}
	if errors.Is(err, fs.ErrPermission) {
		return nil
	}
	return err
}
