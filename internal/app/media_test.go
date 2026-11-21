package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"amdl/internal/config"
	defrag "amdl/internal/media/defrag"
	"amdl/internal/model"

	"github.com/itouakirai/go-mp4tag"
)

func TestWriteMP4TagsItunesMetadata(t *testing.T) {
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
		name     string
		id       string
		writeID  bool
		wantID   uint32
		rating   string
		advisory mp4tag.ItunesAdvisory
	}{
		{name: "catalog track", id: "1717034230", writeID: true, wantID: 1717034230, rating: "explicit", advisory: mp4tag.ItunesAdvisoryExplicit},
		{name: "unsigned ID", id: "4294967295", writeID: true, wantID: 4294967295, rating: "clean", advisory: mp4tag.ItunesAdvisoryClean},
		{name: "out of range", id: "4294967296", writeID: true},
		{name: "noncatalog ID", id: "library-track", writeID: true},
		{name: "disabled", id: "1717034230"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "track.m4a")
			if err := os.WriteFile(path, audio, 0o600); err != nil {
				t.Fatal(err)
			}
			if err := defrag.DefragmentMP4(path); err != nil {
				t.Fatal(err)
			}
			cfg := config.ConfigSet{}
			cfg.Metadata.Tags.ItunesID = tt.writeID
			track := &model.Track{ID: tt.id, SavePath: path}
			track.Storefront = "cn"
			track.Resp.Href = "/v1/catalog/us/songs/1717034230"
			track.Resp.Attributes.ContentRating = tt.rating
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
			if tags.ItunesCatalogID != tt.wantID {
				t.Errorf("catalog ID = %d, want %d", tags.ItunesCatalogID, tt.wantID)
			}
			var wantStorefrontID uint32
			if tt.writeID {
				wantStorefrontID = 143441
			}
			if tags.ItunesStorefrontID != wantStorefrontID {
				t.Errorf("storefront ID = %d, want %d", tags.ItunesStorefrontID, wantStorefrontID)
			}
			if tags.ItunesMediaType == nil || *tags.ItunesMediaType != mp4tag.ItunesMediaTypeMusic {
				t.Error("expected native iTunes music media type")
			}
			if tags.ItunesAdvisory != tt.advisory {
				t.Errorf("advisory = %v, want %v", tags.ItunesAdvisory, tt.advisory)
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
