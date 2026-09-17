// Package theme keeps the branding state and turns it into OpenCloud's
// themes/_branding/theme.json overlay.
package theme

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
)

// KV is a decoded JSON object.
type KV = map[string]any

// AssetPrefix is how theme.json refers to files in the _branding folder.
const AssetPrefix = "themes/_branding/"

// ownedPaths are the overlay keys this service writes. Other keys, such as
// ones OpenCloud's own /branding/logo endpoint may have added, stay as they are.
var ownedPaths = [][]string{
	{"common", "name"},
	{"common", "slogan"},
	{"common", "logo"},
	{"clients", "web", "defaults", "logo"},
	{"clients", "web", "defaults", "logoMobile"},
	{"clients", "web", "defaults", "favicon"},
	{"clients", "web", "themes"},
}

// LoadBase reads the upstream theme.json baked into the image.
func LoadBase(path string) (KV, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var kv KV
	if err := json.Unmarshal(b, &kv); err != nil {
		return nil, fmt.Errorf("theme: %s: %w", path, err)
	}
	if _, err := baseThemes(kv); err != nil {
		return nil, err
	}
	return kv, nil
}

// Merge clears every owned key from overlay and sets the ones st defines.
// base is only read.
func Merge(overlay, base KV, st State) (KV, error) {
	if overlay == nil {
		overlay = KV{}
	}
	for _, p := range ownedPaths {
		deletePath(overlay, p)
	}
	if st.Name != "" {
		setPath(overlay, []string{"common", "name"}, st.Name)
	}
	if st.Slogan != "" {
		setPath(overlay, []string{"common", "slogan"}, st.Slogan)
	}
	if st.Favicon != "" {
		setPath(overlay, []string{"clients", "web", "defaults", "favicon"}, AssetPrefix+st.Favicon)
	}
	if st.Logo != "" {
		for _, p := range [][]string{{"common", "logo"}, {"clients", "web", "defaults", "logo"}, {"clients", "web", "defaults", "logoMobile"}} {
			setPath(overlay, p, AssetPrefix+st.Logo)
		}
	}
	dark := st.LogoDark
	if dark == "" {
		dark = st.Logo
	}
	if dark != "" {
		// The dark theme names its own logo inside the themes list, and
		// OpenCloud replaces lists instead of merging them, so the whole list
		// is copied from the base theme of the bundled version.
		themes, err := baseThemes(base)
		if err != nil {
			return nil, err
		}
		list := make([]any, 0, len(themes))
		for _, th := range themes {
			if isDark, _ := th["isDark"].(bool); isDark {
				th["logo"] = AssetPrefix + dark
				th["logoMobile"] = AssetPrefix + dark
			}
			list = append(list, th)
		}
		setPath(overlay, []string{"clients", "web", "themes"}, list)
	}
	prune(overlay)
	return overlay, nil
}

// baseThemes returns a deep copy of base's clients.web.themes.
func baseThemes(base KV) ([]KV, error) {
	raw, ok := getPath(base, []string{"clients", "web", "themes"})
	if !ok {
		return nil, errors.New("theme: base theme has no clients.web.themes")
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var themes []KV
	if err := json.Unmarshal(b, &themes); err != nil {
		return nil, fmt.Errorf("theme: clients.web.themes: %w", err)
	}
	if len(themes) == 0 {
		return nil, errors.New("theme: base theme lists no themes")
	}
	return themes, nil
}

func getPath(kv KV, path []string) (any, bool) {
	var cur any = kv
	for _, p := range path {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[p]; !ok {
			return nil, false
		}
	}
	return cur, true
}

func setPath(kv KV, path []string, v any) {
	m := kv
	for _, p := range path[:len(path)-1] {
		next, ok := m[p].(map[string]any)
		if !ok {
			next = map[string]any{}
			m[p] = next
		}
		m = next
	}
	m[path[len(path)-1]] = v
}

func deletePath(kv KV, path []string) {
	m := kv
	for _, p := range path[:len(path)-1] {
		next, ok := m[p].(map[string]any)
		if !ok {
			return
		}
		m = next
	}
	delete(m, path[len(path)-1])
}

// prune drops objects that became empty, so an unbranded overlay is {}.
func prune(m map[string]any) {
	for k, v := range m {
		if child, ok := v.(map[string]any); ok {
			prune(child)
			if len(child) == 0 {
				delete(m, k)
			}
		}
	}
}
