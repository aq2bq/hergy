// Package cheat holds hergy's cheatsheets: a short one for people, in the
// language in use, and a complete one for an agent. Neither reads a settings
// file nor touches Herdr, so both print even on a machine with broken settings
// and no Herdr.
package cheat

import "github.com/aq2bq/hergy/internal/i18n"

// People is the short cheatsheet, in the language in use. The wording lives in
// the locales; the shape is the first three steps a newcomer types, then a
// short what-you-want-to-command table.
func People() string { return i18n.T("cheat.people") }

// Machine is the agent cheatsheet. It is English whatever --lang says: an agent
// reads it on its own, without the locales, and needs the whole shape, not a
// translation. It reads no settings, so it reflects the shape hergy is built
// with, not the one on this machine.
const Machine = `hergy cheat --ai - the whole of hergy, for an agent that will not read anything else

Commands (hergy <command>; every command accepts --help; the common flags are listed under Flags):
  hergy up [team] [--as team] [--global|--local] [--dry] [--json] [--timeout ms] [-C dir]
      Boot a team. --dry prints the plan (each member's expanded prompt) without touching Herdr; --json, only with --dry, prints that plan as JSON.
  hergy down [team] [--as team] [--global|--local] [--dry] [-C dir]
      Close the live members' panes. --dry prints what would close and closes nothing.
  hergy refresh <seat>|--all [--team team] [--as team] [--global|--local] [--dry] [--json] [-C dir]
      Re-submit the bootstrap prompt to waiting members. Both <seat> and --all is an error; neither is an error.
  hergy hire <name> --profile P --purpose X --task "..." [--from seat] [--team team] [--as team] [--global|--local] [--timeout ms] [--dry] [--json] [-C dir]
      Start one worker in its own tab. --profile, --purpose and --task are required. --from names the seat that hires the worker; it is required when no member has may_spawn, or when more than one does.
  hergy ls [--global|--local] [-C dir]
      List the visible teams and the layer each came from.
  hergy show <team> [--global|--local] [-C dir]
      Print the effective settings and where each value and document came from.
  hergy new <team> [--local] [-C dir]
      Write a starter team's files (global by default; --local for the project). It never overwrites an existing team.
  hergy validate [-C dir]
      Check every settings file the way up reads it and list each broken one. Exit code 1 when any is broken.
  hergy version
      Print the commit and time of this hergy.
  hergy cheat [--ai] [--lang en|ja]
      Print a cheatsheet. --ai prints this text; without it, a short cheatsheet for people. It reads no settings and exits 0.

Flags:
  --ai            Cheat only: print the agent cheatsheet (English, whatever --lang says).
  --as <team>     Boot the settings under this team name instead of the settings name. up/down/refresh/hire.
  --dry           Print what would happen, and touch no Herdr. up/down/refresh/hire.
  --json          With --dry, print the plan as JSON instead of text. up/refresh/hire.
  --global        Use the global layer only.
  --local         Use the local layer only. --global and --local together is an error.
  --timeout <ms>  Startup timeout in milliseconds, 3001..300000 (default 30000). up/hire.
  --team <team>   The team to act on. refresh/hire.
  --profile <name> The named engine for the worker (a profile from profiles.yaml). hire.
  --purpose <text> The purpose line for the worker, checked against the hiring member's may_spawn. hire.
  --task <text>   The task text handed to the worker. hire.
  --from <seat>   The seat hiring the worker. hire.
  --lang <en|ja>  The language for this run. The global config.yaml can set lang; --lang wins; default en.
  -C, --dir <dir> The project root to resolve against (default: the git root, else the current directory).
  -h, --help      Print the usage.

Reading the output:
  Most lines are "name: value" (for example "team: demo", "layer: local", "file: ..."). Nested values are indented two spaces per level.
  A line "next: <command>" names the command to type next.
  --dry prints the plan instead of acting. --json prints that plan as JSON: an object with mode, team, startup_timeout_ms and members.
  Exit codes: 0 success; 1 a failure, or validate finding a broken file; 130 interrupted with Ctrl-C.

Team resolution:
  A team is <layer>/teams/<name>/team.yaml. The local layer is <project>/.hergy; the global layer is $XDG_CONFIG_HOME/hergy (or ~/.config/hergy when unset).
  A name is looked up local first, then global; a name in both resolves to local. With --global or --local, only that layer is looked at.
  The project root is the git root (else the current directory); -C, --dir overrides it.
  With no team name: a single local team is used; otherwise the candidates are listed and hergy stops.

Team name and seat names:
  members[].name in team.yaml is a short seat name (for example lead, worker). hergy prefixes the team name, so the name Herdr sees is <team>-<seat>.
  --as <team> runs the same settings under another team name: demo's lead becomes demo-lead, and dev2-lead with --as dev2.
  refresh <seat> and hire --from <seat> take the short seat name, not the Herdr name.
  The team name must match [a-z][a-z0-9_-]{0,24}, and <team>-<seat> must match [a-z][a-z0-9_-]{0,31}.

Settings files and their keys:
  Global config.yaml, at <global>/config.yaml:
    version: 1        required, int; must be 1.
    lang: en|ja       string; the language when --lang is not given; default en.
  A team's team.yaml, at <layer>/teams/<team>/team.yaml:
    version: 1        required, int; must be 1.
    team: <name>      string, optional; if written it must equal the settings name (the directory name).
    cwd: <dir>        string; working directory, relative to the project root; default ".".
    prompt: "<text>"  string, optional; the bootstrap prompt template for every member.
    docs: [<path>...] list of strings; documents every member reads, joined in order; a directory means its *.md in name order.
    roles: [<dir>...] list of strings; where role files are looked for; default [roles].
    members:          required, nonempty list.
      name: <seat>       required, string; short seat name matching [a-z][a-z0-9_-]{0,31}.
      role: <role>       required, string; reads <role>.md from a roles directory.
      profile: <name>    string; a named engine from profiles.yaml. Write this or kind/args/env inline, not both.
      kind: <kind>       string; Herdr's agent kind (claude, codex, opencode, ...).
      args: [<arg>...]   list of strings; passed to the agent CLI as is. Quote numbers.
      env: {NAME: value} mapping of string to string; environment for the pane.
      cwd: <dir>         string; overrides the team cwd.
      prompt: "<text>"   string; overrides the team prompt.
      may_spawn:         mapping, optional; what this member may hire. Written into the prompt, not enforced.
        purposes: [<purpose>...] list of strings; purposes the member may hire for.
        profiles: [<profile>...] list of strings; each must be a profile in profiles.yaml.
  profiles.yaml, at <layer>/profiles.yaml (a team may name profiles; a team with no profile and no kind is an error):
    version: 1        required, int; must be 1.
    profiles:         mapping of profile name to engine.
      <name>:
        kind: <kind>       required, string; Herdr's agent kind.
        args: [<arg>...]   list of strings; passed to the agent CLI as is.
        env: {NAME: value} mapping of string to string; environment for the pane.
  A local team reads the project's profiles.yaml over the global one: a profile of the same name resolves to the local one, and a global team reads only the global one.

Bootstrap prompt placeholders:
  %{team} %{role} %{name}  the team name, the role, and the member's name (the Herdr name, <team>-<seat>)
  %{role_doc}  the role file     %{docs}  the docs documents     %{roster}  the roster
  %{herdr}  how to reach others over Herdr     %{spawn}  what the member may hire, with the command (only when may_spawn is set)
`
