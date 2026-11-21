package wrapper

import (
	"errors"
	"net/url"
)

// Webplayback queries the wrapper-lite /webplayback endpoint for the given adamID and returns the m3u8 URL.
func (c *Client) Webplayback(adamID string) (string, error) {
	data, err := c.WebplaybackInfo(adamID)
	return data.M3u8, err
}

// WebplaybackInfo returns the playback URL and any metadata available from the wrapper.
func (c *Client) WebplaybackInfo(adamID string) (WebplaybackData, error) {
	endpoint := "/webplayback?adamId=" + url.QueryEscape(adamID)
	body, err := c.get(endpoint)
	if err != nil {
		return WebplaybackData{}, err
	}
	data, err := decodeEnvelope[WebplaybackData](body, endpoint)
	if err != nil {
		return WebplaybackData{}, err
	}
	if data.M3u8 == "" {
		return WebplaybackData{}, errors.New("Unavailable")
	}
	return data, nil
}

// GetWebplayback queries wrapper-lite's /webplayback endpoint using baseURL.
func GetWebplayback(baseURL, adamID string) (string, error) {
	return New(baseURL).Webplayback(adamID)
}
