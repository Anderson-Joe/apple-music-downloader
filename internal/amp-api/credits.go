package ampapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"amdl/internal/download"
)

type SongCredits struct {
	Composers struct {
		Next string `json:"next"`
		Data []struct {
			ID         string `json:"id"`
			Attributes struct {
				Name string `json:"name"`
			} `json:"attributes"`
		} `json:"data"`
	} `json:"composers"`
	Credits struct {
		Data []struct {
			Attributes struct {
				Kind string `json:"kind"`
			} `json:"attributes"`
			Relationships struct {
				Artists struct {
					Data []struct {
						Attributes struct {
							Name  string   `json:"name"`
							Roles []string `json:"roleNames"`
						} `json:"attributes"`
					} `json:"data"`
				} `json:"credit-artists"`
			} `json:"relationships"`
		} `json:"data"`
	} `json:"credits"`
}

// GetSongCredits requests the structured composer relationship and the role
// credits exposed by the Apple Music web catalog.
func GetSongCredits(storefront, id, language, token string) (*SongCredits, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := newCatalogSongRequest(ctx, storefront, id, token)
	if err != nil {
		return nil, err
	}
	query := req.URL.Query()
	query.Set("include", "composers,credits")
	if language != "" {
		query.Set("l", language)
	}
	req.URL.RawQuery = query.Encode()
	resp, err := download.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Apple Music credits: %s", resp.Status)
	}
	var result struct {
		Data []struct {
			ID            string      `json:"id"`
			Relationships SongCredits `json:"relationships"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	for _, song := range result.Data {
		if song.ID == id {
			return &song.Relationships, nil
		}
	}
	return nil, nil
}
