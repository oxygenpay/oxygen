package util

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/pkg/errors"
)

// EnsureFile checks that file exists. If not, creates an empty file.
func EnsureFile(path string, mode fs.FileMode) error {
	stat, err := os.Stat(path)

	switch {
	case os.IsNotExist(err):
		if errDir := EnsureDirectory(filepath.Dir(path), mode); errDir != nil {
			return errDir
		}

		return createFile(path, mode)
	case err != nil:
		return err
	case stat.IsDir():
		return fmt.Errorf("path %q is a directory", path)
	}

	return nil
}

// EnsureDirectory checks that file exists. If not, creates an empty directory.
func EnsureDirectory(path string, mode fs.FileMode) error {
	if err := os.MkdirAll(path, mode); err != nil {
		return errors.Wrapf(err, "unable to ensure directory %q", path)
	}

	return nil
}

func createFile(path string, mode fs.FileMode) error {
	file, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE, mode)
	if err != nil {
		return err
	}

	_ = file.Close()

	return nil
}
