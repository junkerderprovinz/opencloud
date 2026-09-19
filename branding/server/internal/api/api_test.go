package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
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

func TestPublicRoutesOnlyRead(t *testing.T) {
	_, h := newServer(t, auth.ErrForbidden)
	for _, path := range []string{"/brandingsvc/health", "/brandingsvc/login.js", "/brandingsvc/login.json"} {
		for _, method := range []string{http.MethodGet, http.MethodHead} {
			if rec := do(h, method, path, nil, nil); rec.Code != http.StatusOK {
				t.Errorf("%s %s = %d, want 200", method, path, rec.Code)
			}
		}
		for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodOptions} {
			if rec := do(h, method, path, nil, admin); rec.Code != http.StatusMethodNotAllowed {
				t.Errorf("%s %s = %d, want 405", method, path, rec.Code)
			}
		}
	}
}

func TestLoginScriptIsServedAsJavaScript(t *testing.T) {
	_, h := newServer(t, nil)
	rec := do(h, http.MethodGet, "/brandingsvc/login.js", nil, nil)
	want := map[string]string{
		"Content-Type":           "text/javascript; charset=utf-8",
		"X-Content-Type-Options": "nosniff",
		"Cache-Control":          "no-cache",
	}
	for k, v := range want {
		if got := rec.Header().Get(k); got != v {
			t.Errorf("%s = %q, want %q", k, got, v)
		}
	}
	if !bytes.Equal(rec.Body.Bytes(), loginScript) || !strings.Contains(rec.Body.String(), "/brandingsvc/login.json") {
		t.Errorf("body is not the login script: %.200q", rec.Body)
	}
}

func loginJSON(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := do(h, http.MethodGet, "/brandingsvc/login.json", nil, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("login.json = %d %s", rec.Code, rec.Body)
	}
	if cc, ct := rec.Header().Get("Cache-Control"), rec.Header().Get("Content-Type"); cc != "no-store" || ct != "application/json" {
		t.Errorf("login.json Cache-Control %q, Content-Type %q", cc, ct)
	}
	return strings.TrimSpace(rec.Body.String())
}

func TestLoginDataFollowsTheSavedState(t *testing.T) {
	_, h := newServer(t, nil)
	if got, want := loginJSON(t, h), `{"name":"","slogan":"","background":"","favicon":"","theme":""}`; got != want {
		t.Errorf("nothing saved: %s, want %s", got, want)
	}
	for _, r := range []struct {
		path string
		body []byte
	}{
		{"/brandingsvc/api/text", []byte(`{"name":"Knight Cloud","slogan":"Files, forged"}`)},
		{"/brandingsvc/api/image/background", pngBytes},
		{"/brandingsvc/api/image/favicon", append(append([]byte{}, pngBytes...), 'f')},
		{"/brandingsvc/api/image/logo", append(append([]byte{}, pngBytes...), 'l')},
		{"/brandingsvc/api/login-theme", []byte(`{"loginTheme":"auto"}`)},
	} {
		if rec := do(h, http.MethodPut, r.path, r.body, admin); rec.Code != http.StatusOK {
			t.Fatalf("PUT %s = %d %s", r.path, rec.Code, rec.Body)
		}
	}
	var got map[string]string
	if err := json.Unmarshal([]byte(loginJSON(t, h)), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 || got["name"] != "Knight Cloud" || got["slogan"] != "Files, forged" ||
		!strings.HasPrefix(got["background"], "/themes/_branding/background-") ||
		!strings.HasPrefix(got["favicon"], "/themes/_branding/favicon-") || got["theme"] != "auto" {
		t.Errorf("saved: %v", got)
	}
}

func TestLoginThemeRoundTrip(t *testing.T) {
	_, h := newServer(t, nil)
	for _, want := range []string{"dark", "auto", ""} {
		rec := do(h, http.MethodPut, "/brandingsvc/api/login-theme", []byte(`{"loginTheme":"`+want+`"}`), admin)
		if rec.Code != http.StatusOK {
			t.Fatalf("PUT %q = %d %s", want, rec.Code, rec.Body)
		}
		var answer, state map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &answer)
		_ = json.Unmarshal(do(h, http.MethodGet, "/brandingsvc/api/state", nil, admin).Body.Bytes(), &state)
		if answer["loginTheme"] != want || state["loginTheme"] != want || len(answer) != len(state) {
			t.Errorf("PUT %q answered %v, state %v", want, answer, state)
		}
		if got := loginJSON(t, h); !strings.Contains(got, `"theme":"`+want+`"`) {
			t.Errorf("PUT %q: login.json = %s", want, got)
		}
	}
}

func TestLoginThemeRejections(t *testing.T) {
	_, h := newServer(t, nil)
	if rec := do(h, http.MethodPut, "/brandingsvc/api/login-theme", []byte(`{"loginTheme":"dark"}`), admin); rec.Code != http.StatusOK {
		t.Fatalf("dark = %d %s", rec.Code, rec.Body)
	}
	for _, body := range []string{
		`{"loginTheme":"light"}`,
		`{"loginTheme":"Dark"}`,
		`{"loginTheme":null}`,
		`{"loginTheme":1}`,
		`{"theme":"auto"}`,
		`{}`,
		`"dark"`,
		`{"loginTheme":`,
		``,
	} {
		if rec := do(h, http.MethodPut, "/brandingsvc/api/login-theme", []byte(body), admin); rec.Code != http.StatusBadRequest {
			t.Errorf("%s = %d, want 400", body, rec.Code)
		}
	}
	if got := loginJSON(t, h); !strings.Contains(got, `"theme":"dark"`) {
		t.Errorf("a refused body changed the theme: %s", got)
	}
}

