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

	maxDepth = 256 // element nesting limit, counting dropped subtrees
	maxUse   = 256 // <use> elements kept in the output, in document order
)

// ErrNotSVG is returned when the document root is not an <svg> element.
var ErrNotSVG = errors.New("svgclean: root element is not svg")

var errTooDeep = errors.New("svgclean: nesting too deep")

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

// allowedFuncs are the only CSS/SVG function calls permitted in an
// attribute value; anything else (image-set, calc, var, ...) is rejected.
var allowedFuncs = map[string]bool{
	"url": true, "rgb": true, "rgba": true, "hsl": true, "hsla": true,
	"matrix": true, "translate": true, "scale": true, "rotate": true,
	"skewx": true, "skewy": true,
}

var (
	funcCallRe = regexp.MustCompile(`(?i)([A-Za-z_-][A-Za-z0-9_-]*)\s*\(`)
	localURLRe = regexp.MustCompile(`(?i)^url\s*\(\s*['"]?#`)
	rasterData = regexp.MustCompile(`^data:image/(png|jpeg|gif|webp);base64,[A-Za-z0-9+/=\s]*$`)
)

// node is a kept element, still holding its children in document order so
// <use> can be resolved against them before anything is written out.
type node struct {
	name  string
	attrs []attr
	kids  []child
}

type attr struct {
	name  string
	value string
}

// child is either an element (elem set) or a run of character data (elem
// nil, text set), kept in source order so mixed text/element content
// round-trips.
type child struct {
	elem *node
	text []byte
}

func (n *node) attr(name string) (string, bool) {
	for _, a := range n.attrs {
		if a.name == name {
			return a.value, true
		}
	}
	return "", false
}

// Sanitize parses r and returns a rebuilt SVG that contains only allowlisted
// elements and attributes. Comments, processing instructions and DOCTYPE
// declarations are dropped; entity references make parsing fail. The
// document is parsed into a tree before anything is written, so <use>
// elements can be resolved against the elements they reference.
func Sanitize(r io.Reader) ([]byte, error) {
	root, err := parseTree(r)
	if err != nil {
		return nil, err
	}
	pruneUses(root)
	var out bytes.Buffer
	writeNode(&out, root, true)
	return out.Bytes(), nil
}

func parseTree(r io.Reader) (*node, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = true

	var root *node
	var stack []*node // kept ancestors, root first
	skip := 0         // depth inside a dropped subtree
	depth := 0        // raw element depth, including dropped subtrees
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
			depth++
			if depth > maxDepth {
				return nil, errTooDeep
			}
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
			n := &node{name: t.Name.Local, attrs: cleanAttrs(t.Name.Local, t.Attr)}
			if len(stack) > 0 {
				parent := stack[len(stack)-1]
				parent.kids = append(parent.kids, child{elem: n})
			} else {
				root = n
			}
			stack = append(stack, n)
		case xml.EndElement:
			depth--
			if skip > 0 {
				skip--
				continue
			}
			if len(stack) == 0 {
				continue
			}
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				return root, nil // root closed; ignore anything after it
			}
		case xml.CharData:
			if skip > 0 || len(stack) == 0 {
				continue
			}
			top := stack[len(stack)-1]
			if textElements[top.name] {
				top.kids = append(top.kids, child{text: append([]byte(nil), t...)})
			}
		}
	}
	if !sawRoot {
		return nil, ErrNotSVG
	}
	return root, nil
}

func cleanAttrs(element string, raw []xml.Attr) []attr {
	var out []attr
	seen := map[string]bool{}
	for _, a := range raw {
		name, value, ok := cleanAttr(element, a)
		if !ok || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, attr{name, value})
	}
	return out
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
	if !valueSafe(value) {
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
		if strings.ContainsAny(value, `<>@`) || strings.Contains(lower, "expression") || strings.Contains(lower, "javascript") {
			return "", "", false
		}
	}
	return local, value, true
}

// valueSafe rejects CSS escapes and comments, and any function call whose
// name is not in allowedFuncs; url() must additionally point at a local
// fragment. It applies to every attribute, not just style.
func valueSafe(value string) bool {
	if strings.ContainsRune(value, '\\') || strings.Contains(value, "/*") {
		return false
	}
	for _, m := range funcCallRe.FindAllStringSubmatchIndex(value, -1) {
		name := strings.ToLower(value[m[2]:m[3]])
		if !allowedFuncs[name] {
			return false
		}
		if name == "url" && !localURLRe.MatchString(value[m[0]:]) {
			return false
		}
	}
	return true
}

// pruneUses drops any <use> that does not point at a kept element whose own
// subtree contains no <use>, and keeps at most maxUse of the ones that
// qualify, in document order.
func pruneUses(root *node) {
	byID := map[string]*node{}
	indexIDs(root, byID)
	hasUse := map[*node]bool{}
	markUse(root, hasUse)
	kept := 0
	filterUses(root, byID, hasUse, &kept)
}

func indexIDs(n *node, byID map[string]*node) {
	if id, ok := n.attr("id"); ok {
		if _, exists := byID[id]; !exists {
			byID[id] = n
		}
	}
	for _, c := range n.kids {
		if c.elem != nil {
			indexIDs(c.elem, byID)
		}
	}
}

// markUse records, for every node, whether its subtree (itself included)
// contains a <use>, using the tree as parsed so the check does not depend
// on the order in which other <use> elements are later resolved.
func markUse(n *node, hasUse map[*node]bool) bool {
	has := n.name == "use"
	for _, c := range n.kids {
		if c.elem != nil && markUse(c.elem, hasUse) {
			has = true
		}
	}
	hasUse[n] = has
	return has
}

func filterUses(n *node, byID map[string]*node, hasUse map[*node]bool, kept *int) {
	kids := n.kids[:0]
	for _, c := range n.kids {
		if c.elem != nil && c.elem.name == "use" && !useAllowed(c.elem, byID, hasUse, kept) {
			continue
		}
		if c.elem != nil {
			filterUses(c.elem, byID, hasUse, kept)
		}
		kids = append(kids, c)
	}
	n.kids = kids
}

func useAllowed(n *node, byID map[string]*node, hasUse map[*node]bool, kept *int) bool {
	if *kept >= maxUse {
		return false
	}
	href, ok := n.attr("href")
	if !ok {
		href, ok = n.attr("xlink:href")
	}
	if !ok || !strings.HasPrefix(href, "#") {
		return false
	}
	target, ok := byID[href[1:]]
	if !ok || hasUse[target] {
		return false
	}
	*kept++
	return true
}

func writeNode(out *bytes.Buffer, n *node, root bool) {
	out.WriteString("<" + n.name)
	if root {
		out.WriteString(` xmlns="` + nsSVG + `" xmlns:xlink="` + nsXLink + `"`)
	}
	for _, a := range n.attrs {
		out.WriteString(" " + a.name + `="`)
		_ = xml.EscapeText(out, []byte(a.value))
		out.WriteString(`"`)
	}
	out.WriteString(">")
	for _, c := range n.kids {
		if c.elem != nil {
			writeNode(out, c.elem, false)
		} else {
			_ = xml.EscapeText(out, c.text)
		}
	}
	out.WriteString("</" + n.name + ">")
}
