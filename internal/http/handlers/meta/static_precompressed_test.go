package meta

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
)

func TestFingerprintAssetOverridesNoStoreWithImmutablePolicy(t *testing.T) {
	assets := http.FS(fstest.MapFS{
		"app.0123456789abcdef.js": &fstest.MapFile{
			Data: []byte("console.log('authara');"),
			Mode: fs.FileMode(0o444),
		},
	})
	handler := httpmiddleware.SecurityHeaders(httpmiddleware.SecurityHeadersConfig{})(
		PrecompressedFileServer(assets, false),
	)
	req := httptest.NewRequest(http.MethodGet, "/app.0123456789abcdef.js", nil)
	rr := httptest.NewRecorder()

	handler.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
	if got := rr.Result().Header.Get("Cache-Control"); got != "public, max-age=31536000, immutable" {
		t.Fatalf("expected immutable Cache-Control policy, got %q", got)
	}
}
