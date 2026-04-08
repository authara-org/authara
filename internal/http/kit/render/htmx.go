package render

import (
	"net/http"

	"github.com/a-h/templ"
	"github.com/authara-org/authara/internal/http/kit/htmx"
)

type HTMXRenderConfig struct {
	Target  string
	Swap    string
	PushURL string
}

func WithHTMX(
	renderer Renderer,
	w http.ResponseWriter,
	r *http.Request,
	status int,
	component templ.Component,
	cfg HTMXRenderConfig,
) error {
	target := cfg.Target
	if target == "" {
		target = "#body"
	}

	swap := cfg.Swap
	if swap == "" {
		swap = "innerHTML"
	}

	htmx.ReTarget(w, target)
	htmx.ReSwap(w, swap)

	if cfg.PushURL != "" {
		htmx.PushUrl(w, cfg.PushURL)
	}

	return renderer(w, r, status, component)
}

func IntoBody(
	renderer Renderer,
	w http.ResponseWriter,
	r *http.Request,
	status int,
	pushURL string,
	component templ.Component,
) error {
	return WithHTMX(
		renderer,
		w,
		r,
		status,
		component,
		HTMXRenderConfig{
			Target:  "#body",
			Swap:    "innerHTML",
			PushURL: pushURL,
		},
	)
}
