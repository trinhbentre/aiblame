package render

import (
	"encoding/json"
	"fmt"
	"html"
	"strings"
)

// Named colours accepted by --color (shields.io names) mapped to hex.
var namedColors = map[string]string{
	"brightgreen": "4c1", "green": "97ca00", "yellowgreen": "a4a61d", "yellow": "dfb317",
	"orange": "fe7d37", "red": "e05d44", "blue": "007ec6", "lightgrey": "9f9f9f", "lightgray": "9f9f9f",
	"grey": "555", "gray": "555", "blueviolet": "8a2be2", "violet": "7c3aed", "purple": "7c3aed",
	"success": "4c1", "important": "fe7d37", "critical": "e05d44", "informational": "007ec6", "inactive": "9f9f9f",
	"black": "000", "white": "fff", "pink": "ff69b4", "teal": "008080",
}

// DefaultBadgeColor is the violet used when no colour is configured.
const DefaultBadgeColor = "7c3aed"

// ResolveColor turns a name or hex string into a 3/6-digit hex without '#'.
// "auto" maps the percentage onto a neutral five-step scale.
func ResolveColor(c string, pct float64) string {
	c = strings.TrimSpace(strings.ToLower(c))
	if c == "" {
		return DefaultBadgeColor
	}
	if c == "auto" {
		switch {
		case pct <= 0:
			return namedColors["lightgrey"]
		case pct < 25:
			return namedColors["blue"]
		case pct < 50:
			return "5b6cf9"
		case pct < 75:
			return DefaultBadgeColor
		default:
			return "9333ea"
		}
	}
	if v, ok := namedColors[c]; ok {
		return v
	}
	c = strings.TrimPrefix(c, "#")
	if isHex(c) && (len(c) == 3 || len(c) == 6) {
		return c
	}
	return DefaultBadgeColor
}

func isHex(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f' || r >= 'A' && r <= 'F') {
			return false
		}
	}
	return true
}

// BadgeSVG renders a shields.io-style badge. Text widths are pinned with
// textLength so the layout is identical in every renderer.
func BadgeSVG(label, message, color, style string) string {
	color = ResolveColor(color, 0)
	lw := textWidth(label)
	mw := textWidth(message)
	w := lw + mw
	h := 20
	rx := 3
	if style == "flat-square" || style == "square" {
		rx = 0
	}
	esc := html.EscapeString
	title := esc(label + ": " + message)
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="%d" height="%d" role="img" aria-label="%s">`, w, h, title)
	fmt.Fprintf(&b, `<title>%s</title>`, title)
	if rx > 0 {
		fmt.Fprintf(&b, `<linearGradient id="s" x2="0" y2="100%%"><stop offset="0" stop-color="#bbb" stop-opacity=".1"/><stop offset="1" stop-opacity=".1"/></linearGradient>`)
		fmt.Fprintf(&b, `<clipPath id="r"><rect width="%d" height="%d" rx="%d" fill="#fff"/></clipPath><g clip-path="url(#r)">`, w, h, rx)
	} else {
		b.WriteString(`<g>`)
	}
	fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="#555"/>`, lw, h)
	fmt.Fprintf(&b, `<rect x="%d" width="%d" height="%d" fill="#%s"/>`, lw, mw, h, color)
	if rx > 0 {
		fmt.Fprintf(&b, `<rect width="%d" height="%d" fill="url(#s)"/>`, w, h)
	}
	b.WriteString(`</g>`)
	b.WriteString(`<g fill="#fff" text-anchor="middle" font-family="Verdana,Geneva,DejaVu Sans,sans-serif" text-rendering="geometricPrecision" font-size="110">`)
	// label
	fmt.Fprintf(&b, `<text aria-hidden="true" x="%d" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="%d">%s</text>`, lw*5, (lw-10)*10, esc(label))
	fmt.Fprintf(&b, `<text x="%d" y="140" transform="scale(.1)" fill="#fff" textLength="%d">%s</text>`, lw*5, (lw-10)*10, esc(label))
	// message
	fmt.Fprintf(&b, `<text aria-hidden="true" x="%d" y="150" fill="#010101" fill-opacity=".3" transform="scale(.1)" textLength="%d">%s</text>`, (lw*2+mw)*5, (mw-10)*10, esc(message))
	fmt.Fprintf(&b, `<text x="%d" y="140" transform="scale(.1)" fill="#fff" textLength="%d">%s</text>`, (lw*2+mw)*5, (mw-10)*10, esc(message))
	b.WriteString(`</g></svg>`)
	return b.String()
}

// textWidth approximates Verdana 11px width plus 10px padding.
func textWidth(s string) int {
	w := 0.0
	for _, r := range s {
		switch {
		case r == ' ':
			w += 3.5
		case r == '.' || r == ',' || r == ':' || r == '\'' || r == '|' || r == 'i' || r == 'l' || r == 'j' || r == 'I':
			w += 3.2
		case r == '%':
			w += 10.0
		case r >= '0' && r <= '9':
			w += 7.0
		case r >= 'A' && r <= 'Z':
			w += 7.8
		case r == 'm' || r == 'w' || r == 'M' || r == 'W':
			w += 9.5
		case r > 127:
			w += 9.0
		default:
			w += 6.4
		}
	}
	return int(w+0.5) + 10
}

// ShieldsEndpoint is the JSON shape consumed by https://img.shields.io/endpoint.
type ShieldsEndpoint struct {
	SchemaVersion int    `json:"schemaVersion"`
	Label         string `json:"label"`
	Message       string `json:"message"`
	Color         string `json:"color"`
	Style         string `json:"style,omitempty"`
	NamedLogo     string `json:"namedLogo,omitempty"`
}

// ShieldsJSON renders the endpoint document.
func ShieldsJSON(label, message, color, style string) ([]byte, error) {
	e := ShieldsEndpoint{SchemaVersion: 1, Label: label, Message: message, Color: ResolveColor(color, 0)}
	if style != "" && style != "flat" {
		e.Style = style
	}
	return json.MarshalIndent(e, "", "  ")
}
