package meta

import (
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strings"
)

// PrecompressedFileServer serves foo.br / foo.gz if present and accepted.
// It assumes `r.URL.Path` is already stripped (so it looks like "app.js").
func PrecompressedFileServer(fs http.FileSystem) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only GET/HEAD for static
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.WriteHeader(http.StatusMethodNotAllowed)
			return
		}

		name := path.Clean("/" + r.URL.Path)
		name = strings.TrimPrefix(name, "/")

		ae := r.Header.Get("Accept-Encoding")
		tryBr := strings.Contains(ae, "br")
		tryGz := strings.Contains(ae, "gzip")

		if tryBr && fileExists(fs, name+".br") {
			serveEncoded(w, r, fs, name+".br", name, "br")
			return
		}
		if tryGz && fileExists(fs, name+".gz") {
			serveEncoded(w, r, fs, name+".gz", name, "gzip")
			return
		}

		http.FileServer(fs).ServeHTTP(w, r)
	})
}

func fileExists(fs http.FileSystem, name string) bool {
	f, err := fs.Open(name)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
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
