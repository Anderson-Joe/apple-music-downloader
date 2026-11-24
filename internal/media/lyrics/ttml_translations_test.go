package lyrics

import (
	"strings"
	"testing"
)

func TestAddTTMLTranslations(t *testing.T) {
	makeTTML := func(locale, key string) string {
		return `<tt xmlns="http://www.w3.org/ns/ttml" xmlns:itunes="http://music.apple.com/lyric-ttml-internal"><head><metadata><iTunesMetadata xmlns="http://music.apple.com/lyric-ttml-internal"><translations><translation type="subtitle" xml:lang="` +
			locale + `"><text for="` + key + `">translated</text></translation></translations></iTunesMetadata></metadata></head><body><div><p itunes:key="L1" begin="1s" end="3s"><span>one</span> <span>day</span></p></div></body></tt>`
	}
	source := makeTTML("en-US", "L1")
	var requests []string
	got := addTTMLTranslations(source, "en-US", []string{"EN-us", "zh-Hans-CN", "ja-JP", "fr-FR"}, func(locale string) (string, error) {
		requests = append(requests, locale)
		switch locale {
		case "zh-Hans-CN":
			return makeTTML("zh-Hans", "L1"), nil
		case "ja-JP":
			return makeTTML("ja-JP", "L99"), nil
		default:
			return makeTTML("zh-Hans", "L1"), nil
		}
	})
	if strings.Join(requests, ",") != "zh-Hans-CN,ja-JP,fr-FR" {
		t.Fatalf("requests = %v", requests)
	}
	doc, err := parseTTML(got)
	if err != nil {
		t.Fatal(err)
	}
	tracks := doc.Root().FindElements("./head/metadata/iTunesMetadata/translations/translation")
	if len(tracks) != 2 || tracks[1].SelectAttrValue("xml:lang", "") != "zh-Hans" {
		t.Fatalf("expected original and actual Chinese track, got %s", got)
	}
	if !strings.Contains(got, "<span>one</span> <span>day</span>") {
		t.Fatal("lyric spacing changed")
	}
}
