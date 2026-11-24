package lyrics

import (
	"encoding/xml"
	"fmt"
	"io"
	"regexp"
	"strings"
)

const ttmlDeclaration = `<?xml version="1.0" encoding="UTF-8"?>`

var ttmlRootAttribute = regexp.MustCompile(`[^\s=<>]+\s*=\s*(?:"[^"]*"|'[^']*')`)

// formatTTML indents structural elements using byte offsets from the XML
// decoder. Lyric lines and other leaf elements are copied as complete
// subtrees so whitespace between timed spans and XML entities survive.
func formatTTML(source string) (string, error) {
	source = strings.TrimPrefix(source, "\uFEFF")
	decoder := xml.NewDecoder(strings.NewReader(source))
	var output strings.Builder
	output.Grow(len(source))
	output.WriteString(ttmlDeclaration + "\n")
	writeLine := func(depth int, raw string) {
		output.WriteString(strings.Repeat("  ", depth))
		output.WriteString(raw)
		output.WriteByte('\n')
	}

	depth := 0
	for {
		start := decoder.InputOffset()
		token, err := decoder.Token()
		if err == io.EOF {
			return output.String(), nil
		}
		if err != nil {
			return "", err
		}
		raw := source[start:decoder.InputOffset()]
		switch element := token.(type) {
		case xml.StartElement:
			if depth == 0 && element.Name.Local == "tt" {
				raw = formatTTMLRoot(raw)
			}
			switch element.Name.Local {
			case "tt", "head", "metadata", "iTunesMetadata", "body", "div",
				"translations", "translation", "transliterations", "transliteration":
				writeLine(depth, raw)
				depth++
			default:
				if err := decoder.Skip(); err != nil {
					return "", err
				}
				writeLine(depth, source[start:decoder.InputOffset()])
			}
		case xml.EndElement:
			depth--
			if raw != "" {
				writeLine(depth, raw)
			}
		case xml.CharData:
			if strings.Trim(string(element), " \t\r\n") != "" {
				return "", fmt.Errorf("unexpected text outside a lyric line")
			}
		case xml.ProcInst:
			if element.Target != "xml" {
				writeLine(depth, raw)
			}
		default:
			writeLine(depth, raw)
		}
	}
}

// Match complete attributes so whitespace inside quoted values stays intact.
func formatTTMLRoot(raw string) string {
	attrs := ttmlRootAttribute.FindAllStringIndex(raw, -1)
	if len(attrs) < 2 {
		return raw
	}
	var root strings.Builder
	root.WriteString(raw[:attrs[0][1]])
	for _, attr := range attrs[1:] {
		root.WriteString("\n    ")
		root.WriteString(raw[attr[0]:attr[1]])
	}
	root.WriteString(raw[attrs[len(attrs)-1][1]:])
	return root.String()
}
