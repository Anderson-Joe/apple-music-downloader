package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"amdl/internal/config"
	defrag "amdl/internal/media/defrag"
	"amdl/internal/model"

	"github.com/itouakirai/go-mp4tag"
)

func TestWriteMP4TagsPreservesNativeGenres(t *testing.T) {
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
