package ampapi

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"amdl/internal/download"
)

type creditsRoundTripper func(*http.Request) (*http.Response, error)

func (f creditsRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGetSongCredits(t *testing.T) {
	client := download.Client
	t.Cleanup(func() { download.Client = client })
	download.Client = &http.Client{Transport: creditsRoundTripper(func(req *http.Request) (*http.Response, error) {
		if got := req.URL.Path; got != "/v1/catalog/tw/songs/1717034230" {
			t.Errorf("catalog path = %q", got)
		}
		if got := req.URL.Query().Get("include"); got != "composers,credits" {
			t.Errorf("include = %q", got)
		}
		if got := req.URL.Query().Get("l"); got != "zh-Hant" {
			t.Errorf("language = %q", got)
		}
		if _, timed := req.Context().Deadline(); !timed {
			t.Error("credits request has no deadline")
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Body: io.NopCloser(strings.NewReader(`{"data":[
				{"id":"other"},
				{"id":"1717034230","relationships":{
					"composers":{"data":[{"attributes":{"name":"Composer"}}]},
					"credits":{"data":[{"attributes":{"kind":"production-and-engineering"},
						"relationships":{"credit-artists":{"data":[{"attributes":{"name":"Producer","roleNames":["製作人"]}}]}}}]}
				}}
			]}`)),
		}, nil
	})}
	credits, err := GetSongCredits("tw", "1717034230", "zh-Hant", "test-token")
	if err != nil {
		t.Fatal(err)
	}
	if credits == nil || len(credits.Composers.Data) != 1 || len(credits.Credits.Data) != 1 {
		t.Fatalf("unexpected credits response: %#v", credits)
	}
	producer := credits.Credits.Data[0].Relationships.Artists.Data
	if len(producer) != 1 || producer[0].Attributes.Name != "Producer" ||
		len(producer[0].Attributes.Roles) != 1 || producer[0].Attributes.Roles[0] != "製作人" {
		t.Fatalf("unexpected production credits: %#v", producer)
	}
}
