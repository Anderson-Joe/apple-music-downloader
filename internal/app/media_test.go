package app

import (
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
		inferAlbum   bool
		albumSongs   int
		wantTypes    []string
		wantBarcode  string
		wantCompiled bool
	}{
		{name: "compilation album", source: "albums", compilation: true, upc: "0196871644518", wantBarcode: "0196871644518", wantCompiled: true, wantTypes: []string{"Compilation"}},
		{name: "regular album", source: "albums", upc: "0196871486996", wantBarcode: "0196871486996"},
		{name: "album fallback enabled", source: "albums", albumSongs: 7, inferAlbum: true, wantTypes: []string{"Album"}},
		{name: "album fallback disabled", source: "albums", albumSongs: 7},
		{name: "playlist grouping skips album fallback", source: "playlists", albumSongs: 7, inferAlbum: true},
		{name: "playlist using original album allows fallback", source: "playlists", useSongInfo: true, albumSongs: 7, inferAlbum: true, wantTypes: []string{"Album"}},
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
			cfg.Metadata.Tags.InferAlbumReleaseType = tt.inferAlbum
			track := &model.Track{SavePath: path, PreType: tt.source}
			track.Resp.Attributes.Name = tt.songName
			track.AlbumData.Attributes.Upc = tt.upc
			track.AlbumData.Attributes.IsCompilation = tt.compilation
			track.AlbumData.Attributes.Name = tt.albumName
			track.AlbumData.Attributes.IsSingle = tt.isSingle
			if tt.albumSongs > 0 {
				track.AlbumData.Type = "albums"
				track.AlbumData.Attributes.IsComplete = true
				track.AlbumData.Attributes.TrackCount = tt.albumSongs
				for range tt.albumSongs {
					track.AlbumData.Relationships.Tracks.Data = append(track.AlbumData.Relationships.Tracks.Data, ampapi.TrackRespData{Type: "songs"})
				}
			}
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
