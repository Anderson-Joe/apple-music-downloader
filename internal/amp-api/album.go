package ampapi

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"

	"amdl/internal/download"
)

func GetAlbumResp(storefront string, id string, language string, token string) (*AlbumResp, error) {
	var err error
	if token == "" {
		token, err = GetToken()
		if err != nil {
			return nil, err
		}
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://amp-api.music.apple.com/v1/catalog/%s/albums/%s", storefront, id), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Origin", "https://music.apple.com")
	query := url.Values{}
	query.Set("omit[resource]", "autos")
	query.Set("include", "tracks,artists,record-labels")
	query.Set("include[songs]", "artists")
	//query.Set("fields[artists]", "name,artwork")
	//query.Set("fields[albums:albums]", "artistName,artwork,name,releaseDate,url")
	//query.Set("fields[record-labels]", "name")
	query.Set("extend", "editorialVideo,extendedAssetUrls")
	query.Set("l", language)
	req.URL.RawQuery = query.Encode()
	do, err := download.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	obj := new(AlbumResp)
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	if len(obj.Data[0].Relationships.Tracks.Next) > 0 {
		next := obj.Data[0].Relationships.Tracks.Next
		for {
			obj2, err := fetchTrackPage(next, token)
			if err != nil {
				return nil, err
			}
			obj.Data[0].Relationships.Tracks.Data = append(obj.Data[0].Relationships.Tracks.Data, obj2.Data...)
			next = obj2.Next
			if len(next) == 0 {
				obj.Data[0].Relationships.Tracks.Next = ""
				break
			}
		}
	}
	return obj, nil
}

func fetchTrackPage(next string, token string) (*TrackResp, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("https://amp-api.music.apple.com%s", next), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Origin", "https://music.apple.com")
	query := req.URL.Query()
	query.Set("omit[resource]", "autos")
	query.Set("include", "artists")
	query.Set("extend", "editorialVideo,extendedAssetUrls")
	req.URL.RawQuery = query.Encode()

	do, err := download.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	obj2 := new(TrackResp)
	if err := json.NewDecoder(do.Body).Decode(obj2); err != nil {
		return nil, err
	}
	return obj2, nil
}

func GetAlbumRespByHref(href string, language string, token string) (*AlbumResp, error) {
	var err error
	if token == "" {
		token, err = GetToken()
		if err != nil {
			return nil, err
		}
	}
	href = strings.Split(href, "?")[0]
	req, err := http.NewRequest("GET", fmt.Sprintf("https://amp-api.music.apple.com%s/albums", href), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Origin", "https://music.apple.com")
	query := url.Values{}
	query.Set("omit[resource]", "autos")
	query.Set("include", "tracks,artists,record-labels")
	query.Set("include[songs]", "artists")
	//query.Set("fields[artists]", "name,artwork")
	//query.Set("fields[albums:albums]", "artistName,artwork,name,releaseDate,url")
	//query.Set("fields[record-labels]", "name")
	query.Set("extend", "editorialVideo,extendedAssetUrls")
	query.Set("l", language)
	req.URL.RawQuery = query.Encode()
	do, err := download.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer do.Body.Close()
	if do.StatusCode != http.StatusOK {
		return nil, errors.New(do.Status)
	}
	obj := new(AlbumResp)
	err = json.NewDecoder(do.Body).Decode(&obj)
	if err != nil {
		return nil, err
	}
	if len(obj.Data[0].Relationships.Tracks.Next) > 0 {
		next := obj.Data[0].Relationships.Tracks.Next
		for {
			req, err := http.NewRequest("GET", fmt.Sprintf("https://amp-api.music.apple.com%s", next), nil)
			if err != nil {
				return nil, err
			}
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
			req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
			req.Header.Set("Origin", "https://music.apple.com")
			query := req.URL.Query()
			query.Set("omit[resource]", "autos")
			query.Set("include", "artists")
			query.Set("extend", "editorialVideo,extendedAssetUrls")
			req.URL.RawQuery = query.Encode()
			do, err := download.Client.Do(req)
			if err != nil {
				return nil, err
			}
			defer do.Body.Close()
			if do.StatusCode != http.StatusOK {
				return nil, errors.New(do.Status)
			}
			obj2 := new(TrackResp)
			err = json.NewDecoder(do.Body).Decode(&obj2)
			if err != nil {
				return nil, err
			}
			obj.Data[0].Relationships.Tracks.Data = append(obj.Data[0].Relationships.Tracks.Data, obj2.Data...)
			next = obj2.Next
			if len(next) == 0 {
				obj.Data[0].Relationships.Tracks.Next = ""
				break
			}
		}
	}
	return obj, nil
}

type AlbumResp struct {
	Href string          `json:"href"`
	Next string          `json:"next"`
	Data []AlbumRespData `json:"data"`
}

type AlbumRespData struct {
	ID         string `json:"id"`
	Type       string `json:"type"`
	Href       string `json:"href"`
	Attributes struct {
		Artwork struct {
			Width      int    `json:"width"`
			Height     int    `json:"height"`
			URL        string `json:"url"`
			BgColor    string `json:"bgColor"`
			TextColor1 string `json:"textColor1"`
			TextColor2 string `json:"textColor2"`
			TextColor3 string `json:"textColor3"`
			TextColor4 string `json:"textColor4"`
		} `json:"artwork"`
		ArtistName           string   `json:"artistName"`
		IsSingle             bool     `json:"isSingle"`
		URL                  string   `json:"url"`
		IsComplete           bool     `json:"isComplete"`
		GenreNames           []string `json:"genreNames"`
		TrackCount           int      `json:"trackCount"`
		IsMasteredForItunes  bool     `json:"isMasteredForItunes"`
		IsAppleDigitalMaster bool     `json:"isAppleDigitalMaster"`
		ContentRating        string   `json:"contentRating"`
		ReleaseDate          string   `json:"releaseDate"`
		Name                 string   `json:"name"`
		RecordLabel          string   `json:"recordLabel"`
		Upc                  string   `json:"upc"`
		AudioTraits          []string `json:"audioTraits"`
		Copyright            string   `json:"copyright"`
		PlayParams           struct {
			ID   string `json:"id"`
			Kind string `json:"kind"`
		} `json:"playParams"`
		IsCompilation  bool `json:"isCompilation"`
		EditorialVideo struct {
			MotionTall struct {
				Video string `json:"video"`
			} `json:"motionTallVideo3x4"`
			MotionSquare struct {
				Video string `json:"video"`
			} `json:"motionSquareVideo1x1"`
			MotionDetailTall struct {
				Video string `json:"video"`
			} `json:"motionDetailTall"`
			MotionDetailSquare struct {
				Video string `json:"video"`
			} `json:"motionDetailSquare"`
		} `json:"editorialVideo"`
	} `json:"attributes"`
	Relationships struct {
		RecordLabels struct {
			Href string        `json:"href"`
			Data []interface{} `json:"data"`
		} `json:"record-labels"`
		Artists struct {
			Href string `json:"href"`
			Data []struct {
				ID         string `json:"id"`
				Type       string `json:"type"`
				Href       string `json:"href"`
				Attributes struct {
					Name    string `json:"name"`
					Artwork struct {
						Url string `json:"url"`
					} `json:"artwork"`
				} `json:"attributes"`
			} `json:"data"`
		} `json:"artists"`
		Tracks TrackResp `json:"tracks"`
	} `json:"relationships"`
}

var releaseVersion = regexp.MustCompile(`\(([^()]*)\)|\[([^\[\]]*)\]`)

// ReleaseTypes returns release classifications in primary-then-secondary order.
// Explicit Apple Music title markers take precedence over the optional fallback.
func (a *AlbumRespData) ReleaseTypes(inferAlbum bool) []string {
	name := strings.ToLower(strings.TrimSpace(a.Attributes.Name))
	var types []string
	switch {
	case strings.HasSuffix(name, " - ep"):
		types = append(types, "EP")
	case strings.HasSuffix(name, " - single"):
		types = append(types, "Single")
	case inferAlbum && a.excludesSingleAndEP():
		types = append(types, "Album")
	}
	if a.Attributes.IsCompilation {
		types = append(types, "Compilation")
	}
	for _, version := range releaseVersion.FindAllStringSubmatch(name, -1) {
		releaseType := versionReleaseType(strings.TrimSpace(version[1] + version[2]))
		if releaseType != "" && !slices.Contains(types, releaseType) {
			types = append(types, releaseType)
		}
	}
	return types
}

func (a *AlbumRespData) excludesSingleAndEP() bool {
	attrs, tracks := &a.Attributes, &a.Relationships.Tracks
	if a.Type != "albums" || !attrs.IsComplete || attrs.IsSingle ||
		tracks.Next != "" || len(tracks.Data) != attrs.TrackCount {
		return false
	}

	// Apple's single and EP rules count songs, excluding bonus music videos.
	const singleSongLimitMS = 10 * 60 * 1000
	const epLimitMS = 30 * 60 * 1000
	var totalMS int64
	var songs int
	var hasLongSong, missingDuration bool
	for _, track := range tracks.Data {
		switch track.Type {
		case "music-videos":
			continue
		case "songs":
			songs++
		default:
			return false
		}
		duration := track.Attributes.DurationInMillis
		if duration <= 0 {
			missingDuration = true
			continue
		}
		totalMS += int64(duration)
		hasLongSong = hasLongSong || duration >= singleSongLimitMS
	}
	if songs > 6 {
		return true
	}
	// For up to six songs, a running time above 30 minutes rules out EP.
	// One to three songs still need a song of at least 10 minutes to rule out Single.
	return !missingDuration && totalMS > epLimitMS && (songs >= 4 || hasLongSong)
}

func versionReleaseType(version string) string {
	switch strings.TrimPrefix(version, "score from the ") {
	case "soundtrack", "original soundtrack", "original motion picture soundtrack",
		"original television soundtrack", "original game soundtrack",
		"original video game soundtrack", "korean original motion picture soundtrack":
		return "Soundtrack"
	}

	switch version {
	case "live", "live acoustic", "ao vivo", "en vivo", "en directo":
		return "Live"
	case "remix", "remixes":
		return "Remix"
	}
	switch {
	case strings.HasPrefix(version, "live at "), strings.HasPrefix(version, "live in "),
		strings.HasPrefix(version, "live from "), strings.HasPrefix(version, "live on "):
		return "Live"
	case strings.HasSuffix(version, " remix"):
		return "Remix"
	}
	return ""
}
