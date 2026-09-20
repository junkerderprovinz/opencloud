// Package imagefmt names the image slots of the branding app, their upload
// limits and the formats accepted by magic bytes.
package imagefmt

import "bytes"

// Kind is an image slot in the branding state.
type Kind string

const (
	Logo       Kind = "logo"
	LogoDark   Kind = "logo-dark"
	Favicon    Kind = "favicon"
	Background Kind = "background"
)

// ParseKind maps a URL segment to a Kind.
func ParseKind(s string) (Kind, bool) {
	switch k := Kind(s); k {
	case Logo, LogoDark, Favicon, Background:
		return k, true
	}
	return "", false
}

// MaxBytes is the upload limit for a slot.
func MaxBytes(k Kind) int64 {
	switch k {
	case Favicon:
		return 2 << 20
	case Background:
		return 25 << 20
	default:
		return 5 << 20
	}
}

// Raster returns the file extension for a PNG, JPEG, GIF or WebP payload.
// Only the magic bytes count; a client-supplied name or type is never used.
func Raster(b []byte) (string, bool) {
	switch {
	case bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")):
		return "png", true
	case bytes.HasPrefix(b, []byte{0xFF, 0xD8, 0xFF}):
		return "jpg", true
	case bytes.HasPrefix(b, []byte("GIF87a")), bytes.HasPrefix(b, []byte("GIF89a")):
		return "gif", true
	case len(b) >= 12 && bytes.Equal(b[0:4], []byte("RIFF")) && bytes.Equal(b[8:12], []byte("WEBP")):
		return "webp", true
	}
	return "", false
}

// LooksLikeSVG reports whether the payload starts like an XML or SVG
// document and should go through the SVG sanitiser.
func LooksLikeSVG(b []byte) bool {
	t := bytes.TrimLeft(b, " \t\r\n\xef\xbb\xbf")
	return bytes.HasPrefix(t, []byte("<?xml")) || bytes.HasPrefix(t, []byte("<svg")) || bytes.HasPrefix(t, []byte("<!"))
}
