package launcher

import (
	"bytes"
	"strings"
	"testing"

	"github.com/aq2bq/hergy/internal/definition"
	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/protocol"
)

// inLanguage reads the printed lines in lang from here on, and puts the fallback
// back afterwards so a test that reads another language is not left there.
func inLanguage(t *testing.T, lang string) {
	t.Helper()
	if err := i18n.Use(lang); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := i18n.Use(i18n.Fallback); err != nil {
			t.Error(err)
		}
	})
}

// blockedRefresh refreshes one live member whose start herdr refuses because the
// agent asks a human a question while it starts. answer runs before each reading
// of the member while it is blocked, so a test answers the question there (sets
// it idle); settle runs before each reading after it stopped asking, so a test
// models an agent that ends instead of taking input (deletes it). Either may be
// nil. The launcher, what it printed and the refresh's own error come back.
func blockedRefresh(t *testing.T, answer, settle func(port *fakePort, name string)) (*Launcher, *fakePort, *bytes.Buffer, error) {
	t.Helper()
	port := &fakePort{
		agents: map[string]protocol.Agent{
			"team-lead": {Name: "team-lead", Kind: "claude", Status: "idle", PaneID: "p1", TabID: "t7"},
		},
		gone:           map[string]bool{},
		process:        map[string][2]int{"p1": {4242, 900}},
		tabs:           []protocol.Tab{{ID: "t7", Label: "team"}},
		startupBlocked: map[string]bool{"team-lead": true},
		startupTab:     "t7",
	}
	l, out := downLauncher(t, port, "team-lead")
	l.timeout = 3000
	l.kill = func(_ int) error {
		port.gone["team-lead"] = true
		return nil
	}
	readings := 0
	if answer != nil {
		port.onStartupGet = func(name string) {
			readings++
			if readings > 1 {
				answer(port, name)
			}
		}
	}
	if settle != nil {
		port.onSettleGet = func(name string) { settle(port, name) }
	}
	err := l.Refresh(l.starts, "")
	return l, port, out, err
}

// answered sets the member idle: the person answered the question in the pane.
func answered(port *fakePort, name string) {
	agent := port.agents[name]
	agent.Status = "idle"
	port.agents[name] = agent
}

// TestStartupQuestionIsWaitedOnAndTheBootGoesOn: herdr refuses the start while
// the agent asks a question, so hergy names the member, the tab, the pane and the
// kind and waits; once the answer is in, it says it goes on and submits the
// bootstrap prompt, in that order, and answers nothing itself.
func TestStartupQuestionIsWaitedOnAndTheBootGoesOn(t *testing.T) {
	inLanguage(t, "ja")
	_, port, out, err := blockedRefresh(t, answered, nil)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	printed := out.String()
	for _, want := range []string{
		"許可待ち: team-lead が、起動の途中で確認に答えを待っています（タブ team（t7）、ペイン p1）",
		"エージェント（claude）が、このフォルダを信頼するか、などを聞いています。そのペインを開いて答えてください。確認は続けて出ることがあります。",
		"答え終わると、hergy は team-lead に初期プロンプトを送って続きを進めます。やめるときは Ctrl-C を押してください。",
		"team-lead が確認を終えて起動しました。続けます",
		"送りました: team-lead（ペイン p1）",
	} {
		if !strings.Contains(printed, want) {
			t.Fatalf("the boot does not say %q:\n%s", want, printed)
		}
	}
	blocked := strings.Index(printed, "許可待ち: team-lead")
	unblocked := strings.Index(printed, "が確認を終えて起動しました")
	prompt := strings.Index(printed, "送りました: team-lead")
	if blocked > unblocked || unblocked > prompt {
		t.Fatalf("the wait, the answer and the prompt came in another order:\n%s", printed)
	}
	trail := commandTrail(port.calls)
	if got := strings.Count(strings.Join(trail, "\n"), "agent start"); got != 1 {
		t.Fatalf("the start was tried %d times, want once (herdr keeps the agent):\n%v", got, trail)
	}
	for _, never := range []string{"agent send-keys", "agent focus"} {
		if strings.Contains(strings.Join(trail, "\n"), never) {
			t.Fatalf("hergy %s while a person answers:\n%v", never, trail)
		}
	}
}

// TestStartupQuestionIsWaitedOnAndTheBootGoesOnInEnglish is the same boot read in
// English: the waiting line and the line that goes on are the ones the wording
// gives for the language in use.
func TestStartupQuestionIsWaitedOnAndTheBootGoesOnInEnglish(t *testing.T) {
	inLanguage(t, "en")
	_, _, out, err := blockedRefresh(t, answered, nil)
	if err != nil {
		t.Fatalf("Refresh: %v", err)
	}
	printed := out.String()
	for _, want := range []string{
		"waiting for permission: team-lead is asking a question while it starts (tab team (t7), pane p1)",
		"The agent (claude) is asking, for example, whether to trust this folder. Open that pane and answer it. More than one question may come in a row.",
		"When they are answered, hergy hands team-lead its bootstrap prompt and goes on. Press Ctrl-C to stop.",
		"team-lead answered and started; going on",
		"submitted: team-lead (pane p1)",
	} {
		if !strings.Contains(printed, want) {
			t.Fatalf("the boot does not say %q:\n%s", want, printed)
		}
	}
}

