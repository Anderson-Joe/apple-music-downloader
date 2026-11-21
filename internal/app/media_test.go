package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	ampapi "amdl/internal/amp-api"
	"amdl/internal/config"
	defrag "amdl/internal/media/defrag"
	"amdl/internal/model"

	"github.com/itouakirai/go-mp4tag"
)

func taggableTestM4A(t *testing.T) string {
	t.Helper()
	const fixture = "../media/mv/testdata/A48"
	init, err := os.ReadFile(filepath.Join(fixture, "init.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	segment, err := os.ReadFile(filepath.Join(fixture, "1.m4s"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "genres.m4a")
	if err := os.WriteFile(path, append(init, segment...), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := defrag.DefragmentMP4(path); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestWriteMP4TagsPreservesNativeGenres(t *testing.T) {
	path := taggableTestM4A(t)
	runner := NewRunner(config.ConfigSet{})
	track := &model.Track{SavePath: path}
	track.Resp.Attributes.Isrc = "USAAA2600001"
	for _, tt := range []struct {
		genres        []string
		relationships string
		ids           []string
	}{
		{genres: []string{"Pop"}, relationships: `[{"id":"14","attributes":{"name":"Pop"}}]`, ids: []string{"14"}},
		{
			genres:        []string{"華語流行樂", "音樂", "流行樂"},
			relationships: `[{"id":"14","attributes":{"name":"流行樂"}},{"id":"1253","attributes":{"name":"華語流行樂"}},{"id":"34","attributes":{"name":"音樂"}}]`,
			ids:           []string{"1253", "34", "14"},
		},
		{
			genres:        []string{"Mandopop", "Music", "Pop"},
			relationships: `[{"id":"34","attributes":{"name":"Music"}},{"id":"14","attributes":{"name":"Pop"}}]`,
		},
		{
			genres:        []string{"Pop", "Music"},
			relationships: `[{"id":"14","attributes":{"name":"Pop"}},{"id":"34","attributes":{"name":"Music"}}]`,
			ids:           []string{"14", "34"},
		},
		{
			genres:        []string{"Pop"},
			relationships: `[{"id":"14","attributes":{"name":"Pop"}},{"id":"35","attributes":{"name":"Pop"}}]`,
		},
	} {
		track.Resp.Attributes.GenreNames = tt.genres
		if err := json.Unmarshal([]byte(tt.relationships), &track.Resp.Relationships.Genres.Data); err != nil {
			t.Fatal(err)
		}
		if err := runner.writeMP4Tags(track, ""); err != nil {
			t.Fatal(err)
		}
		file, err := mp4tag.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		tags, err := file.Read()
		file.Close()
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(tags.CustomGenres, tt.genres) {
			t.Fatalf("native genres = %q, want %q", tags.CustomGenres, tt.genres)
		}
		var ids []string
		if first, ok := tags.Custom["GENREID"]; ok {
			ids = append([]string{first}, tags.OtherCustom["GENREID"]...)
		}
		if !slices.Equal(ids, tt.ids) {
			t.Fatalf("genre IDs = %q, want %q for %q", ids, tt.ids, tt.genres)
		}
		if _, ok := tags.Custom["GENRE"]; ok {
			t.Fatal("unexpected freeform GENRE tag")
		}
		if tags.Custom["ISRC"] != track.Resp.Attributes.Isrc {
			t.Fatalf("ISRC = %q, want %q", tags.Custom["ISRC"], track.Resp.Attributes.Isrc)
		}
	}
}

func TestWriteMP4TagsSongCredits(t *testing.T) {
	var credits ampapi.SongCredits
	if err := json.Unmarshal([]byte(`{
		"composers":{"data":[{"attributes":{"name":"Composer A"}},{"attributes":{"name":"Composer B"}}]},
		"credits":{"data":[
			{"attributes":{"kind":"performer"},"relationships":{"credit-artists":{"data":[
				{"attributes":{"name":"Singer","roleNames":["演唱","Vocals"]}}
			]}}},
			{"attributes":{"kind":"composer-and-lyrics"},"relationships":{"credit-artists":{"data":[
				{"attributes":{"name":"Lyricist A","roleNames":["作詞","Lyrics","Arranger"]}},
				{"attributes":{"name":"Lyricist B","roleNames":["Lyrics","編曲"]}},
				{"attributes":{"name":"Unspecified","roleNames":["Songwriter","Adapter"]}}
			]}}},
			{"attributes":{"kind":"production-and-engineering"},"relationships":{"credit-artists":{"data":[
				{"attributes":{"name":"Producer A","roleNames":["Producer","Mastering Engineer"]}},
				{"attributes":{"name":"Producer B","roleNames":["製作人","混音師"]}}
			]}}}
		]}
	}`), &credits); err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct {
		name    string
		credits *ampapi.SongCredits
		sort    bool
	}{
		{name: "credits and sort tags", credits: &credits, sort: true},
		{name: "credits without sort tags", credits: &credits},
		{name: "missing credits keep catalog composer", sort: true},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := taggableTestM4A(t)
			cfg := config.ConfigSet{}
			cfg.Metadata.Tags.SortOrder = tt.sort
			track := &model.Track{SavePath: path, Credits: tt.credits}
			track.Resp.Attributes.ComposerName = "Catalog composers"
			track.Resp.Attributes.ArtistName = "Display artist"
			runner := NewRunner(cfg)
			for range 2 {
				if err := runner.writeMP4Tags(track, ""); err != nil {
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
			wantComposers := []string{"Catalog composers"}
			wantCustom := map[string][]string{"PERFORMER": {"Display artist"}}
			if tt.credits != nil {
				wantComposers = []string{"Composer A", "Composer B"}
				wantCustom = map[string][]string{
					"LYRICIST":  {"Lyricist A", "Lyricist B"},
					"ARRANGER":  {"Lyricist A", "Lyricist B"},
					"PRODUCER":  {"Producer A", "Producer B"},
					"ENGINEER":  {"Producer A"},
					"MIXER":     {"Producer B"},
					"PERFORMER": {"Singer (演唱)", "Singer (Vocals)"},
				}
				if tt.sort {
					wantCustom["LYRICISTSORT"] = wantCustom["LYRICIST"]
				}
			}
			if !slices.Equal(tags.Composers, wantComposers) {
				t.Errorf("COMPOSER = %q, want %q", tags.Composers, wantComposers)
			}
			var wantComposerSort []string
			if tt.sort {
				wantComposerSort = wantComposers
			}
			if !slices.Equal(tags.ComposerSorts, wantComposerSort) {
				t.Errorf("COMPOSERSORT = %q, want %q", tags.ComposerSorts, wantComposerSort)
			}
			for _, name := range []string{"LYRICIST", "ARRANGER", "PRODUCER", "ENGINEER", "MIXER", "PERFORMER", "LYRICISTSORT", "SONGWRITER"} {
				var names []string
				if first, ok := tags.Custom[name]; ok {
					names = append([]string{first}, tags.OtherCustom[name]...)
				}
				if !slices.Equal(names, wantCustom[name]) {
					t.Errorf("%s = %q, want %q", name, names, wantCustom[name])
				}
			}
		})
	}
}

func TestWriteMP4TagsReleaseMetadata(t *testing.T) {
	const fixture = "../media/mv/testdata/A48"
	init, err := os.ReadFile(filepath.Join(fixture, "init.mp4"))
	if err != nil {
		t.Fatal(err)
	}
	segment, err := os.ReadFile(filepath.Join(fixture, "1.m4s"))
	if err != nil {
		t.Fatal(err)
	}
	audio := append(init, segment...)

	for _, tt := range []struct {
		name         string
		source       string
		useSongInfo  bool
		compilation  bool
		upc          string
		albumName    string
		songName     string
		isSingle     bool
		wantTypes    []string
		wantBarcode  string
		wantCompiled bool
	}{
		{name: "compilation album", source: "albums", compilation: true, upc: "0196871644518", wantBarcode: "0196871644518", wantCompiled: true, wantTypes: []string{"Compilation"}},
		{name: "regular album", source: "albums", upc: "0196871486996", wantBarcode: "0196871486996"},
		{name: "single", source: "albums", albumName: "唯一 - Single", isSingle: true, upc: "196871486996", wantBarcode: "196871486996", wantTypes: []string{"Single"}},
		{name: "EP", source: "albums", albumName: "T.I.M.E. - EP", upc: "196871644518", wantBarcode: "196871644518", wantTypes: []string{"EP"}},
		{name: "unmarked one-track release", source: "albums", albumName: "Long Suite", isSingle: true},
		{name: "EP marker takes precedence for a one-track release", source: "albums", albumName: "Long Suite - EP", isSingle: true, wantTypes: []string{"EP"}},
		{name: "nonfinal text is not a type marker", source: "albums", albumName: "Studio - EP Sessions"},
		{name: "music inspired by a soundtrack", source: "albums", albumName: "Songs (Inspired by the Original Motion Picture Soundtrack)"},
		{name: "soundtrack score", source: "albums", albumName: "Barbie (Score from the Original Motion Picture Soundtrack)", wantTypes: []string{"Soundtrack"}},
		{name: "compiled TV soundtrack", source: "albums", compilation: true, albumName: "Queen of Tears (Original Television Soundtrack) Special", wantCompiled: true, wantTypes: []string{"Compilation", "Soundtrack"}},
		{name: "soundtrack EP", source: "albums", albumName: "Series [Original Television Soundtrack] - EP", wantTypes: []string{"EP", "Soundtrack"}},
		{name: "live acoustic release", source: "albums", albumName: "MTV Unplugged In New York (Live Acoustic)", wantTypes: []string{"Live"}},
		{name: "live EP", source: "albums", albumName: "Sessions (Live at the Theatre) - EP", wantTypes: []string{"EP", "Live"}},
		{name: "remixes EP", source: "albums", albumName: "Out of This World (Remixes) - EP", wantTypes: []string{"EP", "Remix"}},
		{name: "named remix", source: "albums", albumName: "siren (ABSOLUTE. Remix) - Single", wantTypes: []string{"Single", "Remix"}},
		{name: "deduplicate independent versions", source: "albums", albumName: "Sessions (Live) [Live at the Theatre] [Remix] - EP", wantTypes: []string{"EP", "Live", "Remix"}},
		{name: "live track does not classify its album", source: "albums", albumName: "Studio Album", songName: "Bonus Track (Live)"},
		{name: "remix track does not classify its album", source: "albums", albumName: "Studio Album", songName: "Bonus Track (Remix)"},
		{name: "remaster is not a remix", source: "albums", albumName: "Studio Album (2022 Remaster)"},
		{name: "missing UPC", source: "albums", compilation: true, wantCompiled: true, wantTypes: []string{"Compilation"}},
		{name: "playlist using album", source: "playlists", useSongInfo: true, compilation: true, upc: "0196871644518", albumName: "Release - EP", wantBarcode: "0196871644518", wantCompiled: true, wantTypes: []string{"EP", "Compilation"}},
		{name: "playlist as album", source: "playlists", compilation: true, upc: "0196871644518", albumName: "Release - EP"},
		{name: "station using album", source: "stations", useSongInfo: true, compilation: true, upc: "0196871644518", albumName: "Release - Single", wantBarcode: "0196871644518", wantCompiled: true, wantTypes: []string{"Single", "Compilation"}},
		{name: "station as album", source: "stations", compilation: true, upc: "0196871644518", albumName: "Release - Single"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "release.m4a")
			if err := os.WriteFile(path, audio, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := defrag.DefragmentMP4(path); err != nil {
				t.Fatal(err)
			}
			cfg := config.ConfigSet{}
			cfg.Metadata.Tags.UseSongInfoForPlaylist = tt.useSongInfo
			track := &model.Track{SavePath: path, PreType: tt.source}
			track.Resp.Attributes.Name = tt.songName
			track.AlbumData.Attributes.Upc = tt.upc
			track.AlbumData.Attributes.IsCompilation = tt.compilation
			track.AlbumData.Attributes.Name = tt.albumName
			track.AlbumData.Attributes.IsSingle = tt.isSingle
			if err := NewRunner(cfg).writeMP4Tags(track, ""); err != nil {
				t.Fatal(err)
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
			if barcode, upc := tags.Custom["BARCODE"], tags.Custom["UPC"]; barcode != tt.wantBarcode || upc != tt.wantBarcode {
				t.Fatalf("BARCODE = %q, UPC = %q, want %q for both", barcode, upc, tt.wantBarcode)
			}
			var releaseTypes []string
			if firstType, present := tags.Custom["RELEASETYPE"]; present {
				releaseTypes = append([]string{firstType}, tags.OtherCustom["RELEASETYPE"]...)
			}
			if !slices.Equal(releaseTypes, tt.wantTypes) {
				t.Errorf("RELEASETYPE = %q, want %q", releaseTypes, tt.wantTypes)
			}
			if tt.wantCompiled {
				if tags.Compilation == nil || !*tags.Compilation {
					t.Fatal("expected an enabled native compilation tag")
				}
			} else if tags.Compilation != nil {
				t.Fatal("expected the compilation tag to be absent")
			}
			if _, ok := tags.Custom["COMPILATION"]; ok {
				t.Fatal("unexpected freeform compilation tag")
			}
		})
	}
}

func TestStreamAllowedCPC(t *testing.T) {
	const master = `#EXTM3U
#EXT-X-STREAM-INF:BANDWIDTH=1000,RESOLUTION=1920x1080,ALLOWED-CPC="com.microsoft.playready:SL2000,urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed:WIDEVINE_HARDWARE"
video_1080.m3u8
#EXT-X-STREAM-INF:BANDWIDTH=2000,RESOLUTION=3840x2160,ALLOWED-CPC="com.apple.streamingkeydelivery:Main/AppleMain,com.microsoft.playready:SL3000"
video_2160.m3u8
`

	if got := streamAllowedCPC(master, "video_1080.m3u8"); got != "com.microsoft.playready:SL2000,urn:uuid:edef8ba9-79d6-4ace-a3c8-27dcd51d21ed:WIDEVINE_HARDWARE" {
		t.Fatalf("unexpected 1080p ALLOWED-CPC: %q", got)
	}
	if got := streamAllowedCPC(master, "video_2160.m3u8"); got != "com.apple.streamingkeydelivery:Main/AppleMain,com.microsoft.playready:SL3000" {
		t.Fatalf("unexpected 2160p ALLOWED-CPC: %q", got)
	}
	if got := streamAllowedCPC(master, "missing.m3u8"); got != "" {
		t.Fatalf("expected missing variant to have empty ALLOWED-CPC, got %q", got)
	}
}

func TestWriteM3UPlaylist(t *testing.T) {
	tempDir := t.TempDir()
	r := NewRunner(config.ConfigSet{})
	r.Flags.SaveM3U8 = true

	tracks := []AddedTrack{
		{
			Path:   filepath.Join(tempDir, "01. Song A.m4a"),
			Artist: "Artist A",
			Song:   "Song A",
		},
		{
			Path:   filepath.Join(tempDir, "02. Song B.flac"),
			Artist: "Artist B",
			Song:   "Song B",
		},
	}

	err := r.writeM3UPlaylist(tempDir, "My Playlist", tracks)
	if err != nil {
		t.Fatalf("writeM3UPlaylist failed: %v", err)
	}

	m3uPath := filepath.Join(tempDir, "My Playlist.m3u8")
	data, err := os.ReadFile(m3uPath)
	if err != nil {
		t.Fatalf("failed to read created m3u8 file: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "#EXTM3U") {
		t.Fatalf("missing #EXTM3U in content: %s", content)
	}
	if !strings.Contains(content, "#EXTINF:-1,Artist A - Song A") || !strings.Contains(content, "01. Song A.m4a") {
		t.Fatalf("unexpected content for track 1: %s", content)
	}
	if !strings.Contains(content, "#EXTINF:-1,Artist B - Song B") || !strings.Contains(content, "02. Song B.flac") {
		t.Fatalf("unexpected content for track 2: %s", content)
	}
}

func TestSaveM3UPlaylist(t *testing.T) {
	tempDir := t.TempDir()
	r := NewRunner(config.ConfigSet{})

	// 1. When SaveM3U8 is false, no file should be created
	r.Flags.SaveM3U8 = false
	r.State.AddedTracks = []AddedTrack{
		{Path: filepath.Join(tempDir, "01. First.m4a"), Artist: "Artist 1", Song: "First"},
	}
	r.saveM3UPlaylist(tempDir, "Disabled", 0)
	if _, err := os.Stat(filepath.Join(tempDir, "Disabled.m3u8")); !os.IsNotExist(err) {
		t.Fatalf("expected m3u8 file to not exist when SaveM3U8=false")
	}

	// 2. When SaveM3U8 is true, only tracks from startIdx onwards should be saved
	r.Flags.SaveM3U8 = true
	startIdx := len(r.State.AddedTracks) // startIdx = 1

	r.State.AddedTracks = append(r.State.AddedTracks,
		AddedTrack{Path: filepath.Join(tempDir, "02. Second.m4a"), Artist: "Artist 2", Song: "Second"},
		AddedTrack{Path: filepath.Join(tempDir, "03. Third.m4a"), Artist: "Artist 3", Song: "Third"},
	)

	r.saveM3UPlaylist(tempDir, "Album Name", startIdx)
	m3uPath := filepath.Join(tempDir, "Album Name.m3u8")
	data, err := os.ReadFile(m3uPath)
	if err != nil {
		t.Fatalf("failed to read m3u8 file: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "01. First.m4a") {
		t.Fatalf("m3u8 should not contain track before startIdx, got: %s", content)
	}
	if !strings.Contains(content, "02. Second.m4a") || !strings.Contains(content, "03. Third.m4a") {
		t.Fatalf("m3u8 missing tracks after startIdx, got: %s", content)
	}
}
