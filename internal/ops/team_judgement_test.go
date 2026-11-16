package ops

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/aq2bq/hergy/internal/ops/internal/protocol"
	"github.com/aq2bq/hergy/internal/ops/internal/turns"
)

type unreadableWatcherPort struct{ Port }

func (p unreadableWatcherPort) JSON(args ...string) (map[string]any, error) {
	if len(args) > 1 && args[0] == "pane" && args[1] == "list" {
		return nil, errors.New("panes unavailable")
	}
	return p.Port.JSON(args...)
}

func TestStatusAndActivityUseTheSameTurnJudgement(t *testing.T) {
	cases := []struct {
		name, holder, agent, status, api string
		watching                         bool
	}{
		{"permission", turns.Human, "blocked", "許可待ち", "answer", true},
		{"human turn", turns.Human, "idle", "人間の返事待ち", "turn", true},
		{"member turn stopped", "a", "idle", "止まっている", "stalled", true},
		{"no watcher", turns.Human, "idle", "止まっている", "stalled", false},
		{"no turn state", "", "idle", "止まっている", "stalled", true},
		{"working", "a", "working", "仕事中", "working", true},
		{"human before working", turns.Human, "working", "人間の返事待ち", "turn", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			f, env, _, port, _ := stopFixture(t)
			port.snapshot.Agents[0].Status = c.agent
			if c.watching {
				port.snapshot.Panes = append(port.snapshot.Panes, protocol.Pane{ID: "watch", Label: fixtureSeen("demo-turns"), TabID: "t1"})
			}
			if c.holder != "" {
				if err := turns.Save(turns.Path(filepath.Join(f.state, "hergy"), "demo"), &turns.State{Team: fixtureSeen("demo"), Holder: c.holder, From: "a", Since: activityNow}); err != nil {
					t.Fatal(err)
				}
			}
			status, err := Status(context.Background(), StatusInput{MemberInput: MemberInput{Env: env, Port: port}, Now: func() time.Time { return activityNow }})
			if err != nil {
				t.Fatal(err)
			}
			reader := newActivityRead(env)
			reader.port, reader.now = port, func() time.Time { return activityNow }
			api := reader.List()
			if len(status) != 1 || len(api.Items) != 1 {
				t.Fatalf("status=%+v api=%+v", status, api)
			}
			if status[0].Judgement != c.status || api.Items[0].Summary.State != c.api {
				t.Fatalf("status=%q api=%q; want %q / %q", status[0].Judgement, api.Items[0].Summary.State, c.status, c.api)
			}
		})
	}
}

func TestActivityBadgeAndTurnLineUseOneStateReading(t *testing.T) {
	f, env, r, port, _ := stopFixture(t)
	port.snapshot.Panes = append(port.snapshot.Panes, protocol.Pane{ID: "watch", Label: fixtureSeen("demo-turns"), TabID: "t1"})
	path := turns.Path(filepath.Join(f.state, "hergy"), "demo")
	st := &turns.State{Team: fixtureSeen("demo"), Holder: turns.Human, From: "a", Since: activityNow}
	if err := turns.Save(path, st); err != nil {
		t.Fatal(err)
	}
	reader := newActivityRead(env)
	reader.port = port
	judged := judgeRead(runningTeam{Name: fixtureSeen("demo"), Built: r}, reader.readHerdr(activityNow), activityNow)
	st.Holder = "b"
	if err := turns.Save(path, st); err != nil {
		t.Fatal(err)
	}
	item := itemOf(activityRow{team: judged}, activityNow)
	if item.Summary.State != "turn" || item.Summary.Turn == nil || item.Summary.Turn.Holder != turns.Human {
		t.Fatalf("badge and turn diverged: %+v", item.Summary)
	}
}

func TestUnwatchedTurnDetailsAreNotPresented(t *testing.T) {
	for _, unknown := range []bool{false, true} {
		t.Run(map[bool]string{false: "absent", true: "unreadable"}[unknown], func(t *testing.T) {
			f, env, r, port, _ := stopFixture(t)
			st := &turns.State{Team: fixtureSeen("demo"), Holder: turns.Human, From: "a", Since: activityNow, Opened: map[string][]turns.Opening{"a": {{By: "b"}}}}
			if err := turns.Save(turns.Path(filepath.Join(f.state, "hergy"), "demo"), st); err != nil {
				t.Fatal(err)
			}
			var p Port = port
			if unknown {
				p = unreadableWatcherPort{p}
			}
			input := StatusInput{MemberInput: MemberInput{Env: env, Port: p, Team: downInput("demo")}, Now: func() time.Time { return activityNow }}
			result, err := Status(context.Background(), input)
			if err != nil {
				t.Fatal(err)
			}
			if len(result) != 1 || len(result[0].Turn) != 0 || result[0].NoWatcher == "" {
				t.Fatalf("unwatched CLI details = %+v", result)
			}
			judged := judgeOf(runningTeam{Name: fixtureSeen("demo"), Built: r}, readSession(p), activityNow)
			item := itemOf(activityRow{team: judged}, activityNow)
			if item.Summary.State != "stalled" || item.Summary.Turn != nil {
				t.Fatalf("unwatched card details = %+v", item.Summary)
			}
		})
	}
}
