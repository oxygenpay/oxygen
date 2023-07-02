package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestFS(t *testing.T) {
	// RW
	const mode = 0770

	tempRoot := t.TempDir()
	wrap := func(path string) string {
		return tempRoot + path
	}

	for _, tt := range []struct {
		name   string
		path   string
		isFile bool
	}{
		{
			name:   "ensures env file",
			path:   "/app/oxygen.env",
			isFile: true,
		},
		{
			name:   "ensures env file one more time",
			path:   "/app/oxygen.env",
			isFile: true,
		},
		{
			name: "creates sessions directory",
			path: "/app/sessions",
		},
		{
			name:   "creates bolt.db file",
			path:   "/app/kms/kms.db",
			isFile: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			// ARRANGE
			p := wrap(tt.path)

			// ACT
			var err error
			if tt.isFile {
				err = EnsureFile(p, mode)
			} else {
				err = EnsureDirectory(p, mode)
			}

			// ASSERT
			assert.NoError(t, err)

			if tt.isFile {
				assert.FileExists(t, p)
			} else {
				assert.DirExists(t, p)
			}
		})
	}
}
