package file

import (
	"io"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strings"

	"github.com/pkg/errors"
	log "github.com/sirupsen/logrus"
	"github.com/theQRL/qrysm/config/params"
)

// ExpandPath given a string which may be a relative path.
// 1. replace tilde with users home dir
// 2. expands embedded environment variables
// 3. cleans the path, e.g. /a/b/../c -> /a/c
// Note, it has limitations, e.g. ~someuser/tmp will not be expanded
func ExpandPath(p string) (string, error) {
	if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
		if home := HomeDir(); home != "" {
			p = home + p[1:]
		}
	}
	return filepath.Abs(path.Clean(os.ExpandEnv(p)))
}

// HandleBackupDir takes an input directory path and either alters its permissions to be usable if it already exists, creates it if not
func HandleBackupDir(dirPath string, permissionOverride bool) error {
	expanded, err := ExpandPath(dirPath)
	if err != nil {
		return err
	}
	exists, err := HasDir(expanded)
	if err != nil {
		return err
	}
	if exists {
		info, err := os.Stat(expanded)
		if err != nil {
			return err
		}
		if info.Mode().Perm() != params.BeaconIoConfig().ReadWriteExecutePermissions {
			if permissionOverride {
				if err := os.Chmod(expanded, params.BeaconIoConfig().ReadWriteExecutePermissions); err != nil {
					return err
				}
			} else {
				return errors.New("dir already exists without proper 0700 permissions")
			}
		}
	}
	return os.MkdirAll(expanded, params.BeaconIoConfig().ReadWriteExecutePermissions)
}

// SyncDir makes a directory's entries durable: a rename into or a removal
// from the directory is only on disk once the directory itself is synced.
func SyncDir(dir string) (err error) {
	d, err := os.Open(dir) // #nosec G304
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := d.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	return d.Sync()
}

// MkdirAll takes in a path, expands it if necessary, and looks through the
// permissions of every directory along the path, ensuring we are not attempting
// to overwrite any existing permissions. Finally, creates the directory accordingly
// with standardized, Qrysm project permissions. This is the static-analysis enforced
// method for creating a directory programmatically in Qrysm.
func MkdirAll(dirPath string) error {
	expanded, err := ExpandPath(dirPath)
	if err != nil {
		return err
	}
	exists, err := HasDir(expanded)
	if err != nil {
		return err
	}
	if exists {
		info, err := os.Stat(expanded)
		if err != nil {
			return err
		}
		if info.Mode().Perm() != params.BeaconIoConfig().ReadWriteExecutePermissions {
			return errors.New("dir already exists without proper 0700 permissions")
		}
	}
	return os.MkdirAll(expanded, params.BeaconIoConfig().ReadWriteExecutePermissions)
}

// WriteFile is the static-analysis enforced method for writing binary data to a file
// in Qrysm, enforcing a single entrypoint with standardized permissions.
func WriteFile(file string, data []byte) error {
	expanded, err := ExpandPath(file)
	if err != nil {
		return err
	}
	if FileExists(expanded) {
		info, err := os.Stat(expanded)
		if err != nil {
			return err
		}
		if info.Mode() != params.BeaconIoConfig().ReadWritePermissions {
			return errors.New("file already exists without proper 0600 permissions")
		}
	}
	return os.WriteFile(expanded, data, params.BeaconIoConfig().ReadWritePermissions)
}

// HomeDir for a user.
func HomeDir() string {
	if home := os.Getenv("HOME"); home != "" {
		return home
	}
	if usr, err := user.Current(); err == nil {
		return usr.HomeDir
	}
	return ""
}

// HasDir checks if a directory indeed exists at the specified path.
func HasDir(dirPath string) (bool, error) {
	fullPath, err := ExpandPath(dirPath)
	if err != nil {
		return false, err
	}
	info, err := os.Stat(fullPath)
	if os.IsNotExist(err) {
		return false, nil
	}
	if info == nil {
		return false, err
	}
	return info.IsDir(), err
}

// FileExists returns true if a file is not a directory and exists
// at the specified path.
func FileExists(filename string) bool {
	filePath, err := ExpandPath(filename)
	if err != nil {
		return false
	}
	info, err := os.Stat(filePath)
	if err != nil {
		if !os.IsNotExist(err) {
			log.WithError(err).Info("Checking for file existence returned an error")
		}
		return false
	}
	return info != nil && !info.IsDir()
}

// RecursiveFileFind returns true, and the path,  if a file is not a directory and exists
// at  dir or any of its subdirectories.  Finds the first instant based on the Walk order and returns.
// Define non-fatal error to stop the recursive directory walk
var stopWalk = errors.New("stop walking")

// RecursiveFileFind searches for file in a directory and its subdirectories.
func RecursiveFileFind(filename, dir string) (bool, string, error) {
	var found bool
	var fpath string
	dir = filepath.Clean(dir)
	found = false
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		// checks if its a file  and has the exact name as the filename
		// need to break the walk function by using a non-fatal error
		if !info.IsDir() && filename == info.Name() {
			found = true
			fpath = path
			return stopWalk
		}

		// no errors or file found
		return nil
	})
	if err != nil && err != stopWalk {
		return false, "", err
	}
	return found, fpath, nil
}

// ReadFileAsBytes expands a file name's absolute path and reads it as bytes from disk.
func ReadFileAsBytes(filename string) ([]byte, error) {
	filePath, err := ExpandPath(filename)
	if err != nil {
		return nil, errors.Wrap(err, "could not determine absolute path of password file")
	}
	return os.ReadFile(filePath) // #nosec G304
}

// CopyFile copy a file from source to destination path.
func CopyFile(src, dst string) (err error) {
	if !FileExists(src) {
		return errors.New("source file does not exist at provided path")
	}
	f, err := os.Open(src) // #nosec G304
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	dstFile, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, params.BeaconIoConfig().ReadWritePermissions) // #nosec G304
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := dstFile.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}()
	_, err = io.Copy(dstFile, f)
	return err
}
