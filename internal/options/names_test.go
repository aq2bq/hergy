package options

import (
	"testing"

	domainname "github.com/aq2bq/hergy/internal/domain/name"
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

func TestDownChoiceKeepsLegacyWorkersAndRefusesAmbiguity(t *testing.T) {
	team := domainname.Observe("Upper")
	worker := Worker{Team: team, Short: domainname.Observe("1"), Herdr: domainname.Observe("Upper-1"), Label: domainname.Observe("worker:Upper/1")}
	choice, err := (Input{raw: "Upper-1"}).ResolveDown([]domainname.Seen{team}, []Worker{worker})
	if err != nil {
		t.Fatal(err)
	}
	selected, ok := choice.(DownWorker)
	if !ok || selected.Worker != worker {
		t.Fatalf("lost observed worker: %#v", choice)
	}
	choice, err = (Input{raw: "Upper"}).ResolveDown([]domainname.Seen{team}, []Worker{worker})
	if err != nil {
		t.Fatal(err)
	}
	if selected, ok := choice.(DownTeam); !ok || selected.Team != team {
		t.Fatalf("lost observed team: %#v", choice)
	}
	worker.Short = team
	if _, err := (Input{raw: "Upper"}).ResolveDown([]domainname.Seen{team}, []Worker{worker}); err == nil {
		t.Fatal("accepted a team/worker collision")
	}
}
