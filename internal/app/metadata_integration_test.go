package app

import (
	"encoding/json"
	"slices"
	"testing"

	ampapi "amdl/internal/amp-api"
	"amdl/internal/config"
	"amdl/internal/model"
	"amdl/internal/wrapper"

	"github.com/itouakirai/go-mp4tag"
)

func TestMetadataIntegration(t *testing.T) {
	const song = `{
		"id":"1717034230",
		"href":"/v1/catalog/tw/songs/1717034230",
		"attributes":{"name":"Integration song","albumName":"Series [Original Television Soundtrack] - EP","artistName":"Singer A & Singer B",
			"composerName":"Composer A & Composer B","genreNames":["流行樂","原聲帶"],"contentRating":"explicit",
			"trackNumber":2,"discNumber":1,"isrc":"HKA972302100","releaseDate":"2023-09-24"},
		"relationships":{
			"artists":{"data":[{"id":"425208570","attributes":{"name":"Singer A"}},{"id":"425208571","attributes":{"name":"Singer B"}}]},
			"genres":{"data":[{"id":"16","attributes":{"name":"原聲帶"}},{"id":"14","attributes":{"name":"流行樂"}}]}
		}
	}`
	const album = `{
		"id":"1717034215","type":"albums",
		"attributes":{"name":"Series [Original Television Soundtrack] - EP","artistName":"Album Artist","trackCount":6,
			"upc":"0196871644518","releaseDate":"2023-11-26","isCompilation":true},
		"relationships":{"artists":{"data":[{"id":"3003","attributes":{"name":"Album Artist A"}},{"id":"3001","attributes":{"name":"Album Artist B"}}]}}
	}`
	const songCredits = `{
		"composers":{"data":[{"id":"4294967297","attributes":{"name":"Composer A"}},{"id":"2001","attributes":{"name":"Composer B"}}]},
		"credits":{"data":[
			{"attributes":{"kind":"performer"},"relationships":{"credit-artists":{"data":[
				{"attributes":{"name":"Singer A","roleNames":["Vocals"]}},
				{"attributes":{"name":"Singer B","roleNames":["Guitar"]}}
			]}}},
			{"attributes":{"kind":"composer-and-lyrics"},"relationships":{"credit-artists":{"data":[
				{"attributes":{"name":"Writer A","roleNames":["Lyrics","Arranger"]}},
				{"attributes":{"name":"Writer B","roleNames":["Lyrics","Arranger"]}}
			]}}},
			{"attributes":{"kind":"production-and-engineering"},"relationships":{"credit-artists":{"data":[
				{"attributes":{"name":"Producer A","roleNames":["Producer","Mastering Engineer"]}},
				{"attributes":{"name":"Producer B","roleNames":["Producer","Mixing Engineer"]}}
			]}}}
		]}
	}`

	path := taggableTestM4A(t)
	previous, err := mp4tag.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	existingArtistTags := map[string][]string{
		"ARTISTS":          {"Old artist A", "Old artist B"},
		"ALBUMARTISTS":     {"Old album artist A", "Old album artist B"},
		"ARTISTSSORT":      {"Old artist A", "Old artist B"},
		"ALBUMARTISTSSORT": {"Old album artist A", "Old album artist B"},
	}
	err = previous.Write(&mp4tag.MP4Tags{ReplaceCustomValues: existingArtistTags}, nil)
	previous.Close()
	if err != nil {
		t.Fatal(err)
	}
	track := model.Track{
		ID: "1717034230", Storefront: "us", SavePath: path,
		PreType: "albums", PreID: "1717034215", DiscTotal: 1,
		Credits: &ampapi.SongCredits{},
		Playback: wrapper.WebplaybackData{
			XID: "Vendor:isrc:TEST", Comments: "Playback comment",
			Gapless: json.RawMessage(`false`),
		},
	}
	for _, fixture := range []struct {
		data string
		dest any
	}{{song, &track.Resp}, {album, &track.AlbumData}, {songCredits, track.Credits}} {
		if err := json.Unmarshal([]byte(fixture.data), fixture.dest); err != nil {
			t.Fatal(err)
		}
	}

	cfg := config.ConfigSet{}
	cfg.Metadata.Tags.SortOrder = true
	cfg.Metadata.Tags.ItunesID = true
	cfg.Metadata.Tags.Credits = true
	cfg.Metadata.Tags.PlaybackTags = true
	runner := NewRunner(cfg)
	for range 2 {
		if err := runner.writeMP4Tags(&track, "[00:01.00] Test lyrics"); err != nil {
			t.Fatal(err)
		}
	}
	file, err := mp4tag.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	tags, err := file.Read()
	if err != nil {
		t.Fatal(err)
	}

	for _, field := range []struct {
		name      string
		got, want []string
	}{
		{"ARTIST", tags.Artists, []string{"Singer A", "Singer B"}},
		{"ARTISTSORT", tags.ArtistSorts, []string{"Singer A", "Singer B"}},
		{"ALBUMARTIST", tags.AlbumArtists, []string{"Album Artist A", "Album Artist B"}},
		{"ALBUMARTISTSORT", tags.AlbumArtistSorts, []string{"Album Artist A", "Album Artist B"}},
		{"COMPOSER", tags.Composers, []string{"Composer A", "Composer B"}},
		{"COMPOSERSORT", tags.ComposerSorts, []string{"Composer A", "Composer B"}},
		{"GENRE", tags.CustomGenres, []string{"流行樂", "原聲帶"}},
		{"ARRANGER", tags.Arrangers, []string{"Writer A", "Writer B"}},
		{"PRODUCER", tags.Producers, []string{"Producer A", "Producer B"}},
		{"ENGINEER", tags.Engineers, []string{"Producer A"}},
		{"PERFORMER", tags.Performers, []string{"Singer A (Vocals)", "Singer B (Guitar)"}},
	} {
		if !slices.Equal(field.got, field.want) {
			t.Errorf("%s = %q, want %q", field.name, field.got, field.want)
		}
	}
	wantCustomTags := map[string][]string{
		"ITUNESALBUMARTISTID": {"3003", "3001"},
		"RELEASETYPE":         {"EP", "Compilation", "Soundtrack"},
		"UPC":                 {"0196871644518"}, "BARCODE": {"0196871644518"},
		"ISRC":     {"HKA972302100"},
		"LYRICIST": {"Writer A", "Writer B"}, "LYRICISTSORT": {"Writer A", "Writer B"},
		"MIXER": {"Producer B"},
	}
	if !slices.Equal(tags.ItunesArtistIDs, []uint64{425208570, 425208571}) ||
		!slices.Equal(tags.ItunesComposerIDs, []uint64{4294967297, 2001}) {
		t.Errorf("artist IDs = %v, composer IDs = %v", tags.ItunesArtistIDs, tags.ItunesComposerIDs)
	}
	if tags.XID != "Vendor:isrc:TEST" || tags.Comment != "Playback comment" ||
		tags.ItunesGapless == nil || *tags.ItunesGapless || !slices.Equal(tags.ItunesGenreIDs, []uint64{14, 16}) {
		t.Errorf("playback metadata changed: XID %q, comment %q, gapless %v, genre ID %d",
			tags.XID, tags.Comment, tags.ItunesGapless, tags.ItunesGenreID)
	}
	for field, values := range existingArtistTags {
		wantCustomTags[field] = values
	}
	for field, want := range wantCustomTags {
		var got []string
		if first, ok := tags.Custom[field]; ok {
			got = append([]string{first}, tags.OtherCustom[field]...)
		}
		if !slices.Equal(got, want) {
			t.Errorf("%s = %q, want %q", field, got, want)
		}
	}
	if tags.Title != "Integration song" || tags.Album != "Series [Original Television Soundtrack] - EP" ||
		tags.Date != "2023-11-26" || tags.ReleaseDate != "2023-09-24" || tags.TrackNumber != 2 || tags.TrackTotal != 6 ||
		tags.DiscNumber != 1 || tags.DiscTotal != 1 || tags.Lyrics != "[00:01.00] Test lyrics" {
		t.Error("track title, album, date, numbering, or lyrics changed")
	}
	if tags.Compilation == nil || !*tags.Compilation || tags.ItunesAdvisory != mp4tag.ItunesAdvisoryExplicit ||
		tags.ItunesMediaType == nil || *tags.ItunesMediaType != mp4tag.ItunesMediaTypeMusic ||
		tags.ItunesAlbumID != 1717034215 || tags.ItunesArtistID != 425208570 ||
		tags.ItunesCatalogID != 1717034230 || tags.ItunesStorefrontID != 143470 {
		t.Error("native compilation, rating, media type, or iTunes IDs changed")
	}
}
