package filesystem

import (
	"io/fs"
	"os"
	"path/filepath"
)

type systemFS struct {
	root string
}

func NewSystemFS(root string) fs.FS {
	return &systemFS{root: root}
}

func (sfs *systemFS) Open(name string) (fs.File, error) {
	return os.Open(filepath.Join(sfs.root, name))
}
