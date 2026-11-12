package ops

import (
	"testing"

	"github.com/aq2bq/hergy/internal/activity"
	domainname "github.com/aq2bq/hergy/internal/domain/name"
	"github.com/aq2bq/hergy/internal/options"
)

func downInput(raw string) options.Input {
	o, e := options.Parse([]string{"down", raw})
	if e != nil {
		panic(e)
	}
	return o.Target
}
func TestDownChoiceKeepsLegacyWorkersAndRefusesAmbiguity(t *testing.T) {
	team := fixtureSeen("Upper")
	worker := options.Worker{Team: team, Short: fixtureSeen("1"), Herdr: fixtureSeen("Upper-1"), Label: fixtureSeen("worker:Upper/1")}
	choice, err := downInput("Upper-1").ResolveDown([]domainname.Seen{team}, []options.Worker{worker})
	if err != nil {
		t.Fatal(err)
	}
	selected, ok := choice.(options.DownWorker)
	if !ok || selected.Worker != worker {
		t.Fatalf("lost observed worker: %#v", choice)
	}
	choice, err = downInput("Upper").ResolveDown([]domainname.Seen{team}, []options.Worker{worker})
	if err != nil {
		t.Fatal(err)
	}
	if selected, ok := choice.(options.DownTeam); !ok || selected.Team != team {
		t.Fatalf("lost observed team: %#v", choice)
	}
	worker.Short = team
	if _, err := downInput("Upper").ResolveDown([]domainname.Seen{team}, []options.Worker{worker}); err == nil {
		t.Fatal("accepted a team/worker collision")
	}
}

func TestUnreadableDefinitionKeepsObservedRowsOutsideMemberPattern(t *testing.T) {
	got := activity.Of(activity.Input{Team: fixtureSeen("Upper"), Graph: nil, Now: activityNow, Agents: []activity.Agent{
		{Herdr: fixtureSeen("Upper-1"), Node: fixtureSeen("1"), Status: "working", Seq: 1},
		{Herdr: fixtureSeen("Upper-Other"), Node: fixtureSeen("Other"), Status: "idle", Seq: 2},
	}})
	if got.State != activity.Working || len(got.Nodes) != 2 || len(got.Hired) != 0 {
		t.Fatalf("observed rows changed: %+v", got)
	}
	byName := map[string]activity.Node{}
	for _, node := range got.Nodes {
		byName[node.Name] = node
	}
	if byName["1"].Herdr.String() != "Upper-1" || byName["Other"].Herdr.String() != "Upper-Other" {
		t.Fatalf("lost observed names: %+v", got.Nodes)
	}
}
