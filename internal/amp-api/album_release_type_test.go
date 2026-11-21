package ampapi

import (
	"slices"
	"testing"
)

func TestAlbumReleaseTypeFallback(t *testing.T) {
	const minute = 60 * 1000
	for _, tt := range []struct {
		name       string
		title      string
		durations  []int
		videos     int
		infer      bool
		incomplete bool
		hasNext    bool
		extraTrack bool
		isSingle   bool
		want       []string
	}{
		{name: "disabled by default", durations: []int{1, 1, 1, 1, 1, 1, 1}},
		{name: "seven songs rule out both", durations: []int{1, 1, 1, 1, 1, 1, 1}, infer: true, want: []string{"Album"}},
		{name: "video omitted from song count", durations: []int{1, 1, 1, 1, 1, 1}, videos: 1, infer: true},
		{name: "video-only release", videos: 7, infer: true},
		{name: "single still possible", durations: []int{9 * minute, 9 * minute, 9 * minute}, infer: true},
		{name: "short release at the EP limit", durations: []int{10 * minute, 10 * minute, 10 * minute}, infer: true},
		{name: "four songs exceeding the EP limit", durations: []int{9 * minute, 8 * minute, 7 * minute, 7 * minute}, infer: true, want: []string{"Album"}},
		{name: "two long songs exceed EP limit", durations: []int{16 * minute, 15 * minute}, infer: true, want: []string{"Album"}},
		{name: "missing duration", durations: []int{20 * minute, 15 * minute, 0}, infer: true},
		{name: "incomplete delivery", durations: []int{1, 1, 1, 1, 1, 1, 1}, infer: true, incomplete: true},
		{name: "pagination not finished", durations: []int{1, 1, 1, 1, 1, 1, 1}, infer: true, hasNext: true},
		{name: "catalog count mismatch", durations: []int{1, 1, 1, 1, 1, 1, 1}, infer: true, extraTrack: true},
		{name: "explicit single flag blocks fallback", durations: []int{31 * minute}, infer: true, isSingle: true},
		{name: "explicit EP tag takes precedence", title: "Reissue - EP", durations: []int{1, 1, 1, 1, 1, 1, 1}, infer: true, want: []string{"EP"}},
		{name: "confirmed secondary type with fallback", title: "Set (Live)", durations: []int{1, 1, 1, 1, 1, 1, 1}, infer: true, want: []string{"Album", "Live"}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			album := AlbumRespData{Type: "albums"}
			album.Attributes.Name = tt.title
			album.Attributes.IsComplete = !tt.incomplete
			album.Attributes.IsSingle = tt.isSingle
			album.Attributes.TrackCount = len(tt.durations) + tt.videos
			if tt.extraTrack {
				album.Attributes.TrackCount++
			}
			if tt.hasNext {
				album.Relationships.Tracks.Next = "/next"
			}
			for _, duration := range tt.durations {
				track := TrackRespData{Type: "songs"}
				track.Attributes.DurationInMillis = duration
				album.Relationships.Tracks.Data = append(album.Relationships.Tracks.Data, track)
			}
			for range tt.videos {
				track := TrackRespData{Type: "music-videos"}
				track.Attributes.DurationInMillis = 45 * minute
				album.Relationships.Tracks.Data = append(album.Relationships.Tracks.Data, track)
			}
			if got := album.ReleaseTypes(tt.infer); !slices.Equal(got, tt.want) {
				t.Fatalf("release types = %q, want %q", got, tt.want)
			}
		})
	}
}
