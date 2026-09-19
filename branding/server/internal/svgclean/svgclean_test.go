package svgclean

import (
	"errors"
	"fmt"
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
	out := mustClean(t, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink"><image href="data:image/png;base64,iVBORw0KGgo=" width="1" height="1"/><path id="shape" d="M0 0h1v1z"/><use xlink:href="#shape"/></svg>`)
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

func TestBlocksCSSEscapesAndFunctions(t *testing.T) {
	vectors := []string{
		`<svg xmlns="http://www.w3.org/2000/svg"><rect fill="u\rl(http://evil.example/p.svg#a)"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><rect stroke="u\rl(http://evil.example/p.svg#a)"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><rect clip-path="u\rl(http://evil.example/p.svg#a)"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><rect mask="u\rl(http://evil.example/p.svg#a)"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><rect mask="u\72 l(http://evil.example/m.svg#a)"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><rect mask="image-set('http://evil.example/m.png' 1x)"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg" style="background-image:image-set('http://evil.example/b.png' 1x)"><rect/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><rect style="mask-image:image-set('http://evil.example/m.png' 1x)"/></svg>`,
		`<svg xmlns="http://www.w3.org/2000/svg"><rect style="-webkit-mask-image:-webkit-image-set('http://evil.example/m.png' 1x)"/></svg>`,
	}
	for _, in := range vectors {
		out := mustClean(t, in)
		for _, bad := range []string{"evil.example", "image-set", `\`} {
			if strings.Contains(out, bad) {
				t.Errorf("%q survived cleaning %s -> %s", bad, in, out)
			}
		}
	}
}

func TestKeepsSafeFunctionsAndStyle(t *testing.T) {
	out := mustClean(t, `<svg xmlns="http://www.w3.org/2000/svg"><rect fill="rgb(10, 20, 30)" transform="matrix(1 0 0 1 5 5) rotate(45)"/><rect fill="url(#g)" style="fill:#fff;font-family:'Open Sans'"/></svg>`)
	for _, want := range []string{`fill="rgb(10, 20, 30)"`, `transform="matrix(1 0 0 1 5 5) rotate(45)"`, `fill="url(#g)"`, `style="fill:#fff;font-family:&#39;Open Sans&#39;"`} {
		if !strings.Contains(out, want) {
			t.Errorf("missing %q in %s", want, out)
		}
	}
}

func TestNestedUseTargetingAnotherUseIsDropped(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"><rect id="a0" width="1" height="1"/>`)
	for i := 1; i <= 22; i++ {
		fmt.Fprintf(&b, `<g id="a%d"><use href="#a%d"/><use href="#a%d"/></g>`, i, i-1, i-1)
	}
	b.WriteString(`</svg>`)
	out := mustClean(t, b.String())
	if n := strings.Count(out, "<use"); n > 2 {
		t.Errorf("want at most 2 kept <use> elements, got %d: %s", n, out)
	}
}

func TestUseKeptWhenTargetHasNoUse(t *testing.T) {
	out := mustClean(t, `<svg xmlns="http://www.w3.org/2000/svg"><path id="shape" d="M0 0h1v1z"/><use href="#shape"/></svg>`)
	if !strings.Contains(out, "<use") || !strings.Contains(out, `href="#shape"`) {
		t.Errorf("use element dropped unexpectedly: %s", out)
	}
}

func TestUseDroppedWhenTargetMissing(t *testing.T) {
	out := mustClean(t, `<svg xmlns="http://www.w3.org/2000/svg"><use href="#missing"/></svg>`)
	if strings.Contains(out, "<use") {
		t.Errorf("use with missing target survived: %s", out)
	}
}

func TestUseCapAt256(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg"><path id="shape" d="M0 0h1v1z"/>`)
	for i := 0; i < 300; i++ {
		b.WriteString(`<use href="#shape"/>`)
	}
	b.WriteString(`</svg>`)
	out := mustClean(t, b.String())
	if n := strings.Count(out, "<use"); n != 256 {
		t.Errorf("want exactly 256 kept <use> elements, got %d", n)
	}
}

func TestDedupsAttributes(t *testing.T) {
	out := mustClean(t, `<svg xmlns="http://www.w3.org/2000/svg"><rect fill="red" fill="blue"/></svg>`)
	if n := strings.Count(out, `fill="`); n != 1 {
		t.Errorf("want fill= exactly once, got %d: %s", n, out)
	}

	out2 := mustClean(t, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:x="http://www.w3.org/1999/xlink"><path id="a" d="M0 0"/><use xlink:href="#a" x:href="#a"/></svg>`)
	if n := strings.Count(out2, `xlink:href="`); n != 1 {
		t.Errorf("want xlink:href= exactly once, got %d: %s", n, out2)
	}
}

func TestIgnoresContentAfterRoot(t *testing.T) {
	out := mustClean(t, `<svg xmlns="http://www.w3.org/2000/svg"><rect/></svg><rect/>`)
	if n := strings.Count(out, "<rect"); n != 1 {
		t.Errorf("want exactly one <rect kept, got %d: %s", n, out)
	}
}

func TestRejectsExcessiveNesting(t *testing.T) {
	var b strings.Builder
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg">`)
	for i := 0; i < 300; i++ {
		b.WriteString(`<g>`)
	}
	for i := 0; i < 300; i++ {
		b.WriteString(`</g>`)
	}
	b.WriteString(`</svg>`)
	if _, err := Sanitize(strings.NewReader(b.String())); err == nil {
		t.Fatal("want an error for excessive nesting")
	}
}
