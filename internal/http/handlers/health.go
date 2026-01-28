package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/alexlup06/authgate/internal/store"
)

type HealthHandler struct {
	store *store.Store
}

func NewHealthHandler(store *store.Store) *HealthHandler {
	return &HealthHandler{store: store}
}

func (h *HealthHandler) Health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()

	if err := h.store.Ping(ctx); err != nil {
		w.WriteHeader(http.StatusServiceUnavailable)
		return
	}

	w.WriteHeader(http.StatusOK)
}
