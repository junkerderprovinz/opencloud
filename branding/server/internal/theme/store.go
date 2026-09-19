package theme

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/junkerderprovinz/opencloud/branding/server/internal/imagefmt"
)

// State is what an admin has set. Image fields hold file names inside the
// _branding folder; empty means the OpenCloud default.
type State struct {
	Name       string `json:"name"`
	Slogan     string `json:"slogan"`
	Logo       string `json:"logo"`
	LogoDark   string `json:"logoDark"`
	Favicon    string `json:"favicon"`
	Background string `json:"background"`
}

const (
	MaxNameRunes   = 64
	MaxSloganRunes = 120
)

// ErrTooLong is returned when the name or slogan exceeds its limit.
var ErrTooLong = errors.New("theme: text too long")

var assetName = regexp.MustCompile(`^(logo|logo-dark|favicon|background)-[0-9a-f]{12}\.(png|jpg|gif|webp|svg)$`)

// Store serialises every change to the state file, the image files and the
// theme overlay.
type Store struct {
	AssetsDir string // <data>/web/assets/themes/_branding
	StateFile string // <data>/branding/state.json
	Base      KV

	mu sync.Mutex
}

// Load returns the saved state, or the zero State if nothing was saved yet.
func (s *Store) Load() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, _, err := s.load()
	return st, err
}

// Regenerate rewrites the overlay from the saved state. brandingd calls it on
// start, because a new image can bring a new base theme; it deletes no images,
// so a state.json that was moved aside can still be put back.
func (s *Store) Regenerate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.current()
	if err != nil {
		return err
	}
	return s.commit(st)
}

// SetText saves name and slogan; empty strings restore the defaults.
func (s *Store) SetText(name, slogan string) (State, error) {
	name, slogan = strings.TrimSpace(name), strings.TrimSpace(slogan)
	if utf8.RuneCountInString(name) > MaxNameRunes || utf8.RuneCountInString(slogan) > MaxSloganRunes {
		return State{}, ErrTooLong
	}
	return s.update(func(st *State) error {
		st.Name, st.Slogan = name, slogan
		return nil
	})
}

// SetImage stores data under a content-hashed name. The name changes with
// the content, which matters because OpenCloud serves theme files with a
// long max-age.
func (s *Store) SetImage(k imagefmt.Kind, ext string, data []byte) (State, error) {
	sum := sha256.Sum256(data)
	file := fmt.Sprintf("%s-%x.%s", k, sum[:6], ext)
	return s.update(func(st *State) error {
		if err := os.MkdirAll(s.AssetsDir, 0o755); err != nil {
			return err
		}
		if err := writeAtomic(filepath.Join(s.AssetsDir, file), data); err != nil {
			return err
		}
		*field(st, k) = file
		return nil
	})
}

// ClearImage restores the OpenCloud default for one slot.
func (s *Store) ClearImage(k imagefmt.Kind) (State, error) {
	return s.update(func(st *State) error {
		*field(st, k) = ""
		return nil
	})
}

// load returns the saved state and whether there was no state file at all.
func (s *Store) load() (st State, missing bool, err error) {
	b, err := os.ReadFile(s.StateFile)
	if errors.Is(err, os.ErrNotExist) {
		return st, true, nil
	}
	if err != nil {
		return st, false, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, false, moveAside(s.StateFile, err)
	}
	// The names become file paths, and state.json sits on a volume that other
	// processes can write. A file removed by hand would leave a broken image on
	// every page.
	for _, k := range []imagefmt.Kind{imagefmt.Logo, imagefmt.LogoDark, imagefmt.Favicon, imagefmt.Background} {
		name := field(&st, k)
		if *name == "" {
			continue
		}
		if !assetName.MatchString(*name) {
			log.Printf("theme: %s: ignoring %s %q, not an asset name", s.StateFile, k, *name)
			*name = ""
			continue
		}
		if _, err := os.Stat(filepath.Join(s.AssetsDir, *name)); errors.Is(err, os.ErrNotExist) {
			log.Printf("theme: %s image %s is missing from %s, using OpenCloud's own", k, *name, s.AssetsDir)
			*name = ""
		}
	}
	return st, false, nil
}

// current returns the state to write from: the saved one, or before the first
// save whatever adoptOverlay takes over.
func (s *Store) current() (State, error) {
	st, missing, err := s.load()
	if err == nil && missing {
		err = s.adoptOverlay(&st)
	}
	return st, err
}

