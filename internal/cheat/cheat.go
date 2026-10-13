// Package cheat holds hergy's cheatsheets: a short one for people, in the
// language in use, and a complete one for an agent. Neither touches Herdr, and
// neither reads the team settings, so both print on a machine with broken
// settings and no Herdr. The short one's language comes from --lang and the
// global config.yaml, which the caller resolves before it asks for the text.
package cheat

import "github.com/aq2bq/hergy/internal/i18n"

// People is the short cheatsheet, in the language in use. The wording lives in
// the locales; the shape is the first three steps a newcomer types, then a
// short what-you-want-to-command table.
func People() string { return i18n.T("cheat.people") }

// Machine is the agent cheatsheet. It is English whatever --lang says: an agent
// reads it on its own, without the locales, and needs the whole shape, not a
// translation. It is a constant, so it reflects the shape hergy is built with,
// not the one on this machine.
const Machine = `hergy cheat --ai - the whole of hergy, for an agent that will not read anything else

Commands (every command but version prints --help; a bare -- ends the flags, and --flag=value is accepted):
  hergy up [team] [--as team] [--global|--local] [--dry] [--json] [--timeout ms] [-C dir]
      Boot a team in a new tab. --dry prints the plan and submits nothing; --json, only with --dry, prints it as JSON.
  hergy down [team] [--as team] [--global|--local] [--dry] [-C dir]
      Close the panes of the team's live members and of the workers it hired. --dry reads herdr's live agents and tabs, prints what would close, and closes nothing.
  hergy refresh <seat>|--all [--team team] [--as team] [--global|--local] [--dry] [--json] [--timeout ms] [-C dir]
      Restart each selected member: stop its process group, wait for herdr to forget the name, then start it again in the same pane and submit the bootstrap prompt. The conversation starts empty. The settings are read again first, so an edited profile, kind, args or prompt takes effect; env does not (it is fixed when the pane is made).
      Both <seat> and --all is an error; neither is an error.
      If any selected member is not idle or done, nothing is stopped and the whole command errors.
  hergy reprompt <seat>|--all [--team team] [--as team] [--global|--local] [--dry] [--json] [-C dir]
      Re-submit the bootstrap prompt to members that are idle or done, keeping each one's conversation. It starts nothing.
      Both <seat> and --all is an error; neither is an error.
      If any selected member is not idle or done, nothing is sent and the whole command errors.
  hergy hire <name> --profile P --purpose X --task "..." [--from seat] [--team team] [--as team] [--global|--local] [--timeout ms] [--dry] [--json] [-C dir]
      Start one worker in a tab labelled worker:<team>/<name>. --profile, --purpose and --task are required.
  hergy ls [--global|--local] [-C dir]
      List the visible teams and the layer each came from.
  hergy show <team> [--global|--local] [-C dir]
      Print the effective settings and where each value and document came from.
  hergy new <team> [--local] [-C dir]
      Write a starter team's files (global by default; --local for the project).
  hergy validate [-C dir]
      Check the global config.yaml, both layers' profiles.yaml, and every team in both layers. Exit 1 if any is broken.
  hergy version
      Print the commit and time of this hergy. It takes no other flags.
  hergy cheat [--ai] [--lang en|ja]
      Print a cheatsheet. --ai prints this text. It reads only config.yaml, for the language, and prints even when that file is broken.

Flags:
  --ai            Cheat only: print the agent cheatsheet (always English).
  --as <team>     Run the settings under this team name instead of the settings name. up/down/refresh/reprompt/hire.
  --dry           Print what would happen and change nothing. up/refresh/reprompt/hire never read herdr; down reads the live agents and tabs but closes none.
  --json          With --dry, print the plan as JSON. --json without --dry is an error. up/refresh/reprompt/hire.
  --global        Use the global layer only.
  --local         Use the local layer only. --global and --local together is an error.
  --timeout <ms>  Startup timeout, 3001..300000 (default 30000). up/refresh/hire.
  --team <team>   The team to act on. refresh/reprompt/hire.
  --profile <name> The worker's named engine, from profiles.yaml. hire.
  --purpose <text> The worker's purpose. hire.
  --task <text>   The worker's task text. hire.
  --from <seat>   The member hiring the worker; a short seat or a herdr name. hire.
  --lang <en|ja>  The language for this run. The global config.yaml can set lang; --lang wins; default en.
  -C, --dir <dir> The project root to resolve against (default: the git root, else the current directory).
  -h, --help      Print the usage. --dry-run is an alias of --dry.

Running against herdr:
  up, down, refresh, reprompt and hire, without --dry, must run inside a herdr-managed pane: HERDR_ENV=1 and HERDR_PANE_ID set.
  up and hire fail if a resulting herdr name is already live, and if a member's cwd is not a directory.
  down skips a member that is not live, closes the team's hired workers too, and closes hergy's own pane last.
  env is fixed when a pane is made: after editing env, run down then up. refresh keeps the pane and its env.

Reading the output:
  Most lines are "name: value" (for example "team: demo"). The field names are translated by --lang, so a Japanese run prints "チーム: demo".
  The values, the JSON keys and the "next:" line are never translated.
  A line "next: <command>" names the command to type next, with no leading hergy (for example "next: up demo --dry").
  --dry prints the plan; with --json it is an object: mode (up boot, refresh restart, reprompt reprompt, hire spawn), team, startup_timeout_ms, members.
  A JSON member always has name, role, kind, args, cwd (an absolute path) and prompt; profile, env, may_spawn and sources appear only when set.
  Exit codes: 0 success; 1 a failure, or validate finding a broken file; 130 interrupted with Ctrl-C.
  To read the text form, add --lang en, or use --dry --json.

Team resolution:
  A team is <layer>/teams/<name>/team.yaml. The local layer is <project>/.hergy; the global layer is $XDG_CONFIG_HOME/hergy (or ~/.config/hergy when unset).
  A name is looked up local first, then global; a name in both resolves to local. --global or --local looks at that layer only.
  The project root is the git root (else the current directory); -C, --dir overrides it.
  With no name, one local team is used; --global uses one global team; with neither flag, a lone global team is not chosen; otherwise the candidates are listed and it stops.

Team name and seat names:
  members[].name is a short seat name (lead, worker). hergy prefixes the team name, so the herdr name is <team>-<seat>.
  --as <team> runs the same settings under another name: demo's lead is demo-lead, and dev2-lead with --as dev2.
  refresh <seat>, reprompt <seat> and hire --from <seat> accept the short seat or the herdr name; the short name is preferred.
  The pattern [a-z][a-z0-9_-]{0,24} is checked by new and --as only; what always holds is that <team>-<seat> matches [a-z][a-z0-9_-]{0,31}.

Settings files and their keys:
  Unknown keys are an error in every settings file. kind matches [a-z][a-z0-9_-]*; a profile name matches [a-z][a-z0-9_-]{0,31}.
  env values must be strings (quote numbers); an env name matches [A-Za-z_][A-Za-z0-9_]*.
  members[].name must not contain %{team}. A missing prompt uses the built-in template; a whitespace-only prompt is an error; an unknown %{x} is an error.
  Global config.yaml, at <global>/config.yaml:
    version: 1        required, int; must be 1.
    lang: en|ja       string; the language when --lang is not given; default en.
  A team's team.yaml, at <layer>/teams/<team>/team.yaml. docs and roles paths are relative to this directory; cwd is relative to the project root:
    version: 1        required, int; must be 1.
    team: <name>      string, optional; if written it must equal the settings name (the directory name).
    cwd: <dir>        string; working directory, relative to the project root; default ".".
    prompt: "<text>"  string, optional; the bootstrap prompt template for every member.
    docs: [<path>...] list of strings, or one string; documents every member reads, joined in order; a directory means its *.md in name order; a path that is not there is an error.
    roles: [<dir>...] list of strings, or one string; where role files are looked for; default [roles]; <role>.md comes from the first directory that has it.
    members:          required, nonempty list.
      name: <seat>       required, string; short seat name matching [a-z][a-z0-9_-]{0,31}.
      role: <role>       required, string; reads <role>.md from a roles directory.
      profile: <name>    string; a named engine from profiles.yaml. Write this or kind/args/env inline, not both.
      kind: <kind>       string; herdr's agent kind (claude, codex, opencode, ...).
      args: [<arg>...]   list of strings; passed to the agent CLI as is. Quote numbers.
      env: {NAME: value} mapping of string to string; environment for the pane.
      cwd: <dir>         string; overrides the team cwd.
      prompt: "<text>"   string; overrides the team prompt.
      may_spawn:         mapping, optional; what this member may hire. An empty value is an error.
        purposes: [<purpose>...] list of strings; purposes the member may hire for.
        profiles: [<profile>...] list of strings; each must be a profile in profiles.yaml.
  profiles.yaml, at <layer>/profiles.yaml, next to teams/ in the layer root:
    version: 1        required, int; must be 1.
    profiles:         mapping of profile name to engine.
      <name>:
        kind: <kind>       required, string; herdr's agent kind.
        args: [<arg>...]   list of strings; passed to the agent CLI as is.
        env: {NAME: value} mapping of string to string; environment for the pane.
  A local team reads the project's profiles.yaml over the global one: a profile of the same name resolves to the local one, and a global team reads only the global one.

hire (a member starting a worker):
  --profile must name a profile in profiles.yaml; the worker takes its kind, args and env, and inherits the hiring member's cwd.
  The worker's name must not match an existing seat or herdr name, and <team>-<worker> must match [a-z][a-z0-9_-]{0,31}.
  With no --from, --from is required unless exactly one member has may_spawn.
  --purpose or --profile outside the hiring member's may_spawn does not stop the hire: a note goes to stderr and it continues; a member with no may_spawn gives the same note.

new:
  It writes team.yaml, protocol.md, roles/lead.md and roles/worker.md, and the global config.yaml (version: 1, lang) when that file is missing.
  It never overwrites: an existing team directory stops it and nothing is written (exit 1).

Bootstrap prompt placeholders:
  %{team} %{role} %{name}  the team name, the role, and the member's name (the herdr name, <team>-<seat>)
  %{role_doc}  the role file     %{docs}  the docs documents     %{roster}  the roster
  %{herdr}  how to reach others over herdr     %{spawn}  what the member may hire, with the command (only when may_spawn is set)
`
