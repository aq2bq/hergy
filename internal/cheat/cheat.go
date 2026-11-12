// Package cheat holds hergy's cheatsheets: a short one for people, in the
// language in use, and a complete one for an agent. Neither touches Herdr, and
// neither reads the team settings, so both print on a machine with broken
// settings and no Herdr. The short one's language comes from --lang and the
// global config.yaml, which the caller resolves before it asks for the text.
package cheat

import (
	"fmt"
	"strings"

	"github.com/aq2bq/hergy/internal/i18n"
	"github.com/aq2bq/hergy/internal/options"
)

// People is the short cheatsheet, in the language in use. The wording lives in
// the locales; the shape is the first three steps a newcomer types, then a
// short what-you-want-to-command table.
func People() string { return i18n.T("cheat.people") }

// Machine is the agent cheatsheet. It is English whatever --lang says: an agent
// reads it on its own, without the locales, and needs the whole shape, not a
// translation. It is built when hergy starts, so it reflects the shape hergy is
// built with, not the one on this machine.
//
// What the commands accept is not written here. Each command's line is the
// Synopsis of its row in internal/options/spec.go, and each flag's line ends
// with the commands that take it, read from the same table. What the commands
// and the flags do is written here by hand, in behavior and flagMeaning.
var Machine = buildMachine()

func buildMachine() string {
	var b strings.Builder
	b.WriteString(head)
	for _, kind := range options.Kinds {
		fmt.Fprintf(&b, "  %s\n%s", options.Synopsis(kind, "en"), behavior[kind])
	}
	b.WriteString("\nFlags:\n")
	b.WriteString(flagLines())
	b.WriteString("\n")
	b.WriteString(tail)
	return b.String()
}

// flagLines is one line per flag: its spelling, what it means, and the commands
// that take it (a flag every command takes, --lang and --help, names none).
func flagLines() string {
	summaries := options.FlagSummaries()
	width := 0
	for _, s := range summaries {
		width = max(width, len(s.Label))
	}
	var b strings.Builder
	for _, s := range summaries {
		meaning := flagMeaning[s.Name]
		if len(s.Commands) > 0 {
			meaning += " Commands: " + strings.Join(s.Commands, ", ") + "."
		}
		fmt.Fprintf(&b, "  %-*s  %s\n", width, s.Label, meaning)
	}
	return b.String()
}

// head is what comes before the command lines.
const head = `hergy cheat --ai - the whole of hergy, for an agent that will not read anything else

A team definition is a directory, teams/<definition>/ under .hergy/ or ~/.config/hergy/. A running
team has a team name: the team definition name by default, or another name given
to hergy up --as. Commands that read or write a team definition take a team
definition name (up, new, show); commands that act on a running team take a team
name (down, status, apply, refresh, reprompt, hire). A running team's tab is labelled
with the team's name alone, and a team command finds the definition behind it in
this machine's launch record ($XDG_STATE_HOME/hergy/launches/log.jsonl). The record
also keeps one line for each worker a member hired, naming the member that added
it. A tab is
one of hergy's when a live agent or a named pane in it is named after its label
("<team>-<member>"), or when it still has the older shape
"team:<team>@<definition>" ("team:<team>", where the definition name is the team
name): a team booted by an older hergy is still found from its label, and another
tool's tab, or a person's tab that happens to carry a team's name, is not a team.
When herdr's agent list cannot be read at all, a label this machine recorded a
launch of is taken as a team's instead.

Commands (hergy <command> --help prints one command's page, and every command but version takes it; a bare -- ends the flags, and --flag=value is accepted):
`

// startupQuestion is what a command that starts an agent says about a member that
// asks a question while it starts: hergy waits for the person's answer in that
// pane, and answers nothing itself (d-d589).
const startupQuestion = ` If the agent asks a question while it starts (whether to trust this folder, for example), hergy names the member, the tab and the pane, waits for the answer with no limit, then sends the bootstrap prompt, confirms receipt and goes on; a second question in a row prints nothing more. Ctrl-C stops the wait, and the line it ends with names the way on. hergy answers nothing itself and moves no focus.`

// confirmedPrompt is the receipt contract shared by all initial-prompt commands.
const confirmedPrompt = `      "Sent" means herdr observed working within five seconds. Retry the identical prompt once on any sending error. If the final answer is agent_prompt_stalled, timeout or agent_blocked, name the recipient and pane, show how to send again, continue with the remaining recipients, and finish with failure listing the missing receipts. Other sending or startup failures stop the operation immediately. Only confirmed recipients get success lines.
`

