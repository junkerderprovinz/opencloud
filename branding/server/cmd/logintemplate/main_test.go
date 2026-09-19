package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const signIn = `<!doctype html><html lang="en"><head><meta property="csp-nonce" content="__CSP_NONCE__"><title>Sign in - OpenCloud</title><script defer="defer" src="/signin/v1/static/js/main.1a2b3c4d.js"></script></head><body><main id="root" data-bg-img="__BG_IMG_URL__"></main></body></html>`

// fakeBinary surrounds the template with binary noise and another embedded
// page, the way it sits among the other assets of the opencloud binary.
func fakeBinary(pages ...string) []byte {
	b := []byte("\x7fELF\x02\x01\x01\x00runtime.main\x00<!doctype html><html><body>other page</body></html>\x00\x13\x37")
	for _, p := range pages {
		b = append(b, p...)
		b = append(b, "\x00\xff\xfe go.buildid"...)
	}
	return b
}

func TestExtractsTheSignInPageWithTheBrandingScript(t *testing.T) {
	got, err := extract(fakeBinary(signIn))
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Replace(signIn, "</head>", `<script defer="defer" src="/brandingsvc/login.js"></script></head>`, 1)
	if string(got) != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestRefusesAnUnexpectedTemplate(t *testing.T) {
	cases := map[string][]byte{
		"no template":       fakeBinary(),
		"two templates":     fakeBinary(signIn, signIn),
		"no doctype":        fakeBinary(strings.Replace(signIn, "<!doctype html>", "", 1)),
		"no end tag":        fakeBinary(strings.Replace(signIn, "</html>", "", 1), "<!doctype html><html></html>"),
		"no CSP nonce":      fakeBinary(strings.Replace(signIn, "__CSP_NONCE__", "", 1)),
		"no main bundle":    fakeBinary(strings.Replace(signIn, "static/js/main.", "static/js/app.", 1)),
		"no head end tag":   fakeBinary(strings.Replace(signIn, "</head>", "", 1)),
		"two head end tags": fakeBinary(strings.Replace(signIn, "</head>", "</head></head>", 1)),
	}
	for name, bin := range cases {
		if page, err := extract(bin); err == nil {
			t.Errorf("%s: want an error, got %q", name, page)
		}
	}
}

func TestWritesThePageBelowTheAssetPath(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "opencloud")
	if err := os.WriteFile(bin, fakeBinary(signIn), 0o755); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "idp", "identifier", "index.html")

	if err := run([]string{bin, out}); err != nil {
		t.Fatal(err)
	}
	page, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(page), `src="/brandingsvc/login.js"`) {
		t.Errorf("written page misses the script: %s", page)
	}
}

func TestFailsWithoutArguments(t *testing.T) {
	if err := run(nil); err == nil {
		t.Fatal("want a usage error")
	}
}
