package meta

import (
	"net/http"

	"github.com/authara-org/authara/internal/http/kit/staticfs"
	"github.com/go-chi/chi/v5"
)

type StaticConfig struct {
	Dev bool
}

func RegisterStatic(r chi.Router, cfg StaticConfig) {
	var fs http.FileSystem

	if cfg.Dev {
		// DEV: union filesystem
		fs = staticfs.New(
			http.Dir("./frontend/dist"),
			http.Dir("./internal/http/static"),
		)
	} else {
		// PROD: single immutable dir
		fs = http.Dir("./internal/http/static")
	}

	r.Handle(
		"/auth/static/*",
		http.StripPrefix("/auth/static/", PrecompressedFileServer(fs, cfg.Dev)),
	)
}
