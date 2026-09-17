package theme

import (
	"encoding/json"
	"reflect"
	"testing"
)

func testBase(t *testing.T) KV {
	t.Helper()
	var kv KV
	src := `{"common":{"name":"OpenCloud"},"clients":{"web":{"defaults":{"logo":"themes/opencloud/assets/logo.svg"},"themes":[{"isDark":false,"label":"Light Theme","designTokens":{"roles":{"primary":"#00677f"}}},{"isDark":true,"label":"Dark Theme","logo":"themes/opencloud/assets/logo-dark-mode.svg","designTokens":{"roles":{"primary":"#5cd5fb"}}}]}}}`
	if err := json.Unmarshal([]byte(src), &kv); err != nil {
		t.Fatal(err)
	}
	return kv
}

func TestMergeNameOnlyLeavesThemesAlone(t *testing.T) {
	got, err := Merge(nil, testBase(t), State{Name: "Knight Cloud"})
	if err != nil {
		t.Fatal(err)
	}
	if v, _ := getPath(got, []string{"common", "name"}); v != "Knight Cloud" {
		t.Errorf("name = %v", v)
	}
	if _, ok := getPath(got, []string{"clients", "web", "themes"}); ok {
		t.Error("themes list must not be copied without a logo")
	}
}

func TestMergeLightLogoCoversDarkTheme(t *testing.T) {
	got, err := Merge(nil, testBase(t), State{Logo: "logo-aaaaaaaaaaaa.png"})
	if err != nil {
		t.Fatal(err)
	}
	want := "themes/_branding/logo-aaaaaaaaaaaa.png"
	for _, p := range [][]string{{"common", "logo"}, {"clients", "web", "defaults", "logo"}, {"clients", "web", "defaults", "logoMobile"}} {
		if v, _ := getPath(got, p); v != want {
			t.Errorf("%v = %v, want %s", p, v, want)
		}
	}
	themes, _ := getPath(got, []string{"clients", "web", "themes"})
	dark := themes.([]any)[1].(map[string]any)
	if dark["logo"] != want || dark["logoMobile"] != want {
		t.Errorf("dark theme logo = %v / %v", dark["logo"], dark["logoMobile"])
	}
	primary := dark["designTokens"].(map[string]any)["roles"].(map[string]any)["primary"]
	if primary != "#5cd5fb" {
		t.Errorf("design tokens not copied from base: %v", primary)
	}
}

func TestMergeDarkLogoOnly(t *testing.T) {
	got, err := Merge(nil, testBase(t), State{LogoDark: "logo-dark-bbbbbbbbbbbb.svg"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := getPath(got, []string{"common", "logo"}); ok {
		t.Error("light logo must stay upstream")
	}
	themes, _ := getPath(got, []string{"clients", "web", "themes"})
	list := themes.([]any)
	if _, ok := list[0].(map[string]any)["logo"]; ok {
		t.Error("light theme entry must not get a logo")
	}
	if list[1].(map[string]any)["logo"] != "themes/_branding/logo-dark-bbbbbbbbbbbb.svg" {
		t.Error("dark logo not set")
	}
}

func TestMergeKeepsForeignKeysAndClearsOwnKeys(t *testing.T) {
	overlay := KV{}
	setPath(overlay, []string{"common", "urls", "imprint"}, "https://example.org/imprint")
	setPath(overlay, []string{"common", "logo"}, "themes/_branding/old.png")
	got, err := Merge(overlay, testBase(t), State{})
	if err != nil {
		t.Fatal(err)
	}
	want := KV{"common": map[string]any{"urls": map[string]any{"imprint": "https://example.org/imprint"}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestMergeDoesNotModifyBase(t *testing.T) {
	base := testBase(t)
	before, _ := json.Marshal(base)
	if _, err := Merge(nil, base, State{Logo: "logo-aaaaaaaaaaaa.png"}); err != nil {
		t.Fatal(err)
	}
	after, _ := json.Marshal(base)
	if string(before) != string(after) {
		t.Error("base theme was modified")
	}
}

func TestMergeNeedsThemesInBase(t *testing.T) {
	if _, err := Merge(nil, KV{}, State{Logo: "logo-aaaaaaaaaaaa.png"}); err == nil {
		t.Fatal("expected an error for a base theme without clients.web.themes")
	}
}
