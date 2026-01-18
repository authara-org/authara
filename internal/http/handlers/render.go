package handlers

import (
	"net/http"

	"github.com/a-h/templ"
)

func Render(
	w http.ResponseWriter,
	r *http.Request,
	status int,
	component templ.Component,
) error {
	w.WriteHeader(status)
	return component.Render(r.Context(), w)
}
