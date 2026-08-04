package resources

import "testing"

// TestPackAsset guards the asset-selection rule that keeps resource-pack
// delivery correct: every release ships a `.mcpack` and an identically-sized
// `.zip`, and picking the wrong one silently feeds servers a file BDS cannot
// load. Ordering must not matter.
func TestPackAsset(t *testing.T) {
	mcpack := ReleaseAsset{Name: "PokeBedrock RES 2.18.5.mcpack", BrowserDownloadURL: "https://example.invalid/pack.mcpack"}
	zip := ReleaseAsset{Name: "PokeBedrock RES 2.18.5.zip", BrowserDownloadURL: "https://example.invalid/pack.zip"}

	cases := []struct {
		name    string
		assets  []ReleaseAsset
		wantURL string
		wantOK  bool
	}{
		{name: "mcpack first", assets: []ReleaseAsset{mcpack, zip}, wantURL: mcpack.BrowserDownloadURL, wantOK: true},
		{name: "zip first", assets: []ReleaseAsset{zip, mcpack}, wantURL: mcpack.BrowserDownloadURL, wantOK: true},
		{name: "only mcpack", assets: []ReleaseAsset{mcpack}, wantURL: mcpack.BrowserDownloadURL, wantOK: true},
		{name: "no mcpack", assets: []ReleaseAsset{zip}, wantOK: false},
		{name: "no assets", assets: nil, wantOK: false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			release := &Release{TagName: "v2.18.5", Assets: tc.assets}
			asset, ok := release.packAsset()
			if ok != tc.wantOK {
				t.Fatalf("packAsset() ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && asset.BrowserDownloadURL != tc.wantURL {
				t.Errorf("packAsset() URL = %q, want %q", asset.BrowserDownloadURL, tc.wantURL)
			}
		})
	}
}
