package staticfs

import (
	"net/http"
	"os"
)

type MultiDirFS struct {
	dirs []http.FileSystem
}

func New(dirs ...http.FileSystem) http.FileSystem {
	return MultiDirFS{dirs: dirs}
}

func (m MultiDirFS) Open(name string) (http.File, error) {
	for _, dir := range m.dirs {
		f, err := dir.Open(name)
		if err == nil {
			return f, nil
		}
	}
	return nil, os.ErrNotExist
}
