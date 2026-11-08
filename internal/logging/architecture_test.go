package logging

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestProductionCodeCannotBypassStructuredLogger(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	for _, directory := range []string{"internal", "cmd"} {
		err := filepath.WalkDir(filepath.Join(root, directory), func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.Contains(path, string(filepath.Separator)+"internal"+string(filepath.Separator)+"logging"+string(filepath.Separator)) {
				return nil
			}
			file, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			for _, imported := range file.Imports {
				name, err := strconv.Unquote(imported.Path.Value)
				if err != nil {
					return err
				}
				if name == "log" || name == "log/slog" {
					t.Errorf("%s imports forbidden logging package %q", path, name)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
