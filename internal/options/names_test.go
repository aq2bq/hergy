package options

import (
	"testing"
)

func TestCommandWordsKeepTheirUnresolvedMeaning(t *testing.T) {
	for _, tc := range []struct {
		argv                     []string
		definition, team, target string
	}{
		{[]string{"up", "Upper"}, "Upper", "", ""},
		{[]string{"show", "Upper"}, "Upper", "", ""},
		{[]string{"status", "Upper"}, "", "Upper", ""},
		{[]string{"down", "Upper-1"}, "", "", "Upper-1"},
	} {
		o, err := Parse(tc.argv)
		if err != nil {
			t.Fatalf("%v: %v", tc.argv, err)
		}
		if o.Definition.String() != tc.definition || o.Team.String() != tc.team || o.Target.String() != tc.target {
			t.Fatalf("%v: definition=%q team=%q target=%q", tc.argv, o.Definition, o.Team, o.Target)
		}
	}
}
