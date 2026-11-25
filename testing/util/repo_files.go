package util

import (
	"os"
	"path"

	"github.com/pkg/errors"
	"github.com/theQRL/qrysm/build/repo"
)

// RepoDirectoryNonEmpty returns true if directory exists and is not empty.
func RepoDirectoryNonEmpty(filePath string) (bool, error) {
	fs, err := repoReadDir(filePath)
	if err != nil {
		return false, err
	}
	return len(fs) > 0, nil
}

// RepoFileBytes returns the content of the repository file (see build/repo.File) at the joined path.
func RepoFileBytes(filePaths ...string) ([]byte, error) {
	filepath, err := repo.File(path.Join(filePaths...))
	if err != nil {
		return nil, err
	}
	fileBytes, err := os.ReadFile(filepath) // #nosec G304
	if err != nil {
		return nil, err
	}
	if len(fileBytes) == 0 {
		return nil, errors.New("empty file")
	}
	return fileBytes, nil
}

// RepoListFiles lists all of the file names in a given directory. Excludes directories. Returns
// an error when no non-directory files exist.
func RepoListFiles(filepath string) ([]string, error) {
	d, err := repoReadDir(filepath)
	if err != nil {
		return nil, err
	}

	ret := make([]string, 0, len(d))

	for _, f := range d {
		if f.IsDir() {
			continue
		}
		ret = append(ret, f.Name())
	}

	if len(ret) == 0 {
		return nil, errors.New("no files found")
	}

	return ret, nil
}

// RepoListDirectories lists all of the directories in the given directory. Excludes regular files.
// Returns error when no directories exist.
func RepoListDirectories(filepath string) ([]string, error) {
	d, err := repoReadDir(filepath)
	if err != nil {
		return nil, err
	}
	ret := make([]string, 0)
	for _, f := range d {
		if f.IsDir() {
			ret = append(ret, f.Name())
		}
	}

	if len(ret) == 0 {
		return nil, errors.New("no directories found")
	}

	return ret, nil
}

func repoReadDir(filepath string) ([]os.DirEntry, error) {
	p, err := repo.File(filepath)
	if err != nil {
		return nil, err
	}
	return os.ReadDir(p)
}
