// Copyright 2015 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.
package file_test

import (
	"bufio"
	"bytes"
	"os"
	"os/user"
	"path/filepath"
	"sort"
	"testing"

	"github.com/theQRL/qrysm/config/params"
	"github.com/theQRL/qrysm/io/file"
	"github.com/theQRL/qrysm/testing/assert"
	"github.com/theQRL/qrysm/testing/require"
)

func TestPathExpansion(t *testing.T) {
	u, err := user.Current()
	require.NoError(t, err)
	tests := map[string]string{
		"/home/someuser/tmp": "/home/someuser/tmp",
		"~/tmp":              u.HomeDir + "/tmp",
		"$DDDXXX/a/b":        "/tmp/a/b",
		"/a/b/":              "/a/b",
	}
	require.NoError(t, os.Setenv("DDDXXX", "/tmp"))
	for test, expected := range tests {
		expanded, err := file.ExpandPath(test)
		require.NoError(t, err)
		assert.Equal(t, expected, expanded)
	}
}

func TestMkdirAll_AlreadyExists_WrongPermissions(t *testing.T) {
	dirName := t.TempDir() + "somedir"
	err := os.MkdirAll(dirName, os.ModePerm)
	require.NoError(t, err)
	err = file.MkdirAll(dirName)
	assert.ErrorContains(t, "already exists without proper 0700 permissions", err)
}

func TestMkdirAll_AlreadyExists_Override(t *testing.T) {
	dirName := t.TempDir() + "somedir"
	err := os.MkdirAll(dirName, params.BeaconIoConfig().ReadWriteExecutePermissions)
	require.NoError(t, err)
	assert.NoError(t, file.MkdirAll(dirName))
}

func TestHandleBackupDir_AlreadyExists_Override(t *testing.T) {
	dirName := t.TempDir() + "somedir"
	err := os.MkdirAll(dirName, os.ModePerm)
	require.NoError(t, err)
	info, err := os.Stat(dirName)
	require.NoError(t, err)
	assert.Equal(t, "drwxr-xr-x", info.Mode().String())
	assert.NoError(t, file.HandleBackupDir(dirName, true))
	info, err = os.Stat(dirName)
	require.NoError(t, err)
	assert.Equal(t, "drwx------", info.Mode().String())
}

func TestHandleBackupDir_AlreadyExists_No_Override(t *testing.T) {
	dirName := t.TempDir() + "somedir"
	err := os.MkdirAll(dirName, os.ModePerm)
	require.NoError(t, err)
	info, err := os.Stat(dirName)
	require.NoError(t, err)
	assert.Equal(t, "drwxr-xr-x", info.Mode().String())
	err = file.HandleBackupDir(dirName, false)
	assert.ErrorContains(t, "dir already exists without proper 0700 permissions", err)
	info, err = os.Stat(dirName)
	require.NoError(t, err)
	assert.Equal(t, "drwxr-xr-x", info.Mode().String())
}

func TestHandleBackupDir_NewDir(t *testing.T) {
	dirName := t.TempDir() + "somedir"
	require.NoError(t, file.HandleBackupDir(dirName, true))
	info, err := os.Stat(dirName)
	require.NoError(t, err)
	assert.Equal(t, "drwx------", info.Mode().String())
}

func TestMkdirAll_OK(t *testing.T) {
	dirName := t.TempDir() + "somedir"
	err := file.MkdirAll(dirName)
	assert.NoError(t, err)
	exists, err := file.HasDir(dirName)
	require.NoError(t, err)
	assert.Equal(t, true, exists)
}

func TestWriteFile_AlreadyExists_WrongPermissions(t *testing.T) {
	dirName := t.TempDir() + "somedir"
	err := os.MkdirAll(dirName, os.ModePerm)
	require.NoError(t, err)
	someFileName := filepath.Join(dirName, "somefile.txt")
	require.NoError(t, os.WriteFile(someFileName, []byte("hi"), os.ModePerm))
	err = file.WriteFile(someFileName, []byte("hi"))
	assert.ErrorContains(t, "already exists without proper 0600 permissions", err)
}

