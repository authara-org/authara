package meta

import (
	"mime"
	"net/http"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	hasBr = 1 << iota
	hasGz
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
				staticNotFound(w)
				return
			}

			// Usually you *don’t* want immutable caching in dev:
			w.Header().Set("Cache-Control", "no-cache")
			http.FileServer(fs).ServeHTTP(w, r)
		})
	}

	// bitset: hasBr = br available, hasGz = gz available
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
			staticNotFound(w)
			return
		}

		if strings.HasSuffix(name, ".br") || strings.HasSuffix(name, ".gz") {
			staticNotFound(w)
			return
		}

		flags := index[name]
		if flags == 0 {
			setCacheHeaders(w)
			http.FileServer(fs).ServeHTTP(w, r)
			return
		}
		addVary(w.Header(), "Accept-Encoding")

		encoding, acceptable := negotiateEncoding(r.Header.Values("Accept-Encoding"), flags)
		if !acceptable {
			w.WriteHeader(http.StatusNotAcceptable)
			return
		}
		setCacheHeaders(w)

		switch encoding {
		case "br":
			serveEncoded(w, r, fs, name+".br", name, "br")
			return
		case "gzip":
			serveEncoded(w, r, fs, name+".gz", name, "gzip")
			return
		}

		http.FileServer(fs).ServeHTTP(w, r)
	})
}

func staticNotFound(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNotFound)
}

func buildPrecompressedIndex(fs http.FileSystem) map[string]uint8 {
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

	// Important: serve the encoded file body, but URL path stays original.
	rr := r.Clone(r.Context())
	rr.URL.Path = "/" + encodedName

	http.FileServer(fs).ServeHTTP(w, rr)
}

// negotiateEncoding returns the preferred available representation. An empty
// encoding means the identity representation. The boolean is false only when
// the client explicitly rejects every available representation.
func negotiateEncoding(headerValues []string, flags uint8) (string, bool) {
	preferences := parseAcceptEncoding(headerValues)
	if !preferences.present {
		// An absent field permits any coding, but retaining identity preserves the
		// server's existing behavior and avoids surprising older clients.
		return "", true
	}

	type candidate struct {
		name      string
		available bool
	}

	// For equal qualities, retain the server's existing br-before-gzip order.
	candidates := []candidate{
		{name: "br", available: flags&hasBr != 0},
		{name: "gzip", available: flags&hasGz != 0},
	}

	bestQuality := -1
	bestEncoding := ""
	for _, candidate := range candidates {
		if !candidate.available {
			continue
		}

		quality := preferences.quality(candidate.name)
		if quality > bestQuality {
			bestQuality = quality
			bestEncoding = candidate.name
		}
	}

	identityQuality, identityExplicit := preferences.explicit["identity"]
	if identityExplicit && identityQuality > bestQuality {
		return "", identityQuality > 0
	}
	if bestQuality > 0 {
		return bestEncoding, true
	}
	return "", preferences.identityAcceptable()
}

type encodingPreferences struct {
	present  bool
	explicit map[string]int
	wildcard *int
}

func parseAcceptEncoding(headerValues []string) encodingPreferences {
	preferences := encodingPreferences{
		present:  headerValues != nil,
		explicit: make(map[string]int),
	}

	for _, headerValue := range headerValues {
		for member := range strings.SplitSeq(headerValue, ",") {
			parts := strings.Split(member, ";")
			coding := normalizeEncoding(strings.TrimSpace(parts[0]))
			if coding == "" || len(parts) > 2 {
				continue
			}

			quality := 1000
			if len(parts) == 2 {
				name, value, ok := strings.Cut(strings.TrimSpace(parts[1]), "=")
				if !ok || !strings.EqualFold(strings.TrimSpace(name), "q") {
					continue
				}

				quality, ok = parseQuality(strings.TrimSpace(value))
				if !ok {
					continue
				}
			}

			if coding == "*" {
				setLowestQuality(&preferences.wildcard, quality)
				continue
			}
			if previous, ok := preferences.explicit[coding]; !ok || quality < previous {
				// Repeated contradictory entries are resolved conservatively so an
				// explicit q=0 can never be bypassed by another occurrence.
				preferences.explicit[coding] = quality
			}
		}
	}

	return preferences
}

func (p encodingPreferences) quality(coding string) int {
	if quality, ok := p.explicit[coding]; ok {
		return quality
	}
	if p.wildcard != nil {
		return *p.wildcard
	}
	return 0
}

func (p encodingPreferences) identityAcceptable() bool {
	if quality, ok := p.explicit["identity"]; ok {
		return quality > 0
	}
	// Identity is acceptable by default. A zero-quality wildcard excludes it
	// only when no more specific identity preference was supplied.
	return p.wildcard == nil || *p.wildcard > 0
}

func normalizeEncoding(coding string) string {
	coding = strings.ToLower(coding)
	if coding == "x-gzip" {
		return "gzip"
	}
	return coding
}

func parseQuality(value string) (int, bool) {
	whole, fraction, hasFraction := strings.Cut(value, ".")
	if hasFraction && len(fraction) > 3 {
		return 0, false
	}
	for _, digit := range fraction {
		if digit < '0' || digit > '9' {
			return 0, false
		}
	}

	switch whole {
	case "0":
		if !hasFraction {
			return 0, true
		}
		for len(fraction) < 3 {
			fraction += "0"
		}
		quality, err := strconv.Atoi(fraction)
		return quality, err == nil
	case "1":
		if !hasFraction || strings.Trim(fraction, "0") == "" {
			return 1000, true
		}
	}

	return 0, false
}

func setLowestQuality(current **int, quality int) {
	if *current != nil && **current <= quality {
		return
	}
	value := quality
	*current = &value
}

func addVary(header http.Header, field string) {
	for _, value := range header.Values("Vary") {
		for member := range strings.SplitSeq(value, ",") {
			if strings.EqualFold(strings.TrimSpace(member), field) {
				return
			}
		}
	}
	header.Add("Vary", field)
}

func setCacheHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
}
