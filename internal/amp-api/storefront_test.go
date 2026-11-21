package ampapi

import "testing"

func TestItunesStorefrontID(t *testing.T) {
	for _, tt := range []struct {
		name, href, requested string
		want                  uint32
	}{
		{name: "catalog overrides requested region", href: "/v1/catalog/us/songs/1717034230", requested: "cn", want: 143441},
		{name: "absolute catalog URL", href: "https://amp-api.music.apple.com/v1/catalog/jp/songs/1717034230", want: 143462},
		{name: "missing song href", requested: " CN ", want: 143465},
		{name: "unknown catalog region", href: "/v1/catalog/zz/songs/1717034230", requested: "us"},
		{name: "unknown requested region", requested: "zz"},
		{name: "missing region"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := ItunesStorefrontID(tt.href, tt.requested); got != tt.want {
				t.Fatalf("storefront ID = %d, want %d", got, tt.want)
			}
		})
	}
}
