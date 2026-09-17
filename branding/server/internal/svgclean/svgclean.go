// Package svgclean rebuilds an uploaded SVG from an allowlist of elements
// and attributes. Nothing is passed through: an SVG opened directly on the
// OpenCloud origin runs under a CSP that allows inline script, and the web
// client keeps its tokens in browser storage.
package svgclean

import (
	"bytes"
	"encoding/xml"
	"errors"
	"io"
	"regexp"
	"strings"
)

const (
	nsSVG   = "http://www.w3.org/2000/svg"
	nsXLink = "http://www.w3.org/1999/xlink"
)

// ErrNotSVG is returned when the document root is not an <svg> element.
var ErrNotSVG = errors.New("svgclean: root element is not svg")

var allowedElements = map[string]bool{
	"svg": true, "g": true, "defs": true, "symbol": true, "use": true,
	"title": true, "desc": true,
	"path": true, "rect": true, "circle": true, "ellipse": true,
	"line": true, "polyline": true, "polygon": true,
	"text": true, "tspan": true,
	"linearGradient": true, "radialGradient": true, "stop": true,
	"clipPath": true, "mask": true, "image": true,
}

// textElements are the only elements whose character data is kept.
var textElements = map[string]bool{"text": true, "tspan": true, "title": true, "desc": true}

var allowedAttrs = map[string]bool{
	"id": true, "class": true, "viewBox": true, "preserveAspectRatio": true, "version": true,
	"width": true, "height": true, "x": true, "y": true, "x1": true, "y1": true, "x2": true, "y2": true,
	"cx": true, "cy": true, "r": true, "rx": true, "ry": true, "fx": true, "fy": true,
	"d": true, "points": true, "pathLength": true, "transform": true,
	"fill": true, "fill-opacity": true, "fill-rule": true,
	"stroke": true, "stroke-width": true, "stroke-opacity": true, "stroke-linecap": true,
	"stroke-linejoin": true, "stroke-miterlimit": true, "stroke-dasharray": true, "stroke-dashoffset": true,
	"opacity": true, "clip-path": true, "clip-rule": true, "mask": true,
	"gradientUnits": true, "gradientTransform": true, "spreadMethod": true, "offset": true,
	"stop-color": true, "stop-opacity": true,
	"clipPathUnits": true, "maskUnits": true, "maskContentUnits": true,
	"font-family": true, "font-size": true, "font-weight": true, "font-style": true,
	"text-anchor": true, "dominant-baseline": true, "letter-spacing": true, "dx": true, "dy": true,
	"style": true, "href": true,
}

var (
	anyURL     = regexp.MustCompile(`(?i)url\s*\(`)
	localURL   = regexp.MustCompile(`(?i)url\s*\(\s*['"]?#`)
	rasterData = regexp.MustCompile(`^data:image/(png|jpeg|gif|webp);base64,[A-Za-z0-9+/=\s]*$`)
)

// Sanitize parses r and returns a rebuilt SVG that contains only allowlisted
// elements and attributes. Comments, processing instructions and DOCTYPE
// declarations are dropped; entity references make parsing fail.
func Sanitize(r io.Reader) ([]byte, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = true
	var out bytes.Buffer
	var open []string // allowed elements currently open
	skip := 0         // depth inside a dropped element
	sawRoot := false

	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			if skip > 0 {
				skip++
				continue
			}
			inSVG := t.Name.Space == nsSVG || t.Name.Space == ""
			if !sawRoot {
				if t.Name.Local != "svg" || !inSVG {
					return nil, ErrNotSVG
				}
				sawRoot = true
			}
			if !inSVG || !allowedElements[t.Name.Local] {
				skip = 1
				continue
			}
			writeStart(&out, t, len(open) == 0)
			open = append(open, t.Name.Local)
		case xml.EndElement:
			if skip > 0 {
				skip--
				continue
			}
			if len(open) == 0 {
				continue
			}
			out.WriteString("</" + open[len(open)-1] + ">")
			open = open[:len(open)-1]
		case xml.CharData:
			if skip == 0 && len(open) > 0 && textElements[open[len(open)-1]] {
				if err := xml.EscapeText(&out, t); err != nil {
					return nil, err
				}
			}
		}
	}
	if !sawRoot {
		return nil, ErrNotSVG
	}
	return out.Bytes(), nil
}

func writeStart(out *bytes.Buffer, t xml.StartElement, root bool) {
	out.WriteString("<" + t.Name.Local)
	if root {
		out.WriteString(` xmlns="` + nsSVG + `" xmlns:xlink="` + nsXLink + `"`)
	}
	for _, a := range t.Attr {
		name, value, ok := cleanAttr(t.Name.Local, a)
		if !ok {
			continue
		}
		out.WriteString(" " + name + `="`)
		_ = xml.EscapeText(out, []byte(value))
		out.WriteString(`"`)
	}
	out.WriteString(">")
}

func cleanAttr(element string, a xml.Attr) (string, string, bool) {
	local, value := a.Name.Local, a.Value
	xlink := false
	switch a.Name.Space {
	case "":
	case nsXLink, "xlink":
		xlink = true
	default:
		return "", "", false // xmlns declarations, xml:*, editor namespaces
	}
	if local == "xmlns" || !allowedAttrs[local] || (xlink && local != "href") {
		return "", "", false
	}
	if local == "href" {
		if !strings.HasPrefix(value, "#") && !(element == "image" && rasterData.MatchString(value)) {
			return "", "", false
		}
		if xlink {
			return "xlink:href", value, true
		}
		return "href", value, true
	}
	if local == "style" {
		lower := strings.ToLower(value)
		if strings.ContainsAny(value, `\<>@`) || strings.Contains(lower, "expression") || strings.Contains(lower, "javascript") {
			return "", "", false
		}
	}
	if len(anyURL.FindAllStringIndex(value, -1)) != len(localURL.FindAllStringIndex(value, -1)) {
		return "", "", false
	}
	return local, value, true
}
