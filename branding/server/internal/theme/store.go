package theme

import (
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
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
	return s.load()
}

// Regenerate rewrites the overlay from the saved state. brandingd calls it on
// start, because a new image can bring a new base theme.
func (s *Store) Regenerate() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
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

func (s *Store) load() (State, error) {
	var st State
	b, err := os.ReadFile(s.StateFile)
	if errors.Is(err, os.ErrNotExist) {
		return st, nil
	}
	if err != nil {
		return st, err
	}
	if err := json.Unmarshal(b, &st); err != nil {
		return st, fmt.Errorf("theme: %s: %w", s.StateFile, err)
	}
	return st, nil
}

func (s *Store) update(change func(*State) error) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, err := s.load()
	if err != nil {
		return State{}, err
	}
	if err := change(&st); err != nil {
		return State{}, err
	}
	return st, s.commit(st)
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
		if err := json.Unmarshal(raw, &overlay); err != nil {
			return fmt.Errorf("theme: %s: %w", overlayPath, err)
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
	if err := writeAtomic(overlayPath, out); err != nil {
		return err
	}
	return s.removeUnused(st)
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