// behavior is what each command does, under its command line. Every line starts
// with six spaces.
var behavior = map[options.Kind]string{
	options.Up: `      Boot the team definition in a new tab. --as names the team; without it the team name is the definition name. --dry prints the plan and submits nothing; --json, only with --dry, prints it as JSON.` + startupQuestion + `
` + confirmedPrompt,
	options.Down: `      Close the panes of the running team's members and of the workers it hired. A team is what hergy made for it: the tab labelled with its name, and the panes named after it, "<team>-<member>". A pane whose agent ended, and one whose member never started (a boot stopped at a startup question), are closed too, so nothing hergy made is left; a pane hergy did not name is a person's own and stays. A name that is not a running team is looked for as one worker of a running team (its short name, or its herdr name "<team>-<worker>"): only that worker's pane is closed, and the worker's own tab when that pane was the last one in it; the members and the other workers stay. After closing a team's panes, down waits up to 30000 ms in total for herdr to forget their names. A timeout fails and lists the remaining names. From a team pane, it waits for the other names before closing your own pane last; your own name cannot be awaited. On timeout it keeps your own pane open: run hergy down again after the names disappear. --dry never waits. For a team, both the CLI and screen list remaining worktrees and clear its turn state, pending signals and tells on success or a name-release timeout; a timeout still fails. A screen stop also cancels this serve's pending start of that team. An empty team tab can be stopped by either entry point without closing the tab itself. The screen reports only member and worker panes actually closed; the watcher is excluded. Other errors stop before this cleanup. For one worker, down waits for herdr to forget its name before reporting success; a timeout fails, so a following hire does not reuse a name still held. A name that is both a team's and a worker's is refused, naming both. After closing, it lists the git worktrees that came while the team was running (after the worker was hired, for one worker), other than the project root: an agent keeps its pane in the project root and cds into a worktree, so the pane's own directory is not enough to find them. The time comes from the launch record, and a worktree a closed pane stood in is marked. Each worktree comes with its branch, uncommitted changes and commits not in main, and the project's stashes, with the commands that remove them; hergy removes nothing, and temporary documents are not listed (hergy cannot tell which are the team's). --dry reads herdr's live agents, tabs and panes, prints what would close, and closes nothing; --json, only with --dry, prints the plan and its leftovers as JSON.
`,
	options.Status: `      With a team name: one line per member and per worker, its live agent_status (working/idle/done/blocked), or "not present" when the member is not live; a worker carries "(worker)". Then a judgement line: "waiting for a human: <names>" when any is blocked, else "nobody is working" when none is working or blocked. With no team name: every running team, each with its team definition name, its member and worker counts, and the same judgement. It only reads herdr, and runs outside a herdr-managed pane too.
`,
	options.Refresh: `      Restart each selected member. A live member: stop its process group, wait for herdr to forget the name, then start it again in the same pane and submit the bootstrap prompt. A member that is not live: start it again in the pane hergy named after it, when that pane is still at its shell. A member with no pane is refused, and hergy apply is named: adding a member to a running team is the apply's road. Every member a refresh starts starts with an empty conversation. The settings are read again first, so an edited profile, kind, args or prompt takes effect; env does not (it is fixed when the pane is made).` + startupQuestion + `
      Both <member> and --all is an error; neither is an error.
      A member running in your own pane (HERDR_PANE_ID) cannot refresh itself: stopping it would stop hergy first. That is an error, and nothing is stopped.
      If any selected member is not idle or done, is your own pane, or (when not live) has a pane that is busy, nothing is changed and the whole command errors. A busy pane points at hergy down then hergy up.
      A member with no live agent is refused, and hergy apply is named: a refresh makes no tab, starts no team, and adds no member.
` + confirmedPrompt,
	options.Apply: `      Compare the saved definition with the teams running from it and put each member right: send its prompt again when the prompt moved (the conversation stays), start a member the definition has and no agent is live for, or restart a member whose start (kind or args) changed (the conversation goes). A changed env cannot be applied: env is fixed when the pane is made, so hergy says to stop the team and start it again. A member the definition no longer has cannot be applied either, for the same reason. A member that is working or waiting for permission is left, with the reason, and the rest go on; the command answers 1 while one is left, including with --dry or --dry --json. With no member named, every member with drift is applied. --dry prints each member's way of applying, what changed and the team's own notes (a worker's changed profile takes effect from the next worker), and changes nothing.
` + confirmedPrompt,
	options.Reprompt: `      Re-submit the bootstrap prompt to members that are idle or done, keeping each one's conversation. It starts nothing.
      Both <member> and --all is an error; neither is an error.
      If any selected member is not idle or done, nothing is sent and the whole command errors.
      --dry prints the plan and reads no herdr, so a team that is not running can be planned for: --team names the team, and with no --team the one team in this project's launch record is used.
` + confirmedPrompt,
	options.Focus: `      Move the focus to a member's pane, by short member name or herdr name, so a person can answer it there. It changes nothing else.
`,
	options.Nudge: `      Send a member a short push to carry on, only when it is idle or done: a working or blocked member is left alone, and the whole command errors. The text is headed by who it is from (the member whose pane you type in, else the human); with no text, a short "carry on". --dry prints the message, reads nothing, and shows the human as the sender.
`,
	options.Stop: `      Send esc to a working member, ending its current turn and keeping its conversation. An idle or done member is left alone, and the command still succeeds: there is nothing to stop. Some agents need esc twice: the first only asks them to interrupt and they keep working. hergy watches the member's state after each esc and sends a second esc only while the member is still working, so an agent that stopped on the first is never sent a second (on some harnesses the second esc does something else). A member still working after two escs is reported as not stopped (exit 1), and refresh is named as the next step, the same step a real stop names. A member running in your own pane (HERDR_PANE_ID) is refused. --dry prints what would stop and reads nothing.
`,
	options.Tell: `      Send a member a message headed by who it is from: --from names a member (short member name or herdr name), else the member whose pane you type in, else the human. It does not read the member's state, so the message reaches an agent in any state. This is how a member hands its turn on (r-103d): the tell is recorded for the team's watcher ($XDG_STATE_HOME/hergy/turns/<team>.tells/), which then knows who opened whose turn and who waits for whose answer. hergy tell human (人間, 外側, outside), typed in a member's pane or with --from, sends nothing and needs no text: it marks what the member wrote in its pane as the person's, so the watcher does not ask who it is for when the member stops; from a pane that is no member's it is refused. --dry prints the message, reads nothing, records nothing, and without --from shows the human as the sender.
`,
	options.Hire: `      Start one worker in a tab labelled worker:<team>/<name>. --purpose and --task are required. The purpose must be one the hiring member's workers: names, and the worker's profile comes from that entry; a purpose outside it is refused, naming the purposes the member may hire for. The worker's prompt tells it to write its result in its own pane and stop; hergy's watcher hands it to the member that hired it. A worker whose prompt was not received stays in its pane, and the record still names who hired it.` + startupQuestion + `
      --dry plans from the team definition and reads no herdr, so a team that is not running can be planned for: --team names the team (a team name, or a team definition name the launch record does not know), and with no --team the one team in this project's launch record is used.
` + confirmedPrompt,
	options.Turns: `      Watch one running team's turn (r-103d, r-378e). Members hand the turn on themselves with hergy tell; the watcher hands nothing on of its own. Once a second it takes the tells hergy tell recorded and the signals the members' harnesses wrote (hergy turn-done), and reads herdr. A turn ends when the harness says so (the signal, with the turn's last reply) or, for a member without a signal, when herdr sees the agent go from working to done or idle (then what it wrote in its pane during that turn is read); a wired member's stop waits up to 5 seconds for the signal; a signal for a stop already acted on is dropped. Who opened the turn decides what happens when it ends. A turn the outside opened (the person's hergy tell, the start prompt, hergy's own question) rests: the reply is the person's and nothing is sent. A turn another member opened (its hergy tell, or hergy delivering an answer; a worker's first turn: the member that hired it) that ends with a hergy tell in it was handed on by the member, and nothing more is done; one with hergy tell human in it is the person's: the turn goes to the human, those that waited wait no more, and nothing is asked. A tell that comes while the member works opens its next turn; when the turn then rests and no next turn begins within 10 seconds (a harness that folded the queued message into the running turn), that rested turn is taken as the answer and the member is asked about it. One that ends without a tell is asked about: the watcher sends the member "who is what you wrote for?" with the names it may answer (those waiting for it, then those the definition's turns name), human and nobody. The member's next stop is the answer: one name, and the text that waited is sent to that member with herdr agent prompt (one that is working gets it when it stops, as one message with everything that came meanwhile); human, nobody, no name or several leave the turn with the human, and the state says why. A member that stopped without writing a line while members waited, one hergy could not ask (blocked, or never started), or one whose recipient never started (no working state within 5 seconds, after one retry) also leaves the turn with the human. The same stop is acted on once. blocked is not a stop. Each line in the watcher's pane says how the end was seen: [signal: claude], [herdr saw the stop], or [no signal within 5 seconds; herdr saw the stop]; a delivery decided by an answer says [as answered]. The turn's state is $XDG_STATE_HOME/hergy/turns/<team>.json (holder, who opened whose turn, the question, the box); the tells wait in <team>.tells/ and the signals in <team>.signals/; hergy down removes all three. Its pane shows a three-line band redrawn in place: the team and each member's state (● working, ○ stopped, ◐ blocked), who holds the turn and from whom, who waits for whose answer; the latest handoffs with their time (→ handed, ✓ to the human, ? asked, ⟲ rested, ✗ not delivered or unclear); and what is being asked or boxed. The sentences (one per event, with how the end was seen) go to <team>.log with the time of each line; hergy turns <team> --log prints it, and hergy down leaves it. hergy up starts this in a four-row pane of the team's tab, and it ends when the pane closes.
`,
	options.TurnDone: `      What a member's harness calls when its turn ends (r-378e); a person does not type it. hergy up and hergy hire wire it into each start without writing into the person's own configuration: Claude Code gets --settings with a Stop hook (the person's hooks keep running), Codex gets -c notify=[hergy turn-done ...] (the person's own notify from ~/.codex/config.toml is called afterwards with the same JSON), and a --standalone opencode gets hergy's plugin directory ($XDG_STATE_HOME/hergy/opencode/hergy-turns) added to the plugins of OPENCODE_CONFIG_CONTENT (the profile's own inline config is kept; the person's ~/.config/opencode is read as before, which OPENCODE_CONFIG_DIR would replace), with HERGY_BIN, HERGY_TEAM and HERGY_MEMBER in the pane's environment. A profile that passes --settings or notify itself, whose OPENCODE_CONFIG_CONTENT is not JSON, or that runs opencode without --standalone gets no signal, and hergy up says so; its stops are seen through herdr. The payload is Codex's one JSON argument (last-assistant-message) or, on stdin, Claude Code's Stop hook JSON (last_assistant_message) or the plugin's {"source":"opencode","last_assistant_message":...}. The signal is written for hergy turns to take.
`,
	options.Ls: `      List the visible team definitions and the running team names each one booted.
`,
	options.Show: `      Print the effective team definition and where each value and document came from.
`,
	options.New: `      Write a starter team definition: into the project's .hergy/ when hergy runs inside a git repository, into ~/.config/hergy/ otherwise. The global config.yaml is written too when it is missing.
`,
	options.Validate: `      Check the global config.yaml, every profiles.yaml, and every team definition. Exit 1 if any is broken.
      It prints the size of each role and each team (rules plus fixed template words). The provisional budget is 1,200 characters each. Excess warns but does not change the exit code; invalid settings and legacy YAML do.
`,
	options.Migrate: `      Inspect all legacy role and team YAML files in the project before writing. --global selects shared settings, --dry only prints the plan. Keep adjacent .yaml.bak originals; refuse conflicting destinations. See CHANGELOG for recovery.
`,
	options.Serve: `      Serve the screens over HTTP on 127.0.0.1, print the URL and open it in this machine's default browser, in the foreground, until Ctrl-C. Nothing but this machine can reach it: the listener is bound to 127.0.0.1, a request whose Host is not 127.0.0.1 or localhost is refused, and a write is a POST, PUT or DELETE that must carry the token of this run in the X-Hergy-Token header and come from this page. --port picks the port (default 8787; 0 lets the operating system pick a free one); --no-open prints the URL and opens nothing. A browser that cannot be opened is one line on stderr and serve goes on, with the URL to open by hand. It reads the settings the way the commands do, and it changes nothing but the two profiles.yaml files (a profile's name, kind, args and env, and the profile each member and worker runs), a role document, a team definition's team.yaml and agreement.md, and the projects: list of the global config.yaml. It is the headquarters: it shows the projects config.yaml lists (or, without that key, the project it was started in and the global settings), their teams, roles and profiles with the project each belongs to, the running teams and workspaces, and it starts and stops a listed project's team with the same result as hergy up and hergy down. A stop waits for name release with the same 30000 ms deadline and reports any timeout in its result; if serve runs in a team pane, that pane stays open on timeout. A start runs in a workspace a person picks and reports its progress and a startup question at /api/hq/launches/<id>.
`,
	options.Version: `      Print the commit and time of this hergy. It takes no other flags.
`,
	options.Cheat: `      Print a cheatsheet. --ai prints this text. It reads only config.yaml, for the language, and prints even when that file is broken.
`,
}

