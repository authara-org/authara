package showcase

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/authara-org/authara/internal/http/kit/render"
	"github.com/go-chi/chi/v5"
)

func TestEveryShowcasePageRenders(t *testing.T) {
	handler := New(render.New(render.Assets{}, true))
	if len(handler.pages) != 18 {
		t.Fatalf("showcase has %d pages, want 18", len(handler.pages))
	}

	router := chi.NewRouter()
	router.Get("/auth/showcase/pages/{slug}", handler.Page)
	seen := make(map[string]bool, len(handler.pages))
	for _, page := range handler.pages {
		if page.Slug == "" || page.Name == "" || page.Group == "" {
			t.Fatalf("incomplete page metadata: %+v", page)
		}
		if page.Group == "Admin" || page.Group == "Operator" {
			t.Fatalf("non-user-facing page included in showcase: %+v", page)
		}
		if seen[page.Slug] {
			t.Fatalf("duplicate page slug %q", page.Slug)
		}
		seen[page.Slug] = true

		recorder := httptest.NewRecorder()
		router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/showcase/pages/"+page.Slug, nil))
		if recorder.Code != http.StatusOK {
			t.Errorf("%s: status = %d, body = %s", page.Slug, recorder.Code, recorder.Body.String())
		}
		if !strings.Contains(recorder.Header().Get("Content-Type"), "text/html") {
			t.Errorf("%s: content type = %q", page.Slug, recorder.Header().Get("Content-Type"))
		}
	}
}

func TestGalleryContainsEveryPage(t *testing.T) {
	handler := New(render.New(render.Assets{}, true))
	recorder := httptest.NewRecorder()
	handler.Gallery(recorder, httptest.NewRequest(http.MethodGet, "/auth/showcase", nil))

	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", recorder.Code, recorder.Body.String())
	}
	body := recorder.Body.String()
	for _, page := range handler.pages {
		if !strings.Contains(body, "/auth/showcase/pages/"+page.Slug) {
			t.Errorf("gallery is missing %q", page.Slug)
		}
	}
}

func TestUnknownShowcasePageIsNotFound(t *testing.T) {
	handler := New(render.New(render.Assets{}, true))
	router := chi.NewRouter()
	router.Get("/auth/showcase/pages/{slug}", handler.Page)
	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/auth/showcase/pages/not-a-page", nil))
	if recorder.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
}
