// Command logintemplate copies the sign-in page template out of an opencloud
// binary and adds the branding script to it. The IDP serves the copy through
// IDP_ASSET_PATH in place of its embedded one.
package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

var (
	marker    = []byte(`data-bg-img="__BG_IMG_URL__"`)
	doctype   = []byte("<!doctype html>")
	htmlEnd   = []byte("</html>")
	headEnd   = []byte("</head>")
	scriptTag = []byte(`<script defer="defer" src="/brandingsvc/login.js"></script>`)
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		log.Fatalf("logintemplate: %v", err)
	}
}

func run(args []string) error {
	if len(args) != 2 {
		return errors.New("usage: logintemplate <opencloud binary> <output file>")
	}
	bin, err := os.ReadFile(args[0])
	if err != nil {
		return err
	}
	page, err := extract(bin)
	if err != nil {
		return fmt.Errorf("%s: %w", args[0], err)
	}
	if err := os.MkdirAll(filepath.Dir(args[1]), 0o755); err != nil {
		return err
	}
	return os.WriteFile(args[1], page, 0o644)
}

// extract finds the template by its background placeholder. The page names
// hashed bundle files, so it only fits the binary it came from, and any change
// in its shape fails here rather than on a login page.
func extract(bin []byte) ([]byte, error) {
	if n := bytes.Count(bin, marker); n != 1 {
		return nil, fmt.Errorf("found %d sign-in templates, want 1", n)
	}
	at := bytes.Index(bin, marker)
	start := bytes.LastIndex(bin[:at], doctype)
	end := bytes.Index(bin[at:], htmlEnd)
	if start < 0 || end < 0 {
		return nil, errors.New("sign-in template has no doctype or no closing html tag")
	}
	page := bin[start : at+end+len(htmlEnd)]
	if bytes.Count(page, doctype) != 1 || bytes.Count(page, htmlEnd) != 1 {
		return nil, errors.New("sign-in template runs into a neighbouring file")
	}
	if !bytes.Contains(page, []byte("__CSP_NONCE__")) || !bytes.Contains(page, []byte("static/js/main.")) {
		return nil, errors.New("sign-in template lacks the CSP nonce or the main bundle")
	}
	if bytes.Count(page, headEnd) != 1 {
		return nil, errors.New("sign-in template needs exactly one closing head tag")
	}
	head := bytes.Index(page, headEnd)
	out := make([]byte, 0, len(page)+len(scriptTag))
	out = append(out, page[:head]...)
	out = append(out, scriptTag...)
	return append(out, page[head:]...), nil
}
