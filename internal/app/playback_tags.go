package app

import (
	"strconv"
	"strings"

	"amdl/internal/wrapper"

	"github.com/itouakirai/go-mp4tag"
)

func (r *Runner) applyPlaybackTags(tags *mp4tag.MP4Tags, playback wrapper.WebplaybackData) {
	if !r.Config.Metadata.Tags.PlaybackTags {
		return
	}
	tags.XID = playback.XID
	tags.Comment = playback.Comments
	if gapless, err := strconv.ParseBool(strings.Trim(string(playback.Gapless), `"`)); err == nil {
		tags.ItunesGapless = &gapless
	}
	if r.Config.Metadata.Tags.ItunesID {
		if genreID, err := strconv.ParseUint(strings.Trim(string(playback.GenreID), `"`), 10, 32); err == nil {
			tags.ItunesGenreID = uint32(genreID)
		}
	}
}
