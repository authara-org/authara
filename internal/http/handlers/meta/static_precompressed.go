package meta

import (
	"fmt"
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// PrecompressedFileServer serves foo.br / foo.gz if present and accepted.
// It assumes `r.URL.Path` is already stripped (so it looks like "app.dfh1943hfa.js").
func PrecompressedFileServer(fs http.FileSystem, dev bool) http.Handler {
	if dev {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet && r.Method != http.MethodHead {
				w.WriteHeader(http.StatusMethodNotAllowed)
				return
			}

			name := path.Clean("/" + r.URL.Path)
			name = strings.TrimPrefix(name, "/")

			if name == "manifest.json" || strings.HasSuffix(name, ".br") || strings.HasSuffix(name, ".gz") {
				http.NotFound(w, r)
				return
			}

			// Usually you *don’t* want immutable caching in dev:
			w.Header().Set("Cache-Control", "no-cache")
			http.FileServer(fs).ServeHTTP(w, r)
		})
	}

	// bitset: 1 = br available, 2 = gz available
	index := buildPrecompressedIndex(fs)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only GET/HEAD for static
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		name := path.Clean("/" + r.URL.Path)
		name = strings.TrimPrefix(name, "/")

		if name == "manifest.json" {
			http.NotFound(w, r)
			return
		}

		if strings.HasSuffix(name, ".br") || strings.HasSuffix(name, ".gz") {
			http.NotFound(w, r)
			return
		}

		ae := r.Header.Get("Accept-Encoding")
		tryBr := strings.Contains(ae, "br")
		tryGz := strings.Contains(ae, "gzip")

		setCacheHeaders(w)

		flags := index[name]

		if tryBr && (flags&1) != 0 {
			serveEncoded(w, r, fs, name+".br", name, "br")
			return
		}
		if tryGz && (flags&2) != 0 {
			serveEncoded(w, r, fs, name+".gz", name, "gzip")
			return
		}

		http.FileServer(fs).ServeHTTP(w, r)
	})
}

func buildPrecompressedIndex(fs http.FileSystem) map[string]uint8 {
	const (
		hasBr = 1
		hasGz = 2
	)

	idx := make(map[string]uint8, 256)

	var walk func(dir string)
	walk = func(dir string) {
		f, err := fs.Open(dir)
		if err != nil {
			return
		}
		defer f.Close()

		// http.File exposes Readdir; for non-directories it returns an error.
		entries, err := f.Readdir(-1)
		if err != nil {
			return
		}

		for _, e := range entries {
			p := path.Join(dir, e.Name())
			// path.Join(".", "x") => "x" (nice), and nested stays "a/b".
			if e.IsDir() {
				walk(p)
				continue
			}

			fmt.Println(p)

			switch {
			case strings.HasSuffix(p, ".br"):
				orig := strings.TrimSuffix(p, ".br")
				idx[orig] |= hasBr
			case strings.HasSuffix(p, ".gz"):
				orig := strings.TrimSuffix(p, ".gz")
				idx[orig] |= hasGz
			}
		}
	}

	// Start at "." to match http.Dir semantics; relative paths match your request path cleaning.
	walk(".")

	return idx
}

func serveEncoded(w http.ResponseWriter, r *http.Request, fs http.FileSystem, encodedName, originalName, encoding string) {
	ext := filepath.Ext(originalName)
	if ct := mime.TypeByExtension(ext); ct != "" {
		w.Header().Set("Content-Type", ct)
	}

	w.Header().Set("Content-Encoding", encoding)
	w.Header().Add("Vary", "Accept-Encoding")

	// Important: serve the encoded file body, but URL path stays original.
	rr := r.Clone(r.Context())
	rr.URL.Path = "/" + encodedName

	http.FileServer(fs).ServeHTTP(w, rr)
}

func setCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
}