func (s *Store) update(change func(*State) error) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.current()
	if err != nil {
		return State{}, err
	}
	if err := change(&st); err != nil {
		return State{}, err
	}
	if err := s.commit(st); err != nil {
		return State{}, err
	}
	// The change is saved and live; an image left behind goes with the next one.
	if err := s.removeUnused(st); err != nil {
		log.Printf("theme: removing unused images: %v", err)
	}
	return st, nil
}

// adoptOverlay runs while there is no state file. An overlay that already sets
// branding keys, by hand or through OpenCloud's /branding/logo, is copied
// next to the state file before the commit replaces those keys, and its name
// and slogan are taken over; image keys point at files not named like assets.
func (s *Store) adoptOverlay(st *State) error {
	overlayPath := filepath.Join(s.AssetsDir, "theme.json")
	raw, err := os.ReadFile(overlayPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var overlay KV
	if json.Unmarshal(raw, &overlay) != nil || !hasOwnedKeys(overlay) {
		return nil
	}
	dir := filepath.Dir(s.StateFile)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	backup := freeName(filepath.Join(dir, fmt.Sprintf("theme.json.before-branding-%d", time.Now().Unix())))
	if err := writeAtomic(backup, raw); err != nil {
		return err
	}
	for _, f := range []struct {
		key string
		max int
		dst *string
	}{{"name", MaxNameRunes, &st.Name}, {"slogan", MaxSloganRunes, &st.Slogan}} {
		v, _ := getPath(overlay, []string{"common", f.key})
		if text, ok := v.(string); ok {
			if text = strings.TrimSpace(text); utf8.RuneCountInString(text) <= f.max {
				*f.dst = text
			}
		}
	}
	log.Printf("theme: %s had branding keys set by hand, kept a copy at %s; took over name and slogan where valid and replaced the rest", overlayPath, backup)
	return nil
}

func (s *Store) commit(st State) error {
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.StateFile), 0o755); err != nil {
		return err
	}
	if err := writeAtomic(s.StateFile, b); err != nil {
		return err
	}
	if err := os.MkdirAll(s.AssetsDir, 0o755); err != nil {
		return err
	}
	overlayPath := filepath.Join(s.AssetsDir, "theme.json")
	var overlay KV
	raw, err := os.ReadFile(overlayPath)
	switch {
	case err == nil:
		if jsonErr := json.Unmarshal(raw, &overlay); jsonErr != nil {
			if err := moveAside(overlayPath, jsonErr); err != nil {
				return err
			}
		}
	case !errors.Is(err, os.ErrNotExist):
		return err
	}
	merged, err := Merge(overlay, s.Base, st)
	if err != nil {
		return err
	}
	out, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	return writeAtomic(overlayPath, out)
}

func (s *Store) removeUnused(st State) error {
	entries, err := os.ReadDir(s.AssetsDir)
	if err != nil {
		return err
	}
	used := map[string]bool{st.Logo: true, st.LogoDark: true, st.Favicon: true, st.Background: true}
	for _, e := range entries {
		if e.IsDir() || !assetName.MatchString(e.Name()) || used[e.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(s.AssetsDir, e.Name())); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// moveAside keeps an unreadable file for inspection and frees its name, so
// the store starts over instead of failing on every call.
func moveAside(path string, reason error) error {
	dst := freeName(fmt.Sprintf("%s.invalid-%d", path, time.Now().Unix()))
	if err := os.Rename(path, dst); err != nil {
		return fmt.Errorf("theme: %s: %v, and moving it aside failed: %w", path, reason, err)
	}
	log.Printf("theme: %s: %v, moved to %s", path, reason, dst)
	return nil
}

// freeName returns path, or path with -2, -3 and so on appended while that
// name is taken, so a second backup within the same second keeps the first.
func freeName(path string) string {
	name := path
	for i := 2; ; i++ {
		if _, err := os.Lstat(name); err != nil {
			return name
		}
		name = fmt.Sprintf("%s-%d", path, i)
	}
}

func field(st *State, k imagefmt.Kind) *string {
	switch k {
	case imagefmt.LogoDark:
		return &st.LogoDark
	case imagefmt.Favicon:
		return &st.Favicon
	case imagefmt.Background:
		return &st.Background
	default:
		return &st.Logo
	}
}

func writeAtomic(path string, data []byte) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