func TestStateAnswerCarriesTheBrandingOnly(t *testing.T) {
	_, h := newServer(t, nil)
	var got map[string]any
	if err := json.Unmarshal(do(h, http.MethodGet, "/brandingsvc/api/state", nil, admin).Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	for _, k := range []string{"name", "slogan", "logo", "logoDark", "favicon", "background", "loginTheme"} {
		if _, ok := got[k]; !ok {
			t.Errorf("state lacks %s", k)
		}
		delete(got, k)
	}
	if len(got) != 0 {
		t.Errorf("state carries more: %v", got)
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func TestRefusals(t *testing.T) {
	routes := []struct {
		method, path string
		body         []byte
	}{
		{http.MethodGet, "/brandingsvc/api/state", nil},
		{http.MethodPut, "/brandingsvc/api/text", []byte(`{"name":"Knight Cloud"}`)},
		{http.MethodPut, "/brandingsvc/api/image/logo", pngBytes},
		{http.MethodDelete, "/brandingsvc/api/image/logo", nil},
		{http.MethodPut, "/brandingsvc/api/login-theme", []byte(`{"loginTheme":"dark"}`)},
	}
	cases := []struct {
		name    string
		authErr error
		method  string // empty for the route's own method
		header  map[string]string
		want    int
	}{
		{"options", nil, http.MethodOptions, admin, http.StatusMethodNotAllowed},
		{"missing request header", nil, "", map[string]string{"Authorization": "Basic x"}, http.StatusForbidden},
		{"no credentials", auth.ErrNoCredentials, "", map[string]string{"X-Branding-Request": "1"}, http.StatusUnauthorized},
		{"not an admin", auth.ErrForbidden, "", admin, http.StatusForbidden},
	}
	for _, r := range routes {
		for _, c := range cases {
			method := c.method
			if method == "" {
				method = r.method
			}
			_, h := newServer(t, c.authErr)
			if rec := do(h, method, r.path, r.body, c.header); rec.Code != c.want {
				t.Errorf("%s %s, %s: status %d, want %d", method, r.path, c.name, rec.Code, c.want)
			}
		}
	}
}

func TestPermissionCheckCausesAreLogged(t *testing.T) {
	cases := []struct {
		name    string
		authErr error
		lines   int
	}{
		{"upstream failure", fmt.Errorf("%w: dial tcp: connection refused", auth.ErrForbidden), 1},
		{"not an admin", auth.ErrForbidden, 0},
	}
	for _, c := range cases {
		logs := captureLog(t)
		_, h := newServer(t, c.authErr)
		if rec := do(h, http.MethodPut, "/brandingsvc/api/image/logo", pngBytes, admin); rec.Code != http.StatusForbidden {
			t.Errorf("%s: status %d, want 403", c.name, rec.Code)
		}
		if n := strings.Count(logs.String(), "\n"); n != c.lines {
			t.Errorf("%s: %d log lines, want %d: %q", c.name, n, c.lines, logs)
		}
		if strings.Contains(logs.String(), admin["Authorization"]) {
			t.Errorf("%s: log carries the credentials: %q", c.name, logs)
		}
	}
}

func TestStateReadFailuresAreLogged(t *testing.T) {
	s, h := newServer(t, nil)
	if err := os.MkdirAll(s.Store.StateFile, 0o755); err != nil {
		t.Fatal(err)
	}
	logs := captureLog(t)
	rec := do(h, http.MethodGet, "/brandingsvc/api/state", nil, admin)
	if rec.Code != http.StatusInternalServerError || strings.TrimSpace(rec.Body.String()) != "loading failed" {
		t.Errorf("state = %d %q, want 500 loading failed", rec.Code, rec.Body)
	}
	if n := strings.Count(logs.String(), "\n"); n != 1 {
		t.Errorf("state: %d log lines, want 1: %q", n, logs)
	}
	for _, path := range []string{"/brandingsvc/login-background", "/brandingsvc/login.json"} {
		logs.Reset()
		if rec := do(h, http.MethodGet, path, nil, nil); rec.Code != http.StatusInternalServerError {
			t.Errorf("%s = %d, want 500", path, rec.Code)
		}
		if n := strings.Count(logs.String(), "\n"); n != 1 {
			t.Errorf("%s: %d log lines, want 1: %q", path, n, logs)
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

func TestSVGTooCostlyToRenderIsRefused(t *testing.T) {
	_, h := newServer(t, nil)
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg">`)
	for i := 1; i <= 5; i++ {
		fmt.Fprintf(&b, `<mask id="m%d">%s</mask>`, i, strings.Repeat(fmt.Sprintf(`<rect fill="white" opacity=".9" mask="url(#m%d)"/>`, i+1), 10))
	}
	b.WriteString(`<rect width="10" height="10" mask="url(#m1)"/></svg>`)

	rec := do(h, http.MethodPut, "/brandingsvc/api/image/logo", []byte(b.String()), admin)
	if rec.Code != http.StatusUnsupportedMediaType || strings.TrimSpace(rec.Body.String()) != "SVG too complex" {
		t.Errorf("mask chain = %d %q, want 415 SVG too complex", rec.Code, rec.Body)
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
	if csp := rec.Header().Get("Content-Security-Policy"); csp != "default-src 'none'; img-src data:; style-src 'unsafe-inline'; sandbox" {
		t.Errorf("background CSP = %q", csp)
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
		if got := loginJSON(t, h); !strings.Contains(got, `"background":"`+c.wantURL+`"`) {
			t.Errorf("%s: login.json = %s, want background %q", c.background, got, c.wantURL)
		}
	}
}
