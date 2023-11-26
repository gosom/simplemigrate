package filesystem_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/gosom/simplemigrate/internal/filesystem"
)

func TestSystemFS_Open(t *testing.T) {
	tmpDir := t.TempDir()
	testFile := filepath.Join(tmpDir, "test.txt")
	err := os.WriteFile(testFile, []byte("hello world"), 0o600)
	require.NoError(t, err)

	sysFs := filesystem.NewSystemFS(tmpDir)

	file, err := sysFs.Open("test.txt")

	require.NoError(t, err)
	err = file.Close()
	require.NoError(t, err)

	_, err = sysFs.Open("nonexistent.txt")
	require.Error(t, err)
}
