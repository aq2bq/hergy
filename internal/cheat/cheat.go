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

A team definition is a directory, <layer>/teams/<definition>/team.yaml. A running
team has a team name: the team definition name by default, or another name given
to hergy up --as. Commands that read or write a team definition take a team
definition name (up, new, show); commands that act on a running team take a team
name (down, status, refresh, reprompt, hire). A team command finds the definition
from the team's herdr tab label, "team:<team>@<definition>" (an older tab,
"team:<team>", means the definition name is the team name).

Commands (hergy <command> --help prints one command's page, and every command but version takes it; a bare -- ends the flags, and --flag=value is accepted):
`

// behavior is what each command does, under its command line. Every line starts
// with six spaces.
var behavior = map[options.Kind]string{
	options.Up: `      Boot the team definition in a new tab. --as names the team; without it the team name is the definition name. --dry prints the plan and submits nothing; --json, only with --dry, prints it as JSON.
`,
	options.Down: `      Close the panes of the running team's live members and of the workers it hired. --dry reads herdr's live agents and tabs, prints what would close, and closes nothing.
`,
	options.Status: `      With a team name: one line per member and per worker, its live agent_status (working/idle/done/blocked), or "not present" when the member is not live; a worker carries "(worker)". Then a judgement line: "waiting for a human: <names>" when any is blocked, else "nobody is working" when none is working or blocked. With no team name: every running team, each with its team definition name, its member and worker counts, and the same judgement. It only reads herdr, and runs outside a herdr-managed pane too.
`,
	options.Refresh: `      Restart each selected member. A live member: stop its process group, wait for herdr to forget the name, then start it again in the same pane and submit the bootstrap prompt. A member that is not live: start it again in the pane hergy named after it, when that pane is still at its shell. Either way the conversation starts empty. The settings are read again first, so an edited profile, kind, args or prompt takes effect; env does not (it is fixed when the pane is made).
      Both <member> and --all is an error; neither is an error.
      A member running in your own pane (HERDR_PANE_ID) cannot refresh itself: stopping it would stop hergy first. That is an error, and nothing is stopped.
      If any selected member is not idle or done, is your own pane, or (when not live) has no pane at its shell, nothing is changed and the whole command errors. A pane that is gone or busy points at hergy down then hergy up.
`,
	options.Reprompt: `      Re-submit the bootstrap prompt to members that are idle or done, keeping each one's conversation. It starts nothing.
      Both <member> and --all is an error; neither is an error.
      If any selected member is not idle or done, nothing is sent and the whole command errors.
`,
	options.Hire: `      Start one worker in a tab labelled worker:<team>/<name>. --purpose and --task are required. --profile is optional: without it, the profile the hiring member's may_spawn pairs with --purpose is used.
`,
	options.Ls: `      List the visible team definitions and the layer each came from, and the running team names each one booted.
`,
	options.Show: `      Print the effective team definition and where each value and document came from.
`,
	options.New: `      Write a starter team definition: into the global layer by default, into the project's .hergy/ with --local. The global config.yaml is written too when it is missing, with --local as well.
`,
	options.Validate: `      Check the global config.yaml, both layers' profiles.yaml, and every team definition in both layers. Exit 1 if any is broken.
      It then counts what the rules carry, over the rule files the members of every team definition read (a file two definitions share counts once): how many rules there are, how many say where they came from (incident, instruction or derived) and how many do not, how many carry a reason and how many do not, and how many have a report field or a test and how many do not. It then lists the id of every rule with no origin, by file. That count is not a break: only a broken file makes the exit 1.
`,
	options.Serve: `      Serve the screens over HTTP on 127.0.0.1 and print the URL, in the foreground, until Ctrl-C. Nothing but this machine can reach it: the listener is bound to 127.0.0.1, a request whose Host is not 127.0.0.1 or localhost is refused, and a write is a POST that must carry the token of this run and come from this page. --port picks the port (default 8787; 0 lets the operating system pick a free one). It reads the settings and herdr's state the way the commands do; it changes nothing but the files the screens edit.
`,
	options.Version: `      Print the commit and time of this hergy. It takes no other flags.
`,
	options.Cheat: `      Print a cheatsheet. --ai prints this text. It reads only config.yaml, for the language, and prints even when that file is broken.
`,
}

// flagMeaning is what each flag means, by flag name. The commands that take it
// are added from the table, so a line here does not list them.
var flagMeaning = map[string]string{
	"ai":      "Print the agent cheatsheet (this text) instead of the short one. Always English.",
	"as":      "Run the team definition under this team name. The team name is the first half of the herdr name; without it, the team definition name.",
	"dry":     "Print what would happen and change nothing. up, reprompt and hire never read herdr; down and refresh read the live agents and tabs (refresh also refuses your own pane). --dry-run is an alias.",
	"json":    "With --dry, print the plan as JSON. --json without --dry is an error.",
	"all":     "Act on every member instead of one <member>; both together is an error.",
	"global":  "Use the global layer only.",
	"local":   "Use the local layer only; --global and --local together is an error. For new, the layer to write: --local writes to the project, and without it new writes to the global layer.",
	"timeout": "Startup timeout, 3001..300000 (default 30000).",
	"port":    "The port hergy serve listens on (default 8787; 0 lets the operating system pick a free one).",
	"team":    "The team, by team name, to act on.",
	"profile": "The worker's named engine, from profiles.yaml. Optional: without it, the profile may_spawn pairs with --purpose is used.",
	"purpose": "The worker's purpose.",
	"task":    "The worker's task text.",
	"from":    "The member hiring the worker; a short member name or a herdr name.",
	"lang":    "The language for this run. The global config.yaml can set lang; --lang wins; default en.",
	"dir":     "The project root to resolve against (default: the git root, else the current directory).",
	"help":    "Print the page of the command, with each flag and what it means there. hergy --help lists every command.",
}

// tail is what comes after the Flags: section.
const tail = `Running against herdr:
  up, down, refresh, reprompt and hire, without --dry, must run inside a herdr-managed pane: HERDR_ENV=1 and HERDR_PANE_ID set.
  A team command (down, status, refresh, reprompt, hire) reads herdr's tabs to find the team definition behind the team name; a team name with no tab is an error. With no team name, the one running team is used; none or several is an error that names the running teams (see hergy status).
  up and hire fail if a resulting herdr name is already live, and if a member's cwd is not a directory.
  down skips a member that is not live, closes the team's hired workers too, and closes hergy's own pane last.
  refresh refuses a member running in your own pane (HERDR_PANE_ID): stopping it would stop hergy before it could start the agent again. Run it from another pane, or as the human.
  refresh starts a member that is not live again in the pane hergy named after it, when that pane is still at its shell; a pane that is gone or busy is an error, and refresh points at down then up.
  env is fixed when a pane is made: after editing env, run down then up. refresh keeps the pane and its env.

Reading the output:
  Most lines are "name: value" (for example "team: demo", or "team definition: demo" for a team definition). The field names are translated by --lang, so a Japanese run prints "チーム: demo".
  The values, the JSON keys and the "next:" line are never translated.
  A line "next: <command>" names the command to type next, with no leading hergy (for example "next: down demo", or "next: status demo" after up --as demo).
  --dry prints the plan; with --json it is an object: mode (up boot, refresh restart, reprompt reprompt, hire spawn), team, startup_timeout_ms, members.
  A JSON member always has name, role, kind, args, cwd (an absolute path) and prompt; profile, env, may_spawn and sources appear only when set.
  Exit codes: 0 success; 1 a failure, or validate finding a broken file; 130 interrupted with Ctrl-C.
  validate prints the files it checked, then counts what the rules carry (the origin, the reason and the check of every rule a member reads, with the ids of the rules that have no origin). That count never changes the exit code.
  To read the text form, add --lang en, or use --dry --json.

Web UI:
  The command for the screens is hergy serve: it answers on 127.0.0.1 and prints the URL; --port picks the port (default 8787, 0 lets the operating system pick a free one). It runs in the foreground until Ctrl-C, then waits for the requests in flight before it stops.
  It answers only a request whose Host is 127.0.0.1 or localhost. A write is a POST that must carry the token of this run and come from a page the same server rendered; that page holds it in its forms, so an agent driving the UI reads the token from a page first.
  The screens, one line each:
    /       the home: what moved since the last launch, and the rules with no check, each rule opening in the rules screen
    /teams  the team list: each definition's members, the rules with no check, and the last launch with its hash
    /rules  the rules screen: the rule files the team definitions of a layer read, one rule at a time in the edit panel
    /diff   the prompt diff: per definition and member, the prompt the last launch handed over against the prompt the files build now, line by line
    /bump   the version screen: the documents whose content moved since the last launch, each with the version it holds, the version a bump writes (n → n+1), and the command that hands the change over
  Opening the URL lands on the home, which is the way into the daily flow; every screen reads the same files the commands read.

Resolution:
  A team definition is <layer>/teams/<definition>/team.yaml. The local layer is <project>/.hergy; the global layer is $XDG_CONFIG_HOME/hergy (or ~/.config/hergy when unset).
  A definition name is looked up local first, then global; a name in both resolves to local. --global or --local looks at that layer only.
  The project root is the git root (else the current directory); -C, --dir overrides it.
  up with no name uses one local team definition; --global uses one global one; with neither flag, a lone global definition is not chosen; otherwise the candidates are listed and it stops.
  A team command with no team name uses the one running team; none or several stops with the running team names.
  hergy's own state is $XDG_STATE_HOME/hergy (or ~/.local/state/hergy when unset): launches/log.jsonl is one line per launch (up, refresh or reprompt, written after it succeeds), and launches/blobs/<sha256> is the prompt texts and rule file contents that line points at. A launch that cannot be recorded still runs, with one line on stderr.

Team, definition and member names:
  members[].name is a short member name (lead, builder). hergy prefixes the team name, so the herdr name is <team>-<member>.
  --as <team> runs the same team definition under another name: the demo definition's lead is demo-lead, and dev2-lead with --as dev2.
  refresh <member>, reprompt <member> and hire --from <member> accept the short member name or the herdr name; the short name is preferred.
  The pattern [a-z][a-z0-9_-]{0,24} is checked by new and --as only; what always holds is that <team>-<member> matches [a-z][a-z0-9_-]{0,31}.

Settings files and their keys:
  Unknown keys are an error in every settings file. kind matches [a-z][a-z0-9_-]*; a profile name matches [a-z][a-z0-9_-]{0,31}.
  env values must be strings (quote numbers); an env name matches [A-Za-z_][A-Za-z0-9_]*.
  members[].name must not contain %{team}. A missing prompt uses the built-in template; a whitespace-only prompt is an error; an unknown %{x} is an error.
  Global config.yaml, at <global>/config.yaml:
    version: 1        required, int; must be 1.
    lang: en|ja       string; the language when --lang is not given; default en.
  A team definition's team.yaml, at <layer>/teams/<definition>/team.yaml. docs and roles paths are relative to this directory; cwd is relative to the project root:
    version: 1        required, int; must be 1.
    team: <definition> string, optional; if written it must equal the team definition name (the directory name).
    cwd: <dir>        string; working directory, relative to the project root; default ".".
    prompt: "<text>"  string, optional; the bootstrap prompt template for every member.
    docs: [<path>...] list of strings, or one string; documents every member reads, joined in order; a directory means its *.yaml in name order; a path that is not there is an error. A rule file is YAML (see below).
    roles: [<dir>...] list of strings, or one string; where role files are looked for; default [roles]; <role>.yaml comes from the first directory that has it.
    members:          required, nonempty list.
      name: <member>     required, string; short member name matching [a-z][a-z0-9_-]{0,31}.
      role: <role>       required, string; reads <role>.yaml from a roles directory.
      profile: <name>    string; a named engine from profiles.yaml. Write this or kind/args/env inline, not both.
      kind: <kind>       string; herdr's agent kind (claude, codex, opencode, ...).
      args: [<arg>...]   list of strings; passed to the agent CLI as is. Quote numbers.
      env: {NAME: value} mapping of string to string; environment for the pane.
      cwd: <dir>         string; overrides the team cwd.
      prompt: "<text>"   string; overrides the team prompt.
      may_spawn:         mapping, optional; what this member may hire. An empty value is an error.
        workers:           required, nonempty list of purpose-to-profile pairs.
          - purpose: <text>    required, string; what a hired worker is for; one worker per purpose.
            profile: <profile> required, string; the engine that worker runs; a profile in profiles.yaml.
  profiles.yaml, at <layer>/profiles.yaml, next to teams/ in the layer root:
    version: 1        required, int; must be 1.
    profiles:         mapping of profile name to engine.
      <name>:
        kind: <kind>       required, string; herdr's agent kind.
        args: [<arg>...]   list of strings; passed to the agent CLI as is.
        env: {NAME: value} mapping of string to string; environment for the pane.
  A local team definition reads the project's profiles.yaml over the global one: a profile of the same name resolves to the local one, and a global definition reads only the global one.

Rule files (what docs and roles point at):
  Every document a member reads is one rule file: a team's agreements (protocol.yaml) and a role file (roles/<role>.yaml). hergy up renders each file to Markdown and puts it where %{role_doc} and %{docs} were. Markdown is not read: a .md file, a directory with only .md, or a role with only <role>.md stops with the way to move it over.
  version: 1        required, int; must be 1.
  title: <text>     string, optional; rendered as "# <text>".
  revision: <int>   int, optional; for the people reading the file; never sent.
  sections:         required, nonempty list; sent in the written order.
    heading: <text> string, optional; a section with no heading continues the one before it.
    level: 2..4     int, optional; how deep the heading is; not allowed without a heading; default 2.
    intro: <text>   string, optional; a paragraph before the rules; not a rule.
    style: paragraphs|bullets|numbered|ids   how the rules are laid out; default paragraphs.
    rules:          list; each rule is an entry:
      id: <id>      required, string; [A-Z][A-Z0-9]*(-[A-Z0-9]+)* (C1, P-TURN-1); unique among the documents one member reads.
      text: <text>  required, string; what the member reads, in Markdown.
      why: <text>   string, optional; one space and the reason follow the text, unless send_why is false.
      send_why: false   boolean, optional; keeps the reason in the file and out of the prompt.
      note: <text>  string, optional; for whoever maintains the file; never sent, and not a reason.
      origin:       list, optional; where the rule came from:
        kind: incident|instruction|derived   required; derived needs refs, the rule ids it came from.
        date: YYYY-MM-DD   string, optional; quoted or not.
        refs: [<ref>...]   optional; for incident and instruction an outside reference (an issue, a ledger id, a URL).
        note: <text>  string, optional.
      report: <name>.<field>   string, optional; the report field that shows the rule was kept; never sent. The report may be written in another document the same member reads.
      tests: [<id>...]   list, optional; the tests that check the rule; never sent.
  reports:          mapping, optional; the shape of a report, rendered at the end:
    <name>:
      where: <text>    string, optional; where the report is written.
      fields:          mapping of field name to what it holds.

hire (a member starting a worker):
  --profile names a profile in profiles.yaml; the worker takes its kind, args and env, and inherits the hiring member's cwd.
  Without --profile, may_spawn must pair --purpose with a profile, and that profile is used; a purpose that names no pair is an error that lists the purposes may_spawn allows.
  The worker's name must not match an existing member name or herdr name, and <team>-<worker> must match [a-z][a-z0-9_-]{0,31}.
  With no --from, --from is required unless exactly one member has may_spawn.
  A --purpose or --profile outside the hiring member's may_spawn does not stop the hire: a note goes to stderr and it continues; a member with no may_spawn gives the same note.

new:
  It writes team.yaml, protocol.yaml, roles/lead.yaml and roles/builder.yaml, and the global config.yaml (version: 1, lang) when that file is missing.
  It never overwrites: an existing team definition directory stops it and nothing is written (exit 1).

Bootstrap prompt placeholders:
  %{team} %{role} %{name}  the team name, the role, and the member's name (the herdr name, <team>-<member>)
  %{role_doc}  the role file     %{docs}  the docs documents     %{roster}  the roster
  %{herdr}  how to reach others over herdr, and how to add a teammate     %{spawn}  what the member may hire, with the command (only when may_spawn is set)
`
