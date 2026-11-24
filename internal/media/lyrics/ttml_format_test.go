package lyrics

import (
	"strings"
	"testing"
)

func TestFormatTTMLPreservesLyricLines(t *testing.T) {
	text := `<text for="L1"><span begin="1s">one</span> <span begin="2s">day &amp; night</span></text>`
	line := `<p itunes:key="L1"><span begin="1s">one</span> <span begin="2s">day</span></p>`
	raw := `<tt xmlns:itunes="urn:itunes" xml:lang="en"><head><metadata><transliterations><transliteration>` +
		text + `</transliteration></transliterations></metadata></head><body><div>` + line +
		`</div></body></tt>`
	got, err := formatTTML(raw)
	if err != nil {
		t.Fatal(err)
	}
	wantRoot := ttmlDeclaration + "\n<tt xmlns:itunes=\"urn:itunes\"\n    xml:lang=\"en\">\n"
	if !strings.HasPrefix(got, wantRoot) {
		t.Errorf("formatted TTML is missing header %q", wantRoot)
	}
	for _, want := range []string{"\n          " + text + "\n", "\n      " + line + "\n"} {
		if !strings.Contains(got, want) {
			t.Errorf("formatted TTML is missing line %q", want)
		}
	}
	again, err := formatTTML(got)
	if err != nil || again != got {
		t.Fatalf("formatting twice changed the output: %v", err)
	}
}
