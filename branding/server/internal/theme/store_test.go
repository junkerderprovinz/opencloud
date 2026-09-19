package theme

import (
	"bytes"
	"errors"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
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

func TestMoveAsideKeepsEveryBackup(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.json")
	for _, content := range []string{"{one", "{two"} {
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := moveAside(path, errors.New("invalid")); err != nil {
			t.Fatal(err)
		}
	}
	moved, err := filepath.Glob(path + ".invalid-*")
	if err != nil {
		t.Fatal(err)
	}
	var contents []string
	for _, m := range moved {
		b, _ := os.ReadFile(m)
		contents = append(contents, string(b))
	}
	if len(contents) != 2 || !slices.Contains(contents, "{one") || !slices.Contains(contents, "{two") {
		t.Errorf("backups %v hold %q, want both files", moved, contents)
	}
}

func TestSaveSucceedsWhenUnusedImageStays(t *testing.T) {
	if runtime.GOOS == "windows" || os.Geteuid() == 0 {
		t.Skip("needs directory permissions that bind the test user")
	}
	s := newStore(t)
	if _, err := s.SetImage(imagefmt.Logo, "png", []byte("one")); err != nil {
		t.Fatal(err)
	}
	// Without read permission the folder can take the new image but cannot
	// be listed, so only the cleanup after the commit fails.
	if err := os.Chmod(s.AssetsDir, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(s.AssetsDir, 0o755) })
	logs := captureLog(t)
	st, err := s.SetImage(imagefmt.Logo, "png", []byte("two"))
	if err != nil {
		t.Fatalf("saved change reported as failed: %v", err)
	}
	if saved, _ := s.Load(); saved.Logo != st.Logo {
		t.Errorf("state logo = %q, want %q", saved.Logo, st.Logo)
	}
	if !strings.Contains(logs.String(), s.AssetsDir) {
		t.Errorf("cleanup failure not logged: %q", logs)
	}
}

func captureLog(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })
	return &buf
}

