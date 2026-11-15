package backend

import "testing"

func TestUpdateDetection(t *testing.T) {
	cases := []struct {
		latest, current string
		want            bool
	}{
		{"v0.1.6", "v0.1.6", false},            // AppImage / AUR sul tag esatto
		{"v0.1.6", "v0.1.6-2-g815e9c1", false}, // AUR qualche commit dopo il tag
		{"v0.1.6", "v0.1.5", true},
		{"v0.1.6", "v0.1.5-3-gabc1234", true},
		{"v0.2.0", "v0.1.9", true},
		{"v0.1.6", "v0.1.7", false},
		{"v0.1.6", "r18.815e9c1", false}, // vecchio pkgver AUR: non confrontabile
		{"v0.1.6", "dev", false},         // wails dev
	}
	for _, c := range cases {
		got := isSemver(c.current) && compareVersions(c.latest, c.current) > 0
		if got != c.want {
			t.Errorf("latest=%s current=%s: update=%v, want %v", c.latest, c.current, got, c.want)
		}
	}
}
