package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/opencloud/branding/server/internal/auth"
	"github.com/junkerderprovinz/opencloud/branding/server/internal/theme"
)

type fakeAuth struct{ err error }

func (f fakeAuth) Check(context.Context, string) error { return f.err }

var pngBytes = []byte("\x89PNG\r\n\x1a\nrest-of-image")

var admin = map[string]string{"X-Branding-Request": "1", "Authorization": "Basic YWRtaW46eA=="}

func newServer(t *testing.T, authErr error) (*Server, http.Handler) {
	t.Helper()
	dir := t.TempDir()
	var base theme.KV
	if err := json.Unmarshal([]byte(`{"clients":{"web":{"themes":[{"isDark":false},{"isDark":true,"logo":"x.svg"}]}}}`), &base); err != nil {
		t.Fatal(err)
	}
	s := &Server{
		Store: &theme.Store{
			AssetsDir: filepath.Join(dir, "web", "assets", "themes", "_branding"),
			StateFile: filepath.Join(dir, "branding", "state.json"),
			Base:      base,
		},
		Auth: fakeAuth{err: authErr},
	}
	return s, s.Handler()
}

func writeFile(t *testing.T, path string, data []byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}

func do(h http.Handler, method, path string, body []byte, header map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	for k, v := range header {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestHealthIsPublic(t *testing.T) {
	_, h := newServer(t, auth.ErrForbidden)
	if rec := do(h, http.MethodGet, "/brandingsvc/health", nil, nil); rec.Code != http.StatusOK {
		t.Fatalf("health = %d", rec.Code)
	}
}

func TestRefusals(t *testing.T) {
	cases := []struct {
		name    string
		authErr error
		method  string
		header  map[string]string
		want    int
	}{
		{"options", nil, http.MethodOptions, admin, http.StatusMethodNotAllowed},
		{"missing request header", nil, http.MethodPut, map[string]string{"Authorization": "Basic x"}, http.StatusForbidden},
		{"no credentials", auth.ErrNoCredentials, http.MethodPut, map[string]string{"X-Branding-Request": "1"}, http.StatusUnauthorized},
		{"not an admin", auth.ErrForbidden, http.MethodPut, admin, http.StatusForbidden},
	}
	for _, c := range cases {
		_, h := newServer(t, c.authErr)
		if rec := do(h, c.method, "/brandingsvc/api/image/logo", pngBytes, c.header); rec.Code != c.want {
			t.Errorf("%s: status %d, want %d", c.name, rec.Code, c.want)
		}
	}
}

func TestUploadAndClearLogo(t *testing.T) {
	s, h := newServer(t, nil)
	rec := do(h, http.MethodPut, "/brandingsvc/api/image/logo", pngBytes, admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("upload = %d %s", rec.Code, rec.Body)
	}
	var v view
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(v.Logo, "/themes/_branding/logo-") || !strings.HasSuffix(v.Logo, ".png") {
		t.Errorf("logo url = %q", v.Logo)
	}
	overlay, _ := os.ReadFile(filepath.Join(s.Store.AssetsDir, "theme.json"))
	if !strings.Contains(string(overlay), strings.TrimPrefix(v.Logo, "/")) {
		t.Errorf("overlay misses logo: %s", overlay)
	}
	if rec := do(h, http.MethodDelete, "/brandingsvc/api/image/logo", nil, admin); rec.Code != http.StatusOK {
		t.Fatalf("delete = %d", rec.Code)
	}
	overlay, _ = os.ReadFile(filepath.Join(s.Store.AssetsDir, "theme.json"))
	if string(overlay) != "{}" {
		t.Errorf("overlay after delete = %s", overlay)
	}
}

func TestTextRoundTrip(t *testing.T) {
	_, h := newServer(t, nil)
	rec := do(h, http.MethodPut, "/brandingsvc/api/text", []byte(`{"name":"Knight Cloud","slogan":"Files, forged"}`), admin)
	if rec.Code != http.StatusOK {
		t.Fatalf("text = %d %s", rec.Code, rec.Body)
	}
	rec = do(h, http.MethodGet, "/brandingsvc/api/state", nil, admin)
	var v view
	_ = json.Unmarshal(rec.Body.Bytes(), &v)
	if v.Name != "Knight Cloud" || v.Slogan != "Files, forged" {
		t.Errorf("state = %+v", v)
	}
	long := `{"name":"` + strings.Repeat("a", 65) + `","slogan":""}`
	if rec := do(h, http.MethodPut, "/brandingsvc/api/text", []byte(long), admin); rec.Code != http.StatusBadRequest {
		t.Errorf("too long name = %d", rec.Code)
	}
}

func TestUploadRejections(t *testing.T) {
	_, h := newServer(t, nil)
	if rec := do(h, http.MethodPut, "/brandingsvc/api/image/logo", []byte("%PDF-1.7"), admin); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("pdf = %d", rec.Code)
	}
	if rec := do(h, http.MethodPut, "/brandingsvc/api/image/logo", []byte(`<html><body/></html>`), admin); rec.Code != http.StatusUnsupportedMediaType {
		t.Errorf("html = %d", rec.Code)
	}
	big := append(append([]byte{}, pngBytes...), make([]byte, 2<<20)...)
	if rec := do(h, http.MethodPut, "/brandingsvc/api/image/favicon", big, admin); rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("oversized favicon = %d", rec.Code)
	}
	if rec := do(h, http.MethodPut, "/brandingsvc/api/image/wallpaper", pngBytes, admin); rec.Code != http.StatusNotFound {
		t.Errorf("unknown kind = %d", rec.Code)
	}
}