func writeOverlay(t *testing.T, s *Store, content string) {
	t.Helper()
	if err := os.MkdirAll(s.AssetsDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(s.AssetsDir, "theme.json"), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func overlayBackups(t *testing.T, s *Store) []string {
	t.Helper()
	backups, err := filepath.Glob(filepath.Join(filepath.Dir(s.StateFile), "theme.json.before-branding-*"))
	if err != nil {
		t.Fatal(err)
	}
	return backups
}

func TestFirstStartKeepsHandSetOverlay(t *testing.T) {
	s := newStore(t)
	handSet := `{"common":{"name":" Hand Cloud ","slogan":"Made by hand","logo":"themes/_branding/mylogo.png","urls":{"imprint":"https://example.org/imprint"}},` +
		`"clients":{"web":{"defaults":{"logo":"themes/_branding/mylogo.png","favicon":"themes/_branding/fav.ico"},"themes":[{"isDark":true,"label":"Mine"}]}}}`
	writeOverlay(t, s, handSet)
	logs := captureLog(t)
	if err := s.Regenerate(); err != nil {
		t.Fatal(err)
	}

	backups := overlayBackups(t, s)
	if len(backups) != 1 {
		t.Fatalf("want one overlay backup, found %v", backups)
	}
	if b, _ := os.ReadFile(backups[0]); string(b) != handSet {
		t.Errorf("backup = %s, want the original bytes", b)
	}
	if !strings.Contains(logs.String(), backups[0]) {
		t.Errorf("backup not named in the log: %q", logs)
	}
	if st, err := s.Load(); err != nil || st != (State{Name: "Hand Cloud", Slogan: "Made by hand"}) {
		t.Errorf("state = %+v, err %v", st, err)
	}
	overlay, _ := os.ReadFile(filepath.Join(s.AssetsDir, "theme.json"))
	if want := `{"common":{"name":"Hand Cloud","slogan":"Made by hand","urls":{"imprint":"https://example.org/imprint"}}}`; string(overlay) != want {
		t.Errorf("overlay = %s, want %s", overlay, want)
	}

	if err := s.Regenerate(); err != nil {
		t.Fatal(err)
	}
	if backups := overlayBackups(t, s); len(backups) != 1 {
		t.Errorf("second start made another backup: %v", backups)
	}
}

func TestFirstStartSkipsTextOutOfLimits(t *testing.T) {
	s := newStore(t)
	writeOverlay(t, s, `{"common":{"name":"`+strings.Repeat("a", MaxNameRunes+1)+`","slogan":42}}`)
	if err := s.Regenerate(); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.Load(); st != (State{}) {
		t.Errorf("state = %+v, want empty", st)
	}
	if backups := overlayBackups(t, s); len(backups) != 1 {
		t.Errorf("want one overlay backup, found %v", backups)
	}
}

func TestSavedStateSkipsOverlayBackup(t *testing.T) {
	s := newStore(t)
	if _, err := s.SetText("Knight Cloud", ""); err != nil {
		t.Fatal(err)
	}
	writeOverlay(t, s, `{"common":{"name":"Hand Cloud","logo":"themes/_branding/mylogo.png"}}`)
	if err := s.Regenerate(); err != nil {
		t.Fatal(err)
	}
	if backups := overlayBackups(t, s); len(backups) != 0 {
		t.Errorf("backup made although state.json exists: %v", backups)
	}
	if st, _ := s.Load(); st.Name != "Knight Cloud" {
		t.Errorf("state = %+v", st)
	}
}

func TestMovedAsideStateCanBePutBack(t *testing.T) {
	s := newStore(t)
	if _, err := s.SetImage(imagefmt.Logo, "png", []byte("logo")); err != nil {
		t.Fatal(err)
	}
	saved, err := s.SetImage(imagefmt.Background, "jpg", []byte("background"))
	if err != nil {
		t.Fatal(err)
	}
	good, err := os.ReadFile(s.StateFile)
	if err != nil {
		t.Fatal(err)
	}
	broken := strings.Replace(string(good), "\n}", ",\n}", 1)
	if err := os.WriteFile(s.StateFile, []byte(broken), 0o644); err != nil {
		t.Fatal(err)
	}
	// The second start finds the empty state the first one wrote.
	for range 2 {
		if err := s.Regenerate(); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(s.StateFile, good, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := s.Regenerate(); err != nil {
		t.Fatal(err)
	}
	if st, err := s.Load(); err != nil || st != saved {
		t.Errorf("state = %+v, err %v, want %+v", st, err, saved)
	}
	for _, name := range []string{saved.Logo, saved.Background} {
		if _, err := os.Stat(filepath.Join(s.AssetsDir, name)); err != nil {
			t.Errorf("%s: %v", name, err)
		}
	}
}

func TestRegenerateDropsImagesWhoseFileIsGone(t *testing.T) {
	s := newStore(t)
	var st State
	for _, k := range []imagefmt.Kind{imagefmt.Logo, imagefmt.Favicon, imagefmt.Background} {
		saved, err := s.SetImage(k, "png", []byte(k))
		if err != nil {
			t.Fatal(err)
		}
		st = saved
	}
	for _, name := range []string{st.Logo, st.Favicon, st.Background} {
		if err := os.Remove(filepath.Join(s.AssetsDir, name)); err != nil {
			t.Fatal(err)
		}
	}
	logs := captureLog(t)
	if err := s.Regenerate(); err != nil {
		t.Fatal(err)
	}

	if got, err := s.Load(); err != nil || got != (State{}) {
		t.Errorf("state = %+v, err %v, want empty", got, err)
	}
	if !strings.Contains(logs.String(), st.Background) {
		t.Errorf("missing background not logged: %q", logs)
	}
	b, _ := os.ReadFile(s.StateFile)
	if !strings.Contains(string(b), "\n  \"background\": \"\"") {
		t.Errorf("state file still names the background: %s", b)
	}
	overlay, _ := os.ReadFile(filepath.Join(s.AssetsDir, "theme.json"))
	if strings.Contains(string(overlay), AssetPrefix) {
		t.Errorf("overlay still refers to a missing image: %s", overlay)
	}
}

func TestMissingImageIsLoggedOnce(t *testing.T) {
	s := newStore(t)
	saved, err := s.SetImage(imagefmt.Background, "png", []byte("background"))
	if err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(s.AssetsDir, saved.Background)
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	logs := captureLog(t)
	for range 3 {
		if st, err := s.Load(); err != nil || st.Background != "" {
			t.Errorf("state = %+v, err %v, want no background", st, err)
		}
	}
	lines := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if len(lines) != 1 || !strings.Contains(lines[0], "ignoring it") {
		t.Fatalf("want one line about the missing image, got %q", logs)
	}

	if err := os.WriteFile(file, []byte("background"), 0o644); err != nil {
		t.Fatal(err)
	}
	if st, _ := s.Load(); st.Background != saved.Background {
		t.Errorf("background = %q after the file came back, want %q", st.Background, saved.Background)
	}
	if err := os.Remove(file); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Load(); err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(logs.String(), "ignoring it"); n != 2 {
		t.Errorf("image missing a second time logged %d lines in all, want 2: %q", n, logs)
	}
}
