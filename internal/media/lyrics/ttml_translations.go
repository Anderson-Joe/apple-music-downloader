package lyrics

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/beevik/etree"
)

type translationTrack struct {
	language string
	kind     string
}

func addTTMLTranslations(source, language string, locales []string, fetch func(string) (string, error)) string {
	doc, err := parseTTML(source)
	if err != nil || doc.Root() == nil {
		return source
	}
	root := doc.Root()
	lines := ttmlLineTimings(root)
	if len(lines) == 0 {
		return source
	}
	metadata := root.FindElement("./head/metadata/iTunesMetadata")
	tracks := make(map[translationTrack]bool)
	if metadata != nil {
		for _, element := range metadata.FindElements("./translations/translation") {
			tracks[ttmlTranslationTrack(element)] = true
		}
	}
	requested := map[string]bool{strings.ToLower(strings.TrimSpace(language)): true}
	if strings.TrimSpace(language) == "" {
		requested["en"] = true // wrapper's default lyrics locale
	}
	added := false
	for _, locale := range locales {
		locale = strings.TrimSpace(locale)
		key := strings.ToLower(locale)
		if locale == "" || requested[key] {
			continue
		}
		requested[key] = true
		response, err := fetch(locale)
		if err != nil {
			fmt.Printf("Could not fetch TTML translation for %s: %v\n", locale, err)
			continue
		}
		extra, err := parseTTML(response)
		if err != nil || extra.Root() == nil || !maps.Equal(lines, ttmlLineTimings(extra.Root())) {
			fmt.Printf("Could not merge TTML translation for %s: lyric line keys or timing differ\n", locale)
			continue
		}
		for _, element := range extra.Root().FindElements("./head/metadata/iTunesMetadata/translations/translation") {
			track := ttmlTranslationTrack(element)
			if track.language == "" || tracks[track] || !matchesTTMLLines(element, lines) {
				continue
			}
			if metadata == nil {
				parent := root
				for _, name := range []string{"head", "metadata", "iTunesMetadata"} {
					child := parent.SelectElement(name)
					if child == nil {
						child = etree.NewElement(name)
						if name == "head" {
							parent.InsertChildAt(0, child)
						} else {
							parent.AddChild(child)
						}
						if name == "iTunesMetadata" {
							child.CreateAttr("xmlns", element.Parent().Parent().NamespaceURI())
						}
					}
					parent = child
				}
				metadata = parent
			}
			group := metadata.SelectElement("translations")
			if group == nil {
				group = metadata.CreateElement("translations")
			}
			copy := copyTTMLTranslation(element, group)
			copy.CreateAttr("type", track.kind)
			group.AddChild(copy)
			tracks[track] = true
			added = true
		}
	}
	if !added {
		return source
	}
	result, err := doc.WriteToString()
	if err != nil {
		return source
	}
	return result
}

func ttmlLineTimings(root *etree.Element) map[string][2]string {
	lines := make(map[string][2]string)
	for _, line := range root.FindElements("./body//p") {
		if key := line.SelectAttrValue("itunes:key", ""); key != "" {
			lines[key] = [2]string{line.SelectAttrValue("begin", ""), line.SelectAttrValue("end", "")}
		}
	}
	return lines
}

func ttmlTranslationTrack(element *etree.Element) translationTrack {
	kind := element.SelectAttrValue("type", element.Parent().SelectAttrValue("type", ""))
	return translationTrack{
		language: strings.ToLower(strings.TrimSpace(element.SelectAttrValue("xml:lang", ""))),
		kind:     kind,
	}
}

func matchesTTMLLines(element *etree.Element, lines map[string][2]string) bool {
	texts := element.SelectElements("text")
	if len(texts) == 0 {
		return false
	}
	for _, line := range texts {
		if _, ok := lines[line.SelectAttrValue("for", "")]; !ok {
			return false
		}
	}
	return true
}

// Preserve inherited namespace bindings when copying a track to another TTML document.
func copyTTMLTranslation(element, target *etree.Element) *etree.Element {
	bindings := func(start *etree.Element) map[string]string {
		result := make(map[string]string)
		for parent := start; parent != nil; parent = parent.Parent() {
			for _, attr := range parent.Attr {
				if attr.Space == "xmlns" || attr.FullKey() == "xmlns" {
					key := attr.FullKey()
					if _, exists := result[key]; !exists {
						result[key] = attr.Value
					}
				}
			}
		}
		return result
	}
	dest := bindings(target)
	clone := element.Copy()
	source := bindings(element)
	for _, key := range slices.Sorted(maps.Keys(source)) {
		value := source[key]
		if dest[key] != value {
			clone.CreateAttr(key, value)
		}
	}
	return clone
}