// flagMeaning is what each flag means, by flag name. The commands that take it
// are added from the table, so a line here does not list them.
var flagMeaning = map[string]string{
	"global":  "Select shared settings for migrate; otherwise migrate uses the project.",
	"ai":      "Print the agent cheatsheet (this text) instead of the short one. Always English.",
	"as":      "Run the team definition under this team name. The team name is the first half of the herdr name; without it, the team definition name.",
	"dry":     "Print what would happen and change nothing. up and hire never read herdr (a hire --dry can plan for a team that is not running); down, apply, refresh, reprompt, nudge, stop and tell read the live agents and tabs, so they need the team to be running (refresh also refuses your own pane). --dry-run is an alias.",
	"json":    "With --dry, print the plan as JSON; status --json prints its rows as JSON (state and unchanged_minutes) and is the one command that does not need --dry. --json without --dry is an error on every other command.",
	"all":     "Act on every member instead of one <member>; both together is an error.",
	"log":     "turns only: print the watcher's log (each line with its time) and exit without watching. The sentences are in $XDG_STATE_HOME/hergy/turns/<team>.log; the pane shows the band.",
	"timeout": "Startup timeout, 3001..120000 (default 12000).",
	"port":    "The port hergy serve listens on (default 8787; 0 lets the operating system pick a free one).",
	"no-open": "Print the URL and open nothing. Without it, hergy serve opens the URL in this machine's default browser once the socket is open.",
	"team":    "The team, by team name, to act on.",
	"purpose": "The worker's purpose.",
	"task":    "The worker's task text.",
	"from":    "The member a hire is from, or, on tell, who the message is from; a short member name or a herdr name.",
	"lang":    "The language of the command line's output for this run. The global config.yaml can set lang; --lang wins; default en. The WebUI hergy serve opens is Japanese, whatever this says.",
	"dir":     "The project root to resolve against (default: the git root, else the current directory).",
	"help":    "Print the page of the command, with each flag and what it means there. hergy --help lists every command.",
}