func TestSVGIsStoredSanitised(t *testing.T) {
	s, h := newServer(t, nil)
	evil := []byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><script>alert(2)</script><rect width="1" height="1"/></svg>`)
	if rec := do(h, http.MethodPut, "/brandingsvc/api/image/logo-dark", evil, admin); rec.Code != http.StatusOK {
		t.Fatalf("svg upload = %d %s", rec.Code, rec.Body)
	}
	st, _ := s.Store.Load()
	stored, err := os.ReadFile(filepath.Join(s.Store.AssetsDir, st.LogoDark))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, []byte("alert")) || !bytes.Contains(stored, []byte("<rect")) {
		t.Errorf("stored svg = %s", stored)
	}
}

func TestLoginBackgroundCaching(t *testing.T) {
	_, h := newServer(t, nil)
	if rec := do(h, http.MethodGet, "/brandingsvc/login-background", nil, nil); rec.Code != http.StatusNotFound {
		t.Fatalf("without background = %d", rec.Code)
	}
	if rec := do(h, http.MethodPut, "/brandingsvc/api/image/background", pngBytes, admin); rec.Code != http.StatusOK {
		t.Fatalf("upload = %d", rec.Code)
	}
	rec := do(h, http.MethodGet, "/brandingsvc/login-background", nil, nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Cache-Control") != "no-cache" || rec.Header().Get("ETag") == "" {
		t.Fatalf("background = %d, headers %v", rec.Code, rec.Header())
	}
	again := do(h, http.MethodGet, "/brandingsvc/login-background", nil, map[string]string{"If-None-Match": rec.Header().Get("ETag")})
	if again.Code != http.StatusNotModified {
		t.Errorf("revalidation = %d, want 304", again.Code)
	}
}

func TestSavedImageNamesMustBeAssetNames(t *testing.T) {
	cases := []struct {
		background string
		wantStatus int
		wantURL    string
	}{
		{"../../etc/opencloud/opencloud.yaml", http.StatusNotFound, ""},
		{"background-0123456789ab.png", http.StatusOK, "/themes/_branding/background-0123456789ab.png"},
	}
	for _, c := range cases {
		s, h := newServer(t, nil)
		writeFile(t, filepath.Join(s.Store.AssetsDir, filepath.FromSlash(c.background)), pngBytes)
		writeFile(t, s.Store.StateFile, []byte(`{"background": "`+c.background+`"}`))

		if rec := do(h, http.MethodGet, "/brandingsvc/login-background", nil, nil); rec.Code != c.wantStatus {
			t.Errorf("%s: login-background = %d, want %d", c.background, rec.Code, c.wantStatus)
		}
		var v view
		if err := json.Unmarshal(do(h, http.MethodGet, "/brandingsvc/api/state", nil, admin).Body.Bytes(), &v); err != nil {
			t.Fatal(err)
		}
		if v.Background != c.wantURL {
			t.Errorf("%s: state background = %q, want %q", c.background, v.Background, c.wantURL)
		}
	}
}