// TestStartupQuestionThatEndsTheAgentIsAFailure: a person who answers by ending
// the agent (it was told not to trust the folder) leaves the start failed. herdr
// shows that agent idle for a moment while it exits and forgets it right after,
// so the wait reads it once more before it goes on: it is gone, and the start is
// a failure. The error names the way on, nothing is submitted, and the report
// says where the start stopped rather than that the member still waits.
func TestStartupQuestionThatEndsTheAgentIsAFailure(t *testing.T) {
	inLanguage(t, "ja")
	ending := func(port *fakePort, name string) { delete(port.agents, name) }
	l, _, out, err := blockedRefresh(t, answered, ending)
	if err == nil {
		t.Fatal("the refresh went on after the agent ended")
	}
	for _, want := range []string{
		"team-lead のエージェント（claude）が、起動の途中の確認のあとで終わりました",
		"hergy refresh team-lead を実行して、確認に答えてください",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("the error does not say %q: %v", want, err)
		}
	}
	if strings.Contains(out.String(), "送りました") {
		t.Fatalf("hergy submitted a bootstrap prompt to an agent that ended:\n%s", out.String())
	}
	var report bytes.Buffer
	l.ReportFailure(&report)
	if !strings.Contains(report.String(), "止まったところ: team-lead を起動しています") {
		t.Fatalf("the report does not say the start stopped:\n%s", report.String())
	}
	if strings.Contains(report.String(), "許可待ち（起動の途中") {
		t.Fatalf("the report still says the member waits:\n%s", report.String())
	}
}

// TestStartupNextNamesTheWayOnForEachCommand: a boot, a hire and a refresh each
// say how to go on from their own kind of member, in both languages. The ended
// message and the line an interrupt prints are these same words.
func TestStartupNextNamesTheWayOnForEachCommand(t *testing.T) {
	member := &definition.Start{Name: "demo-lead", Kind: "claude"}
	worker := &definition.Start{Name: "demo-w1", Kind: "claude"}
	cases := []struct {
		name    string
		lang    string
		next    StartupNext
		member  *definition.Start
		tab     string
		line    string
		command string
	}{
		{
			name: "up ja", lang: "ja", next: NextUp("demo", "hergy up demo --as hot"), member: member, tab: "demo（t1）",
			line:    "続けるには: タブ demo（t1）の demo-lead のペインで確認に答えてから、hergy down demo のあと hergy up demo --as hot を実行してください\n",
			command: "hergy down demo のあと hergy up demo --as hot",
		},
		{
			name: "hire ja", lang: "ja", next: NextHire(), member: worker, tab: "worker:demo/w1（t2）",
			line:    "続けるには: タブ worker:demo/w1（t2）の demo-w1 のペインで確認に答えてから、そのタブを herdr で閉じて、同じ hergy hire をもう一度実行してください\n",
			command: "そのタブを herdr で閉じたあと、同じ hergy hire",
		},
		{
			name: "refresh ja", lang: "ja", next: NextRefresh(), member: member, tab: "demo（t1）",
			line:    "続けるには: タブ demo（t1）の demo-lead のペインで確認に答えてから、hergy refresh demo-lead をもう一度実行してください\n",
			command: "hergy refresh demo-lead",
		},
		{
			name: "up en", lang: "en", next: NextUp("demo", "hergy up demo --as hot"), member: member, tab: "demo (t1)",
			line:    "to go on: answer the question in demo-lead's pane (tab demo (t1)), then run hergy down demo and then hergy up demo --as hot\n",
			command: "hergy down demo and then hergy up demo --as hot",
		},
		{
			name: "hire en", lang: "en", next: NextHire(), member: worker, tab: "worker:demo/w1 (t2)",
			line:    "to go on: answer the question in demo-w1's pane (tab worker:demo/w1 (t2)), close that tab in herdr, then run the same hergy hire again\n",
			command: "the same hergy hire after closing that tab in herdr",
		},
		{
			name: "refresh en", lang: "en", next: NextRefresh(), member: member, tab: "demo (t1)",
			line:    "to go on: answer the question in demo-lead's pane (tab demo (t1)), then run hergy refresh demo-lead again\n",
			command: "hergy refresh demo-lead",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			inLanguage(t, tc.lang)
			line := tc.next.Line(tc.member, tc.tab)
			command := tc.next.Command(tc.member)
			if line != tc.line {
				t.Errorf("line = %q, want %q", line, tc.line)
			}
			if command != tc.command {
				t.Errorf("command = %q, want %q", command, tc.command)
			}
		})
	}
}
