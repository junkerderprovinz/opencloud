package svgclean

import (
	"errors"
	"strings"
	"testing"
)

func mustClean(t *testing.T, in string) string {
	t.Helper()
	out, err := Sanitize(strings.NewReader(in))
	if err != nil {
		t.Fatalf("Sanitize: %v", err)
	}
	return string(out)
}

func TestKeepsDrawing(t *testing.T) {
	out := mustClean(t, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10"><defs><linearGradient id="g"><stop offset="0" stop-color="#fff"/></linearGradient></defs><g fill="url(#g)"><path d="M0 0h10v10z"/><text x="1">Hi &amp; bye</text></g></svg>`)
	for _, want := range []string{`viewBox="0 0 10 10"`, `<path d="M0 0h10v10z">`, `fill="url(#g)"`, `<stop offset="0" stop-color="#fff">`, `Hi &amp; bye`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
	if !strings.HasPrefix(out, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"`) {
		t.Errorf("root namespaces not normalised: %s", out)
	}
}

func TestRemovesActiveContent(t *testing.T) {
	cases := map[string]string{
		"script":         `<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script><rect/></svg>`,
		"onload":         `<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"><rect/></svg>`,
		"href js":        `<svg xmlns="http://www.w3.org/2000/svg"><use href="javascript:alert(1)"/></svg>`,
		"xlink js":       `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="javascript:alert(1)"/></svg>`,
		"foreignObject":  `<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><body xmlns="http://www.w3.org/1999/xhtml"><script>alert(1)</script></body></foreignObject></svg>`,
		"anchor animate": `<svg xmlns="http://www.w3.org/2000/svg"><a><animate attributeName="href" to="javascript:alert(1)"/></a></svg>`,
		"set":            `<svg xmlns="http://www.w3.org/2000/svg"><set attributeName="onmouseover" to="alert(1)"/></svg>`,
		"style element":  `<svg xmlns="http://www.w3.org/2000/svg"><style>@import url(https://evil.example/x.css)</style></svg>`,
		"style url":      `<svg xmlns="http://www.w3.org/2000/svg"><rect style="fill:url(https://evil.example/x)"/></svg>`,
		"external fill":  `<svg xmlns="http://www.w3.org/2000/svg"><rect fill="url(https://evil.example/x#a)"/></svg>`,
		"svg data image": `<svg xmlns="http://www.w3.org/2000/svg"><image href="data:image/svg+xml;base64,PHN2Zz4="/></svg>`,
		"remote image":   `<svg xmlns="http://www.w3.org/2000/svg"><image href="https://evil.example/pixel.png"/></svg>`,
		"xhtml element":  `<svg xmlns="http://www.w3.org/2000/svg" xmlns:h="http://www.w3.org/1999/xhtml"><h:iframe src="javascript:alert(1)"/></svg>`,
	}
	bad := []string{"script", "alert", "javascript", "foreignObject", "animate", "<set", "evil.example", "onload", "image/svg+xml", "@import", "iframe"}
	for name, in := range cases {
		out := mustClean(t, in)
		for _, b := range bad {
			if strings.Contains(out, b) {
				t.Errorf("%s: %q survived in %s", name, b, out)
			}
		}
	}
}

func TestKeepsRasterDataImageAndLocalRefs(t *testing.T) {
	out := mustClean(t, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><image href="data:image/png;base64,iVBORw0KGgo=" width="1" height="1"/><use xlink:href="#shape"/></svg>`)
	for _, want := range []string{`href="data:image/png;base64,iVBORw0KGgo="`, `xlink:href="#shape"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestRejectsEntities(t *testing.T) {
	in := `<?xml version="1.0"?><!DOCTYPE svg [<!ENTITY x "boom">]><svg xmlns="http://www.w3.org/2000/svg"><text>&x;</text></svg>`
	if _, err := Sanitize(strings.NewReader(in)); err == nil {
		t.Fatal("expected an error for an entity reference")
	}
}

func TestDropsDoctypeAndComments(t *testing.T) {
	out := mustClean(t, `<?xml version="1.0"?><!-- Generator: Illustrator --><!DOCTYPE svg PUBLIC "-//W3C//DTD SVG 1.1//EN" "http://www.w3.org/Graphics/SVG/1.1/DTD/svg11.dtd"><svg xmlns="http://www.w3.org/2000/svg"><rect/></svg>`)
	for _, bad := range []string{"DOCTYPE", "Graphics/SVG", "Generator", "<?xml"} {
		if strings.Contains(out, bad) {
			t.Errorf("%q survived: %s", bad, out)
		}
	}
}

func TestRejectsNonSVGRoot(t *testing.T) {
	if _, err := Sanitize(strings.NewReader(`<html><body/></html>`)); !errors.Is(err, ErrNotSVG) {
		t.Fatalf("want ErrNotSVG, got %v", err)
	}
	if _, err := Sanitize(strings.NewReader(``)); !errors.Is(err, ErrNotSVG) {
		t.Fatalf("empty input: want ErrNotSVG, got %v", err)
	}
}
