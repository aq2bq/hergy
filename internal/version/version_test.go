package version

import (
	"runtime/debug"
	"testing"
	"time"
)

// fixedZone pins time.Local so the "2006-01-02 15:04 MST" stamps are the same
// wherever the test runs: 03:04 UTC is 12:04 JST.
func fixedZone(t *testing.T) {
	t.Helper()
	old := time.Local
	time.Local = time.FixedZone("JST", 9*60*60)
	t.Cleanup(func() { time.Local = old })
}

func settings(pairs ...string) []debug.BuildSetting {
	var out []debug.BuildSetting
	for i := 0; i+1 < len(pairs); i += 2 {
		out = append(out, debug.BuildSetting{Key: pairs[i], Value: pairs[i+1]})
	}
	return out
}

func TestLine(t *testing.T) {
	fixedZone(t)
	cases := []struct {
		name     string
		module   string
		settings []debug.BuildSetting
		want     string
	}{
		{
			name:   "pseudo version",
			module: "v0.0.0-20240102030405-abcdef123456",
			want:   "hergy abcdef1 (2024-01-02 12:04 JST)",
		},
		{
			name:   "pseudo version with a branch prefix",
			module: "v1.2.3-0.20240102030405-abcdef123456",
			want:   "hergy abcdef1 (2024-01-02 12:04 JST)",
		},
		{
			name:   "pseudo version with uncommitted changes",
			module: "v0.0.0-20240102030405-abcdef123456+dirty",
			want:   "hergy abcdef1+uncommitted (2024-01-02 12:04 JST)",
		},
		{
			name:   "tagged version",
			module: "v1.2.3",
			want:   "hergy v1.2.3",
		},
		{
			name:   "(devel) with VCS information",
			module: "(devel)",
			settings: settings(
				"vcs.revision", "abcdef1234567890",
				"vcs.time", "2024-01-02T03:04:05Z",
				"vcs.modified", "false",
			),
			want: "hergy abcdef1 (2024-01-02 12:04 JST)",
		},
		{
			name:   "(devel) with VCS information and uncommitted changes",
			module: "(devel)",
			settings: settings(
				"vcs.revision", "abcdef1234567890",
				"vcs.time", "2024-01-02T03:04:05Z",
				"vcs.modified", "true",
			),
			want: "hergy abcdef1+uncommitted (2024-01-02 12:04 JST)",
		},
		{
			name:     "(devel) without VCS information",
			module:   "(devel)",
			settings: settings("GOOS", "darwin", "GOARCH", "arm64"),
			want:     "hergy (devel)",
		},
		{
			name:   "(devel) with a revision but no time",
			module: "(devel)",
			settings: settings(
				"vcs.revision", "abc1234",
				"vcs.modified", "false",
			),
			want: "hergy abc1234",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := line(tc.module, tc.settings); got != tc.want {
				t.Errorf("line(%q) = %q, want %q", tc.module, got, tc.want)
			}
		})
	}
}
