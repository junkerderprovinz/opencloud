package theme

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/junkerderprovinz/opencloud/branding/server/internal/imagefmt"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	return &Store{
		AssetsDir: filepath.Join(dir, "web", "assets", "themes", "_branding"),
		StateFile: filepath.Join(dir, "branding", "state.json"),
		Base:      testBase(t),
	}
}

func TestSetImageReplacesPreviousFile(t *testing.T) {
	s := newStore(t)
	first, err := s.SetImage(imagefmt.Logo, "png", []byte("one"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := s.SetImage(imagefmt.Logo, "png", []byte("two"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Logo == second.Logo {
		t.Fatal("file name did not change with the content")
	}
	if _, err := os.Stat(filepath.Join(s.AssetsDir, first.Logo)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("old logo still on disk: %v", err)
	}
	if _, err := os.Stat(filepath.Join(s.AssetsDir, second.Logo)); err != nil {
		t.Errorf("new logo missing: %v", err)
	}
	overlay, err := os.ReadFile(filepath.Join(s.AssetsDir, "theme.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(overlay), AssetPrefix+second.Logo) {
		t.Errorf("overlay does not reference the new logo: %s", overlay)
	}
}

func TestClearImageRemovesFileAndKey(t *testing.T) {
	s := newStore(t)
	st, err := s.SetImage(imagefmt.Favicon, "png", []byte("icon"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClearImage(imagefmt.Favicon); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(s.AssetsDir, st.Favicon)); !errors.Is(err, os.ErrNotExist) {
		t.Error("favicon file not removed")
	}
	overlay, _ := os.ReadFile(filepath.Join(s.AssetsDir, "theme.json"))
	if string(overlay) != "{}" {
		t.Errorf("overlay = %s, want {}", overlay)
	}
}

func TestSetTextLimits(t *testing.T) {
	s := newStore(t)
	if _, err := s.SetText(strings.Repeat("ä", MaxNameRunes), strings.Repeat("x", MaxSloganRunes)); err != nil {
		t.Fatalf("limits must be inclusive: %v", err)
	}
	if _, err := s.SetText(strings.Repeat("a", MaxNameRunes+1), ""); !errors.Is(err, ErrTooLong) {
		t.Errorf("want ErrTooLong, got %v", err)
	}
}

func TestStateFileLayout(t *testing.T) {
	s := newStore(t)
	if _, err := s.SetText("Knight Cloud", "Files, forged"); err != nil {
		t.Fatal(err)
	}
	fresh := &Store{AssetsDir: s.AssetsDir, StateFile: s.StateFile, Base: s.Base}
	st, err := fresh.Load()
	if err != nil {
		t.Fatal(err)
	}
	if st.Name != "Knight Cloud" || st.Slogan != "Files, forged" {
		t.Errorf("state = %+v", st)
	}
	b, _ := os.ReadFile(s.StateFile)
	// entrypoint.sh reads the background with sed and relies on this layout.
	if !strings.Contains(string(b), "\n  \"background\": \"\"") {
		t.Errorf("unexpected state file layout: %s", b)
	}
}

func TestRegenerateWithoutState(t *testing.T) {
	s := newStore(t)
	if err := s.Regenerate(); err != nil {
		t.Fatal(err)
	}
	overlay, err := os.ReadFile(filepath.Join(s.AssetsDir, "theme.json"))
	if err != nil || string(overlay) != "{}" {
		t.Errorf("overlay = %q, err %v", overlay, err)
	}
}

func TestInvalidOverlayIsMovedAside(t *testing.T) {
	s := newStore(t)
	if err := os.MkdirAll(s.AssetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.AssetsDir, "theme.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	st, err := s.SetText("Knight Cloud", "")
	if err != nil {
		t.Fatalf("SetText must not fail on invalid overlay: %v", err)
	}
	if st.Name != "Knight Cloud" {
		t.Errorf("state = %+v, want Name=Knight Cloud", st)
	}
	overlay, err := os.ReadFile(filepath.Join(s.AssetsDir, "theme.json"))
	if err != nil {
		t.Fatalf("new overlay not created: %v", err)
	}
	if !strings.Contains(string(overlay), "Knight Cloud") {
		t.Errorf("overlay does not contain name: %s", overlay)
	}
	entries, err := os.ReadDir(s.AssetsDir)
	if err != nil {
		t.Fatal(err)
	}
	invalidCount := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "theme.json.invalid-") {
			invalidCount++
		}
	}
	if invalidCount != 1 {
		t.Errorf("expected exactly 1 theme.json.invalid-* file, found %d", invalidCount)
	}
}

func TestInvalidStateIsMovedAside(t *testing.T) {
	s := newStore(t)
	if err := os.MkdirAll(filepath.Dir(s.StateFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(s.StateFile, []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Regenerate(); err != nil {
		t.Fatalf("Regenerate must not fail on invalid state: %v", err)
	}
	if st, err := s.Load(); err != nil || st != (State{}) {
		t.Errorf("state = %+v, err %v, want empty state", st, err)
	}
	moved, err := filepath.Glob(s.StateFile + ".invalid-*")
	if err != nil {
		t.Fatal(err)
	}
	if len(moved) != 1 {
		t.Fatalf("want exactly 1 state.json.invalid-* file, found %v", moved)
	}
	if b, _ := os.ReadFile(moved[0]); string(b) != "{not json" {
		t.Errorf("moved file = %q, want the original content", b)
	}
}
