package meta

import (
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"

	httpmiddleware "github.com/authara-org/authara/internal/http/middleware"
)

func TestPrecompressedFileServerNegotiatesAcceptEncoding(t *testing.T) {
	assets := http.FS(fstest.MapFS{
		"app.js":    &fstest.MapFile{Data: []byte("identity")},
		"app.js.br": &fstest.MapFile{Data: []byte("brotli")},
		"app.js.gz": &fstest.MapFile{Data: []byte("gzip")},
	})
	handler := PrecompressedFileServer(assets, false)

	tests := []struct {
		name         string
		method       string
		headerValues []string
		wantStatus   int
		wantEncoding string
		wantBody     string
		wantVary     string
	}{
		{name: "absent", wantStatus: http.StatusOK, wantBody: "identity", wantVary: "Accept-Encoding"},
		{name: "empty", headerValues: []string{""}, wantStatus: http.StatusOK, wantBody: "identity", wantVary: "Accept-Encoding"},
		{name: "brotli rejected", headerValues: []string{"br;q=0, gzip;q=1, identity;q=0.5"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: "gzip", wantVary: "Accept-Encoding"},
		{name: "gzip rejected", headerValues: []string{"gzip;q=0, br;q=0.5, identity;q=0.1"}, wantStatus: http.StatusOK, wantEncoding: "br", wantBody: "brotli", wantVary: "Accept-Encoding"},
		{name: "preference ordering", headerValues: []string{"br;q=0.4, gzip;q=0.8, identity;q=0.1"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: "gzip", wantVary: "Accept-Encoding"},
		{name: "equal quality uses server preference", headerValues: []string{"gzip;q=0.8, br;q=0.8, identity;q=0.1"}, wantStatus: http.StatusOK, wantEncoding: "br", wantBody: "brotli", wantVary: "Accept-Encoding"},
		{name: "wildcard", headerValues: []string{"*"}, wantStatus: http.StatusOK, wantEncoding: "br", wantBody: "brotli", wantVary: "Accept-Encoding"},
		{name: "explicit rejection overrides wildcard", headerValues: []string{"br;q=0, *;q=0.8, identity;q=0.1"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: "gzip", wantVary: "Accept-Encoding"},
		{name: "malformed member ignored", headerValues: []string{"br;q=invalid, gzip;q=0.5, identity;q=0.1"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: "gzip", wantVary: "Accept-Encoding"},
		{name: "case insensitive alias", headerValues: []string{"X-GZIP, identity;q=0.1"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: "gzip", wantVary: "Accept-Encoding"},
		{name: "multiple field lines", headerValues: []string{"br;q=0", "gzip;q=0.8, identity;q=0.1"}, wantStatus: http.StatusOK, wantEncoding: "gzip", wantBody: "gzip", wantVary: "Accept-Encoding"},
		{name: "identity preferred", headerValues: []string{"br;q=0.5, identity;q=1"}, wantStatus: http.StatusOK, wantBody: "identity", wantVary: "Accept-Encoding"},
		{name: "identity fallback", headerValues: []string{"br;q=0, gzip;q=0"}, wantStatus: http.StatusOK, wantBody: "identity", wantVary: "Accept-Encoding"},
		{name: "all representations rejected", headerValues: []string{"*;q=0"}, wantStatus: http.StatusNotAcceptable, wantVary: "Accept-Encoding"},
		{name: "head", method: http.MethodHead, headerValues: []string{"br"}, wantStatus: http.StatusOK, wantEncoding: "br", wantVary: "Accept-Encoding"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			method := test.method
			if method == "" {
				method = http.MethodGet
			}
			req := httptest.NewRequest(method, "/app.js", nil)
			if test.headerValues != nil {
				req.Header["Accept-Encoding"] = test.headerValues
			}
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			response := rr.Result()
			defer response.Body.Close()
			if response.StatusCode != test.wantStatus {
				t.Fatalf("status = %d, want %d", response.StatusCode, test.wantStatus)
			}
			if got := response.Header.Get("Content-Encoding"); got != test.wantEncoding {
				t.Errorf("Content-Encoding = %q, want %q", got, test.wantEncoding)
			}
			if got := response.Header.Get("Vary"); got != test.wantVary {
				t.Errorf("Vary = %q, want %q", got, test.wantVary)
			}

			body, err := io.ReadAll(response.Body)
			if err != nil {
				t.Fatalf("read response body: %v", err)
			}
			if got := string(body); got != test.wantBody {
				t.Errorf("body = %q, want %q", got, test.wantBody)
			}
		})
	}
}

func TestPrecompressedFileServerSkipsNegotiationWithoutEncodedVariant(t *testing.T) {
	assets := http.FS(fstest.MapFS{
		"plain.js": &fstest.MapFile{Data: []byte("identity")},
	})
	handler := PrecompressedFileServer(assets, false)

	tests := []struct {
		name       string
		path       string
		wantStatus int
		wantBody   string
	}{
		{name: "identity only", path: "/plain.js", wantStatus: http.StatusOK, wantBody: "identity"},
		{name: "missing", path: "/missing.js", wantStatus: http.StatusNotFound, wantBody: "404 page not found\n"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, test.path, nil)
			req.Header.Set("Accept-Encoding", "*;q=0")
			rr := httptest.NewRecorder()

			handler.ServeHTTP(rr, req)

			if rr.Code != test.wantStatus {
				t.Fatalf("status = %d, want %d", rr.Code, test.wantStatus)
			}
			if got := rr.Header().Get("Vary"); got != "" {
				t.Errorf("Vary = %q, want empty", got)
			}
			if got := rr.Body.String(); got != test.wantBody {
				t.Errorf("body = %q, want %q", got, test.wantBody)
			}
		})
	}
}

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
