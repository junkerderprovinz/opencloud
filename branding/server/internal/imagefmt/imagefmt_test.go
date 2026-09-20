package imagefmt

import "testing"

func TestRaster(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		ext  string
		ok   bool
	}{
		{"png", []byte("\x89PNG\r\n\x1a\nrest"), "png", true},
		{"jpeg", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, "jpg", true},
		{"gif87", []byte("GIF87a...."), "gif", true},
		{"gif89", []byte("GIF89a...."), "gif", true},
		{"webp", []byte("RIFF\x00\x00\x00\x00WEBPVP8 "), "webp", true},
		{"riff but not webp", []byte("RIFF\x00\x00\x00\x00WAVEfmt "), "", false},
		{"truncated png", []byte("\x89PNG"), "", false},
		{"pdf", []byte("%PDF-1.7"), "", false},
		{"svg is not raster", []byte("<svg/>"), "", false},
	}
	for _, c := range cases {
		ext, ok := Raster(c.data)
		if ext != c.ext || ok != c.ok {
			t.Errorf("%s: got (%q, %v), want (%q, %v)", c.name, ext, ok, c.ext, c.ok)
		}
	}
}

func TestLooksLikeSVG(t *testing.T) {
	cases := [][]byte{
		[]byte("<svg xmlns=\"http://www.w3.org/2000/svg\"/>"),
		append([]byte{0xef, 0xbb, 0xbf}, []byte("<?xml version=\"1.0\"?><svg/>")...),
		[]byte("\n  <!-- Generator: x --><svg/>"),
		[]byte("<!DOCTYPE svg><svg/>"),
	}
	for _, in := range cases {
		if !LooksLikeSVG(in) {
			t.Errorf("not detected: %q", in)
		}
	}
	for _, in := range []string{"%PDF-1.7", "<html></html>", "GIF89a"} {
		if LooksLikeSVG([]byte(in)) {
			t.Errorf("false positive: %q", in)
		}
	}
}

func TestKindsAndLimits(t *testing.T) {
	want := map[string]int64{"logo": 5 << 20, "logo-dark": 5 << 20, "favicon": 2 << 20, "background": 25 << 20}
	for name, limit := range want {
		k, ok := ParseKind(name)
		if !ok {
			t.Fatalf("ParseKind(%q) failed", name)
		}
		if MaxBytes(k) != limit {
			t.Errorf("MaxBytes(%s) = %d, want %d", name, MaxBytes(k), limit)
		}
	}
	if _, ok := ParseKind("wallpaper"); ok {
		t.Error("unknown kind accepted")
	}
}
