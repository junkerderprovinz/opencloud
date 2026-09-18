package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestIsLoopback(t *testing.T) {
	cases := map[string]bool{
		"127.0.0.1:9299":    true,
		"[::1]:9299":        true,
		"localhost:9200":    true,
		"127.0.0.1":         true,
		"0.0.0.0:9299":      false,
		"192.168.1.10:9200": false,
		"cloud.example.com": false,
		":9299":             false,
	}
	for in, want := range cases {
		if got := isLoopback(in); got != want {
			t.Errorf("isLoopback(%q) = %v, want %v", in, got, want)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestRegenerateRewritesOverlayAndReturns(t *testing.T) {
	dir := t.TempDir()
	base := filepath.Join(dir, "base-theme.json")
	writeFile(t, base, `{"clients":{"web":{"themes":[{"isDark":false},{"isDark":true,"logo":"x.svg"}]}}}`)
	writeFile(t, filepath.Join(dir, "branding", "state.json"), `{"logo": "logo-0123456789ab.png"}`)
	t.Setenv("BRANDING_DATA_DIR", dir)
	t.Setenv("BRANDING_BASE_THEME", base)

	if err := run([]string{"-regenerate"}); err != nil {
		t.Fatal(err)
	}
	overlay, err := os.ReadFile(filepath.Join(dir, "web", "assets", "themes", "_branding", "theme.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overlay), `"isDark":true,"logo":"themes/_branding/logo-0123456789ab.png"`) {
		t.Errorf("dark theme does not carry the saved logo: %s", overlay)
	}
}

func TestRegenerateFailsWithoutBaseTheme(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BRANDING_DATA_DIR", dir)
	t.Setenv("BRANDING_BASE_THEME", filepath.Join(dir, "missing.json"))

	if err := run([]string{"-regenerate"}); err == nil {
		t.Fatal("want an error for a missing base theme")
	}
}
