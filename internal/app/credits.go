package app

import (
	"slices"
	"strings"

	ampapi "amdl/internal/amp-api"

	"github.com/itouakirai/go-mp4tag"
)

func applySongCredits(tags *mp4tag.MP4Tags, credits *ampapi.SongCredits, sortOrder bool) {
	if credits == nil {
		return
	}

	var composers []string
	if credits.Composers.Next == "" {
		for _, composer := range credits.Composers.Data {
			name := strings.TrimSpace(composer.Attributes.Name)
			if name == "" {
				composers = nil
				break
			}
			composers = appendCreditValue(composers, name)
		}
	}

	values := make(map[string][]string)
	for _, category := range credits.Credits.Data {
		kind := category.Attributes.Kind
		for _, artist := range category.Relationships.Artists.Data {
			name := strings.TrimSpace(artist.Attributes.Name)
			if name == "" {
				continue
			}
			for _, label := range artist.Attributes.Roles {
				label = strings.TrimSpace(label)
				if kind == "performer" && label != "" {
					value := name + " (" + label + ")"
					values["PERFORMER"] = appendCreditValue(values["PERFORMER"], value)
				}
				if tag := creditTag(kind, strings.ToLower(label)); tag != "" {
					values[tag] = appendCreditValue(values[tag], name)
				}
			}
		}
	}

	// The composer relationship avoids inferring separate roles from a
	// composite songwriter credit. Keep the catalog display fallback as well.
	if len(composers) == 0 && tags.Composer == "" {
		composers = values["COMPOSER"]
	}
	delete(values, "COMPOSER")
	if len(composers) > 0 {
		tags.Composers = composers
		if sortOrder {
			tags.ComposerSorts = composers
		}
	}
	if sortOrder && len(values["LYRICIST"]) > 0 {
		values["LYRICISTSORT"] = values["LYRICIST"]
	}
	for tag, names := range values {
		tags.ReplaceCustomValues[tag] = names
	}
}

func appendCreditValue(values []string, value string) []string {
	if slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

// The web catalog localizes role labels. Map explicit roles rather than
// inferring a composer or lyricist from the generic Songwriter label.
func creditTag(kind, label string) string {
	if kind == "performer" {
		// Apple uses 混音師 for Remixer in performer credits. The same
		// label in production credits refers to a mixer.
		switch label {
		case "remixer", "混音師", "混音师":
			return "REMIXER"
		case "conductor", "指揮", "指挥":
			return "CONDUCTOR"
		case "dj mixer":
			return "DJMIXER"
		default:
			return ""
		}
	}
	switch label {
	case "composer", "作曲家", "作曲":
		return "COMPOSER"
	case "lyrics", "lyricist", "作詞", "作词", "填詞", "填词":
		return "LYRICIST"
	case "arranger", "編曲", "编曲":
		return "ARRANGER"
	case "producer", "製作人", "制作人":
		return "PRODUCER"
	case "mixing engineer", "mixer", "混音師", "混音师", "混音工程師", "混音工程师":
		return "MIXER"
	case "engineer", "recording engineer", "mastering engineer", "工程師", "工程师", "錄音工程師", "录音工程师", "母帶工程師", "母带工程师":
		return "ENGINEER"
	case "remixer":
		return "REMIXER"
	case "dj mixer":
		return "DJMIXER"
	default:
		return ""
	}
}