// tail is what comes after the Flags: section.
const tail = `Running against herdr:
  up, down, apply, refresh, reprompt and hire, without --dry, must run inside a herdr-managed pane: HERDR_ENV=1 and HERDR_PANE_ID set.
  focus, nudge, stop and tell read and drive herdr from anywhere; the pane you type in says who a message is from, and a member running in it cannot stop itself.
  A team command (down, status, apply, refresh, reprompt, hire, focus, nudge, stop, tell) reads herdr's tabs to find the running team (a tab is a team's when the agents in it are named after the tab's label), and this machine's launch record only says which team definition a found tab came from. The --dry of reprompt, nudge, stop and tell, like hire --dry, does not read herdr: name the team with --team (a team name, or a team definition name the launch record does not know), or use the one team in this project's launch record. up --dry also reads no herdr: it names a team definition. A team name with no hergy tab is an error, whatever the record holds. With no team name, the one running team is used; none or several is an error that names the running teams (see hergy status). down closes the panes named after the team even when its definition cannot be read.
  up and hire fail if a resulting herdr name is already live, and if a member's cwd is not a directory.
  down closes the panes named after the team whether an agent stands in them or not: a member that never started, and one whose agent ended, are closed with the rest, and a pane hergy did not name is a person's own and stays. It closes the team's hired workers too, and hergy's own pane last. It also finds a team whose boot stopped before any agent ran, from the tab labelled with the team's name. One worker of a running team is closed alone with hergy down <worker>, by its short name or its herdr name; only that worker's pane goes, and its tab when that pane was the last one in it, and a name that is both a team's and a worker's is refused. After closing, down lists the git worktrees that came while the team was running (after the worker was hired, for one worker), other than the project root: an agent keeps its pane in the project root and cds into a worktree, so the pane's own directory is not enough to find them. The time comes from the launch record, and a worktree a closed pane stood in is marked. Each worktree comes with its branch, uncommitted changes and commits not in main, and the project's stashes, with the commands that remove them; hergy removes nothing, and temporary documents are not listed.
  refresh refuses a member running in your own pane (HERDR_PANE_ID): stopping it would stop hergy before it could start the agent again. Run it from another pane, or as the human.
  refresh starts a member that is not live again in the pane hergy named after it, when that pane is still at its shell; a pane that is busy is an error, and refresh points at down then up.
  A member with no pane is refused, and hergy apply is named: adding a member to a running team is the apply's road.
  The screens apply the same drift: GET /api/apply names each running team and member that differs from the saved definition, with the way hergy would put it right (resend, start, restart, cannot) and what changed; POST /api/apply takes the members to apply, answers 202 with a start's own state, and the progress and the startup question are read at GET /api/hq/launches/{id}. A member that is working or waiting for permission is left, and the rest go on.
  env is fixed when a pane is made: after editing env, run down then up. refresh keeps the pane and its env. Every pane hergy makes also gets GY_ACTOR=<herdr name>: the name the agent writes into gy. A profile's own GY_ACTOR does not win, and validate warns when a profile writes it.
  The contact guide tells members and workers to hand their result on with hergy tell <name> "<text>" and stop, to mark text for the person with hergy tell human, and that a stop without a tell is asked about by hergy's watcher (hergy turns, started by up). Members and workers do not use herdr agent prompt (the watcher would lose whose turn it is).

Reading the output:
  Most lines are "name: value" (for example "team: demo", or "team definition: demo" for a team definition). The field names are translated by --lang, so a Japanese run prints "チーム: demo".
  The values, the JSON keys and the "next:" line are never translated.
  A line "next: <command>" names the command to type next, with no leading hergy (for example "next: down demo", or "next: status demo" after up --as demo).
  --dry prints the plan; with --json it is an object with these top-level keys:
    up (mode=boot): always: mode, team, startup_timeout_ms, members, turns
    refresh (mode=restart): always: mode, team, startup_timeout_ms, members, turns
    reprompt (mode=reprompt): always: mode, team, startup_timeout_ms, members, turns
    hire (mode=spawn): always: mode, team, startup_timeout_ms, members, turns
    apply: always: herdr, count, teams
    down: always: team, members, workers, leftovers
  apply returns the same shape as GET /api/apply, which the screen's apply sheet reads.
  status --json prints its rows as JSON, the one command that does not need --dry: a team object with members, each {name, state, unchanged_minutes, live, worker}, where unchanged_minutes is the number of minutes a working member's screen has stood (absent when it changed or hergy had not seen it). With no team name it is {"teams": [...]}, one such object each.
  Members in --dry --json have different shapes:
    up/refresh/reprompt/hire members[]: always: name, role, kind, args, env, cwd, prompt, segments, profile; optional: turns, workers, sources
    apply teams[].members[]: always: name, short, role, state, how, changes, can_apply, conversation_lost; optional: why_not
    down members[]: herdr name strings
  For up, refresh, reprompt and hire, optional member keys appear only when set; cwd is an absolute path, and env is the pane's environment, the profile's own variables plus GY_ACTOR. For apply, why_not appears only when the member cannot be applied.
  Exit codes: 0 success; 1 a failure, or validate finding a broken file; 130 interrupted with Ctrl-C.
  validate prints the files it checked and the size of each document.
  Each role and team has a provisional 1,200-character budget. Team counts include fixed template words. A warning leaves exit 0; invalid settings and legacy YAML stop with exit 1.
  To read the text form, add --lang en, or use --dry --json.

Web UI:
  The command for the screens is hergy serve: it answers on 127.0.0.1, prints the URL and opens it in the default browser of this machine; --no-open prints the URL and opens nothing. --port picks the port (default 8787, 0 lets the operating system pick a free one). A browser that cannot be opened is one line on stderr, and serve goes on. It runs in the foreground until Ctrl-C, then waits for the requests in flight before it stops.
  It answers only a request whose Host is 127.0.0.1 or localhost. A write is a POST, PUT or DELETE that must carry the token of this run in the X-Hergy-Token header and come from a page the same server rendered; that page holds the token, so an agent driving the UI reads it from a page first.
  The screens, one line each:
    /             moves to /activity while this project has a team running, else to /teams
    /activity     this project's running teams: one tile per team, with its state, diagram and member/worker counts. Each tile links to the launch definition by its name and place, uses that same definition for its diagram, and opens the apply sheet, or asks for the stop in a confirmation dialog first, which names the panes it closes (the stop sheet then shows the result); waiting and stalled rows select and scroll to their tile. The screen reads itself again every ten seconds. /activity?open=<team>,… selects the named team. Applying selected members has the same result as hergy apply.
    /teams        the team definitions: the list (name, description, place, and the counts of its members and workers, and rules size), a head for each place (the project's, each listed project's, and グローバル) with ＋ <place> にチーム定義を追加 on it (the new row opens at the top of that head, with a name to fill in, and the team.yaml and agreement.md it would write listed; the members' roles and profiles are picked from what either place holds, and the starter roles and profile are written only when the person presses the button for them, and then only the starter names the row's members use), and each row carrying 複製, グローバルにコピー / プロジェクトにコピー and 削除 (a copy into the global place writes the definition's team.yaml and agreement.md alone, and lists the role documents and profiles the global place does not hold; only the ones the person picks are written with it); 削除 asks in the shared dialog, whose body names the files and the folder that go and what stays in the folder, and only its 削除する runs it; so do a role's and a profile's 削除, a new row's 取り消し with input, a team's stop, and leaving a screen or a role edit with unsaved changes; the 状態と操作 column beside a row says which team of that definition is running (with 終了する, and the team's own name when that name is not the definition's), and names the members that definition has and no agent is live for; when no team runs, it starts one: このプロジェクトで起動する for the project's definition and プロジェクトを選んで起動する for a global one, and the sheet it opens takes the team's name (empty for the team definition's name; the same as hergy up --as); a row opens in place with the two tabs トポロジー (team.yaml as a figure: the human and the members as points, the turns as lines; a turn is added or taken away by pressing a point or a line, a card shows what the change would do to the prompt before it is made, a member's card carries 複製, which adds the same member again (its role, profile, turns and worker purposes) under the next free name, and the team.yaml the row would write is shown line by line beside the figure) and 規則 (the team document and its shared roles side by side)
      In the topology editor, selecting a member and opening プロンプト in the attribute panel shows the current topology text under Roster, Contact, Turn recipients, and Adding workers (when present), without rule bodies. The “全文と差分を読む” button opens the recorded body, current text, and their full line diff. No text is folded. Opening the diff shows how many lines were added and removed, and 「最初の差へ」 moves to the first difference in one step; opening the diff does not scroll by itself. Text labels distinguish topology origins from role and team rules in current text and unchanged or added diff lines. Removed lines and the recorded body have no origin metadata and remain unmarked. The four topology sections are drawn as one band from the heading to the end of the section. A heading that comes from the template stands inside the band and carries no origin label. The recorded body shows the operation and time; it excludes the change-reason preamble added by apply or reprompt and is not a receipt. Current text includes unsaved topology edits. A stopped member or missing record shows the reason and current text only. The panel does not save or send prompts. The text is built with the name of a team running from this definition when one is running, and with the definition's name otherwise; the panel also says which name it used. When the running team's name cannot be used, the text falls back to the definition's name and the reason is shown.After a start, a stop, or a change of aliases, the column's text and its note match within the ten-second reading without pressing anything. A worker's preview also includes unsaved edits and the running team's name.
    /teams?open=name|global or /teams?open=name|<current project absolute path> opens only the specified place even without listed projects. Without listed projects, another project or a definition absent from that place shows the existing not-found message. Opening a row writes open=name|place (a project row is open=name|project), and opening that URL again opens the same row. An old URL with no place opens both rows of that name, the project's and the global one.
    /teams/yaml   every team.yaml and agreement.md as text, the project's and the one outside it, with line numbers and validate's problems beside the editor
    /profiles     the profiles: the list (name, kind, place, args, env and who uses it), a row that opens and is written in place, and the kind chips and search above it
    /profiles/assign  the members' and workers' profiles, one card per team definition; a pick is saved into that team.yaml
    /profiles/yaml  both profiles.yaml files as text, line numbers and validate's problems beside the editor
    /profiles/<profile>  the row of that profile, opened
    /roles        the three tabs of the role documents: 一覧 (every role, the people that read it, its rules and character count), 割り当て (each team definition's members and the role each plays) and Markdown (a role document as text with validate's problems beside the editor)
    /roles/assign  each team definition's members and their roles; a pick is saved into that team.yaml
    /roles/yaml  one role document as text, with line numbers and validate's problems beside the editor
    /roles?open=<role>  the list tab with that role's row opened
  A band stands at the top of every screen: the hergy logo with a robot face and pen lettering (a link to /) with the project's folder name, then the four screens, all of them pressable. アクティビティ carries アクティビティ N and 待ち N beside it while this project has a team running: N how many of its teams are running, and how many of them wait for a human or a permission. The screen a person is on is the dark one, and the band shows a dot beside a screen that holds changes no save wrote. A team.yaml, role document or profiles.yaml that cannot be read stays in its list as one row that says why (the same problem line the team screen draws), and the band counts that row. 元に戻す on an unsaved edit goes at once and shows 取り消す in place; 取り消す brings the discarded edit and the unsaved mark back. The figure's 外す of a member, a turn or a worker, the conflict's 元に戻す and the ［プロジェクト］ button's 外す also show 取り消す where the person pressed, and ⌘Z in the figure takes back the newest operation too. The ［プロジェクト］ sheet lists and unlists projects: 外す changes no file of that project, and its 取り消す puts the project back at the position it stood in config.yaml's projects: list, comments and all.
  Walking from one screen to the other with the band carries the changes that were not saved yet, and puts them back when that screen comes round again.
  What the fields write today: a role document, a team definition's team.yaml and its agreement.md, and the two profiles.yaml files (a profile's name, kind, args and env, and the profile each member and worker runs).
  Waiting rows show each member's question or recognized approval block, including its command, without terminal history. Approval blocks require exactly one recognized heading and one consecutive set of numbered choices at the end of the screen; unrecognized boundaries or here-documents direct the person to the terminal. 端末で返答する moves to that member's pane, without sending an answer, and shows progress while waiting. The activity figure shows the selected member's turns and separates the workers below.
  YAML and assignment tabs name 表示するプロジェクト and let the person switch among listed projects; reads and saves use that project. Switching projects or leaving a profile tab asks before discarding unsaved changes. The selected project survives a visit to Activity. An optional team agreement with no sections guides the person to ＋ 節. The screens say 設定の検査 and 改訂番号; the meter line is a provisional guide, not a save/start limit.
  The team definition editor offers a separate 名前を変更 action. Save or revert that definition’s unsaved edits first. The server refuses running or starting teams, an unreadable herdr session, a same-place name collision, invalid names, or changed team/agreement files. Global definitions are checked against the entire connected session, including projects not shown. Names may shadow the other place with an explanation; launch records keep the old name, and the old URL has no redirect. On macOS and Linux, starts use shared file locks and renames use exclusive locks under $XDG_STATE_HOME/hergy/team-locks (or ~/.local/state/hergy/team-locks). Concurrent starts are allowed. Only a lock conflict refuses a start; unavailable locking does not stop it. A rename requires its locks. An unknown launch source refuses a rename only when the definition name matches.
  Member attributes focus the corresponding rules column. It saves the resolved role document with file-hash conflict handling. Shared-file readers are shown; incomplete context disables saving. Rename, duplicate, delete and copy stay on /roles.
  The screens the old WebUI had are gone. /definitions and /agreement, and every path under them, answer 404 with the band on the page and one line saying there is no such screen, as do /team, /running, /rules, /diff, /bump, /figure and /panel. The old files /static/hergy.css and /static/htmx-2.0.4.min.js are gone too. A page that is still there moves instead: /profiles/<name>, and /roles/<role> with /new or /<rule> behind it, go to the list with that profile or role opened.

Resolution:
  A team definition is teams/<definition>/ under the project's .hergy/, or under $XDG_CONFIG_HOME/hergy (or ~/.config/hergy when unset).
  A definition name is looked up local first, then global; a name in both resolves to the project's own.
  The project root is the git root (else the current directory); -C, --dir overrides it.
  Relative and absolute paths, trailing slashes and symbolic-link aliases name the same project. Project paths shown by the CLI and API use the normalized absolute spelling.
  up with no name uses the one team definition of the project; otherwise the candidates are listed and it stops.
  A team command with no team name uses the one running team; none or several stops with the running team names.
  hergy's own state is $XDG_STATE_HOME/hergy (or ~/.local/state/hergy when unset): launches/log.jsonl records up, apply, refresh and reprompt, and launches/blobs/<sha256> holds the prompt texts and rule file contents each line points at. Members are confirmed recipients only. After success or missing receipts, up (including starts from the screens) records the team even when no prompt was received; refresh, reprompt and apply write only with at least one confirmed recipient. Other sending/startup failures and cancellation write none. A launch that cannot be recorded still runs, with one line on stderr.

Team, definition and member names:
  members[].name is a short member name (lead, builder). hergy prefixes the team name, so the herdr name is <team>-<member>.
  --as <team> runs the same team definition under another name: the demo definition's lead is demo-lead, and dev2-lead with --as dev2.
  apply <member>, refresh <member>, reprompt <member>, focus <member>, nudge <member>, stop <member>, tell <member> and hire --from <member> accept the short member name or the herdr name; the short name is preferred.
  The pattern [a-z][a-z0-9_-]{0,24} is checked by new and --as only; what always holds is that <team>-<member> matches [a-z][a-z0-9_-]{0,31}.

Settings files and their keys:
  Unknown keys are an error in every settings file. kind matches [a-z][a-z0-9_-]*; a profile name matches [a-z][a-z0-9_-]{0,31}; a role name matches [a-z][a-z0-9_-]{0,31} too, and the name people read is the role document's title. A role document whose name is not a role name is broken: validate says so, and up of a team that reads it stops.
  env values must be strings (quote numbers); an env name matches [A-Za-z_][A-Za-z0-9_]*.
  members[].name must not contain %{team}. A missing prompt uses the built-in template; a whitespace-only prompt is an error; an unknown %{x} is an error.
  Global config.yaml, at <global>/config.yaml:
    version: 1        required, int; must be 1.
    lang: en|ja       string; the language when --lang is not given; default en.
    projects:         list of directories, optional; the projects hergy serve shows as its headquarters (d-73fc). Each entry is an absolute path or starts with ~/; ~ is expanded when read. Without the key, serve shows the project it was started in and the global settings. With an empty list it shows the global settings alone. A relative path is a broken setting that validate and serve refuse.
  A team definition's team.yaml, at teams/<definition>/team.yaml under .hergy/ or ~/.config/hergy/; cwd is relative to the project root:
    version: 1        required, int; must be 1.
    description: "<text>" string, required; one line saying which case the team is meant for. A definition that writes none, or writes one over more than one line, does not read, and one that still writes the key it replaced is stopped with the sentence that says to write description: instead; hergy validate warns while it is still the starter example hergy new writes. The places a person picks or reads a definition read it (the team list, a definition's screen, the start sheet, hergy show); a team command's header and a running team's tiles do not, and it is never part of a member's prompt or of what hergy apply puts right.
    team: <definition> string, optional; if written it must equal the team definition name (the directory name).
    cwd: <dir>        string; working directory, relative to the project root; default ".".
    prompt: "<text>"  string, optional; the bootstrap prompt template for every member. It must keep %{role_doc} and %{docs}.
    members:          required, nonempty list.
      name: <member>     required, string; short member name matching [a-z][a-z0-9_-]{0,31}.
      role: <role>       required, string; a role name matching [a-z][a-z0-9_-]{0,31}; the name people read is the document's title. Reads roles/<role>.md (the project's first, then ~/.config/hergy/).
      profile: <name>    required, string; a named engine from profiles.yaml. kind, args and env are the profile's: writing one of them on a member is an error that says where it goes.
      cwd: <dir>         string; overrides the team cwd.
      prompt: "<text>"   string; overrides the team prompt. It must keep %{role_doc} and %{docs}.
    workers:          list, optional; which member may hire which worker. Each entry ties one worker to the member that hires it; the worker's two turns (the member to the worker, and the worker back) are made from it and cannot be written in turns:. A member and a purpose written twice is an error. A worker has one purpose: write two purposes as two entries (the same profile may be written twice), and a purpose with a comma or a Japanese comma stops the definition.
      - from: <member>     required, string; the member that hires this worker.
        purpose: <text>    required, string; what the worker is for; one purpose per worker, and one worker per member and purpose.
        profile: <profile> required, string; the engine that worker runs; a profile in profiles.yaml.
    turns:            list, required; who hands the turn to whom, one entry per turn, written "- <from> -> <to>". Each end is a member's name, or human. A turn graph hergy cannot hold stops the definition, the way a file that does not read does: the same sentences stop up, apply, refresh, reprompt, hire and validate, one per line, and down still closes such a team by name. What stops it is a name that is no member and no human, a name written "<kind>:<name>" (no kind but human and a member may stand there yet), a turn to itself, the same turn twice, a member or the human that hands the turn to nobody, and no turn reaching the human. A rule text that still writes "a → b" is compared with turns: and warned about when the two disagree.
    ledger:           list, optional; which member may make which write to gy, one rule per entry, written "<write> by <member>[, <member>...]". <write> is the name of a gy write, the first two words of the why it records: req add, req approve, req done, req revise, criterion satisfy, need add, need close, decide, question add, question close, or "question to human" for a question whose decider is no member of the team. <member> is a members[].name; a worker's name cannot be written (a worker takes no rules). It moves no exit code: hergy validate warns once for each write a writer of the team (GY_ACTOR begins with the team name, or the name the launch record shows) made that the rule gives to other members, and when gy cannot be read it says so in one line instead.
  Where a member's documents are (team.yaml has no key for them):
    roles/<role>.md                 under .hergy/ or ~/.config/hergy/: a member's role document; read first. The project's first, then outside it.
    teams/<definition>/agreement.md the team's own agreement; read next. Optional.
    agreement.yaml at the top of .hergy/ or ~/.config/hergy/ is not read any more (the agreement every team used to share). While one is left, up, apply, refresh, reprompt and hire stop before they build a prompt, naming the file and where the rules go, and validate names it; move the rules into a team's agreement or a role document, then remove the file.
  profiles.yaml, at .hergy/profiles.yaml or ~/.config/hergy/profiles.yaml, next to teams/:
    version: 1        required, int; must be 1.
    profiles:         mapping of profile name to engine.
      <name>:
        kind: <kind>       required, string; herdr's agent kind.
        args: [<arg>...]   list of strings; passed to the agent CLI as is. Quote numbers.
        env: {NAME: value} mapping of string to string; environment for the pane. hergy adds GY_ACTOR itself, so a value written here does not win.
      A name equal to its kind reads as a choice of kind; validate warns on it and says to name the model or the settings.
  A team definition reads the project's profiles.yaml over the one outside it, wherever the definition sits: a profile of the same name resolves to the project's.

Rule documents:
  In the team screen’s rules tab, switch to headings and use a heading’s ⋯ menu to choose another team or role, an insertion position, and whether to include child headings. Drag the grip to another column to move; hold Alt (⌥ on Mac) when dropping to copy, or Shift (⇧ on Mac) when starting the drag to include children. The selected text stays unchanged; a missing blank line at the insertion boundary is supplied. Review the drafts and undo before saving. Moves save the destination first; if saving the source fails, the screen says the text remains in both files. Select a document by its name to read and edit it. The rules tab opens with every document of the definition (the team rules and the role documents its members use) side by side at the standard reading width of about 40 Japanese characters per line in 16px text; the role screen uses the same standard width. Use 並べる to choose again which documents stand together, or すべて表示 to bring them all back. Switching documents preserves unsaved edits; each document keeps its own save target. Use Narrow or Wide in a column header, and Equal to reset the widths. Wide expands the selected document beyond the standard reading width. Apart from a column narrowed on purpose, no column is narrower than the standard reading width, and a wide column grows at twice the rate of the others; the board has its own horizontal scrollbar and an explanation when documents do not fit. At narrower widths, two selected documents are stacked. All four screens use the available width for the header and content, with no width limit and at least 40px of space on each side. Below 420px, each side uses at least 16px to leave more room for the content. At 420px and above, the header stays aligned when switching screens. Narrower screens do not reserve space for a scrollbar, so the right side of the header may move when the page length changes. With no focus in an input, the wheel over it scrolls the page; to read inside an input, press it once and then turn the wheel. Ctrl (⌘ on a Mac) with the wheel and pinch stay with the browser's zoom. Switching the display, re-reading, filtering or changing the theme never moves the page; only an act that names its destination (editing the selected attributes, a waiting row, opening a row from a URL) or one whose result would stand off the screen (copy, a new row, a duplicate) brings that destination into view.

  Choose Light, Dark, or Follow OS at the top right. The preference stays in the same browser for the same host and port, across all four screens and projects. Different hostnames (including localhost versus 127.0.0.1), ports, and browsers do not share it. If browser storage cannot be read or written, the screens follow the OS and explain why. No hergy configuration file is changed.
  roles/<role>.md and teams/<definition>/agreement.md hold complete Markdown text.
  Saved text is inserted unchanged into %{role_doc} and %{docs}; placeholders inside rules are not expanded again.
  The roster includes the short member name, the first-line title, role name, herdr contact and the self marker.
  Arrows and role names in rules are not interpreted. Only team.yaml turns: controls topology validation.
  Each role and team has a provisional 1,200-character budget. A team's total includes rules plus fixed words in team/member prompt templates, excluding placeholders and generated text. Count Unicode code points, normalizing CRLF only for counting. Excess is a warning, never a startup failure.
  The team screen edits the team document and distinct shared roles side by side, with one full-text field per file, heading outlines and block counts. Saving a shared role updates that file for every reader; stale saves are refused by file hash.
  There are no rule IDs, on/off fields or separate notes. Use hergy migrate --dry, then hergy migrate to convert legacy YAML explicitly, or add --global for shared settings. Adjacent .yaml.bak files retain the original bytes, including excluded rules and notes. Old YAML at an active path stops validate and prompt generation. See CHANGELOG for collisions, symlinks, unsupported hard links and recovery.

hire (a member starting a worker):
  The hiring member's workers: must pair --purpose with a profile; the worker takes that profile's kind, args and env, and inherits the hiring member's cwd.
  A --purpose the hiring member's workers: does not name stops the hire, and the sentence lists the purposes that member may hire for. There is no --profile: the profile comes from the workers: entry.
  The worker's name must not match an existing member name or herdr name, and <team>-<worker> must match [a-z][a-z0-9_-]{0,31}.
  With no --from, the one member whose workers: name --purpose hires. No such member is an error, and so are two or more, which list the candidates.

new:
  It writes team.yaml and agreement.md into the team definition, and the global config.yaml (version: 1, lang) when that file is missing.
  The members' roles and profiles are chosen from what the project's .hergy/ or $XDG_CONFIG_HOME/hergy holds: hergy writes no role document and no profile the person did not choose.
  Only when neither place holds a role document does it write roles/lead.md and roles/builder.md; only when neither place defines a profile does it write claude-default into profiles.yaml. Otherwise it names the roles and profiles to write in team.yaml, and the next step is hergy validate.
  It never overwrites: an existing team definition directory stops it and nothing is written (exit 1), and a role document already there is left as it is.

Bootstrap prompt placeholders:
  %{team} %{role} %{name}  the team name, the role, and the member's name (the herdr name, <team>-<member>)
  %{role_doc}  the role document     %{docs}  the agreements     %{roster}  the roster
  %{actor}  the name hergy passes to gy as GY_ACTOR (the herdr name)
  %{herdr}  how to check whether the agent is running inside herdr, reach others, and add a teammate
  %{turns}  who the member hands the turn to, from the definition's turns:
  %{workers}  what the member may hire, with the command (only when workers: names one for it). The old %{spawn} is gone: a template that still writes it is an error that says so.
`