func TestWriteFile_AlreadyExists_OK(t *testing.T) {
	dirName := t.TempDir() + "somedir"
	err := os.MkdirAll(dirName, os.ModePerm)
	require.NoError(t, err)
	someFileName := filepath.Join(dirName, "somefile.txt")
	require.NoError(t, os.WriteFile(someFileName, []byte("hi"), params.BeaconIoConfig().ReadWritePermissions))
	assert.NoError(t, file.WriteFile(someFileName, []byte("hi")))
}

func TestWriteFile_OK(t *testing.T) {
	dirName := t.TempDir() + "somedir"
	err := os.MkdirAll(dirName, os.ModePerm)
	require.NoError(t, err)
	someFileName := filepath.Join(dirName, "somefile.txt")
	require.NoError(t, file.WriteFile(someFileName, []byte("hi")))
	exists := file.FileExists(someFileName)
	assert.Equal(t, true, exists)
}

func TestCopyFile(t *testing.T) {
	fName := t.TempDir() + "testfile"
	err := os.WriteFile(fName, []byte{1, 2, 3}, params.BeaconIoConfig().ReadWritePermissions)
	require.NoError(t, err)

	err = file.CopyFile(fName, fName+"copy")
	assert.NoError(t, err)
	defer func() {
		assert.NoError(t, os.Remove(fName+"copy"))
	}()

	assert.Equal(t, true, deepCompare(t, fName, fName+"copy"))
}

func TestRecursiveFileFind(t *testing.T) {
	tmpDir, _ := tmpDirWithContentsForRecursiveFind(t)
	tests := []struct {
		name  string
		root  string
		path  string
		found bool
	}{
		{
			name:  "file1",
			root:  tmpDir,
			path:  "subfolder1/subfolder11/file1",
			found: true,
		},
		{
			name:  "file2",
			root:  tmpDir,
			path:  "subfolder2/file2",
			found: true,
		},
		{
			name:  "file1",
			root:  tmpDir + "/subfolder1",
			path:  "subfolder11/file1",
			found: true,
		},
		{
			name:  "file3",
			root:  tmpDir,
			path:  "file3",
			found: true,
		},
		{
			name:  "file4",
			root:  tmpDir,
			path:  "",
			found: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			found, _, err := file.RecursiveFileFind(tt.name, tt.root)
			require.NoError(t, err)

			assert.DeepEqual(t, tt.found, found)
		})
	}
}

func deepCompare(t *testing.T, file1, file2 string) bool {
	sf, err := os.Open(file1)
	assert.NoError(t, err)
	df, err := os.Open(file2)
	assert.NoError(t, err)
	sscan := bufio.NewScanner(sf)
	dscan := bufio.NewScanner(df)

	for sscan.Scan() && dscan.Scan() {
		if !bytes.Equal(sscan.Bytes(), dscan.Bytes()) {
			return false
		}
	}
	return true
}

// tmpDirWithContentsForRecursiveFind returns path to temporary directory having some folders/files in it.
// Directory is automatically removed by internal testing cleanup methods.
func tmpDirWithContentsForRecursiveFind(t *testing.T) (string, []string) {
	dir := t.TempDir()
	fnames := []string{
		"subfolder1/subfolder11/file1",
		"subfolder2/file2",
		"file3",
	}
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subfolder1", "subfolder11"), 0777))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "subfolder2"), 0777))
	for _, fname := range fnames {
		require.NoError(t, os.WriteFile(filepath.Join(dir, fname), []byte(fname), 0777))
	}
	sort.Strings(fnames)
	return dir, fnames
}

func TestSyncDir(t *testing.T) {
	require.NoError(t, file.SyncDir(t.TempDir()))
	require.NotNil(t, file.SyncDir(filepath.Join(t.TempDir(), "missing")))
}
