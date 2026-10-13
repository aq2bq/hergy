// Package version says which hergy this is, from what Go recorded in the
// binary at build time. hergy has no version numbers yet, so it names the
// commit and its time.
package version

import (
	"fmt"
	"regexp"
	"runtime/debug"
	"time"
)

// pseudoRE reads the time and commit out of a pseudo-version, which is what
// `go install …@<branch or commit>` records for an untagged module, and what a
// local build records for its checkout, with +dirty for uncommitted changes.
var pseudoRE = regexp.MustCompile(`^v\d+\.\d+\.\d+-(?:\S+\.)?(\d{14})-([0-9a-f]{12})(\+dirty)?$`)

// Line is the one line `hergy version` prints.
func Line() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "hergy (unknown)"
	}
	return line(info.Main.Version, info.Settings)
}

func line(module string, settings []debug.BuildSetting) string {
	if m := pseudoRE.FindStringSubmatch(module); m != nil {
		when, err := time.Parse("20060102150405", m[1])
		if err == nil {
			commit := m[2][:7]
			if m[3] != "" {
				commit += "+uncommitted"
			}
			return fmt.Sprintf("hergy %s (%s)", commit, when.Format("2006-01-02 15:04 UTC"))
		}
	}
	if module != "" && module != "(devel)" {
		return "hergy " + module
	}
	// A local go build records the checkout instead.
	var revision, at string
	modified := false
	for _, s := range settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			at = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}
	if revision == "" {
		return "hergy (devel)"
	}
	if len(revision) > 7 {
		revision = revision[:7]
	}
	out := "hergy " + revision
	if modified {
		out += "+uncommitted"
	}
	if when, err := time.Parse(time.RFC3339, at); err == nil {
		out += fmt.Sprintf(" (%s)", when.UTC().Format("2006-01-02 15:04 UTC"))
	}
	return out
}
