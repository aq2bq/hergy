# Changelog

## Unreleased

### Added

- **`hergy serve`: the settings in a browser**: `hergy serve [--port N]` (default
  8787; `0` lets the operating system pick a free one) serves the web UI on
  127.0.0.1 and prints the URL. The team list answers `/teams`: one row per
  team definition in the layer you pick (local or global, one toggle in the top
  bar), with the number of members `team.yaml` writes and how many of the rules
  those members read have no check. A definition that cannot be read keeps its
  row and carries why, in the one line `hergy validate` writes; the other
  definitions keep rendering. The frame marks the global layer, and the right
  panel widens to the window without scrolling the page. It answers 127.0.0.1
  only, refuses a request whose `Host` is not its own, takes a write only as a
  POST that carries the token of the running process, and sets
  `Content-Security-Policy: default-src 'self'`. HTMX 2.0.4 (0BSD) is embedded
  with its license.

- **Every launch leaves a record**: `hergy up`, `hergy refresh` and `hergy
  reprompt` write one line per launch to
  `$XDG_STATE_HOME/hergy/launches/log.jsonl` (or
  `~/.local/state/hergy/launches` when unset): when it ran, which hergy ran it,
  the project, the layer, the team definition, the team name, the hash of the
  team as it was launched, the members that were handed a prompt, and the rule
  files those prompts were built from. The texts themselves — each prompt, and
  each rule file — go to `launches/blobs/<sha256>`, one file per text, so a
  launch that changes one document adds one file. No environment value is
  written, in the record or in a blob: the values count toward the team's hash
  only. The hash covers the whole team definition as it was launched, so it does
  not change when a profile no member reads changes, and does change when a
  profile a member reads changes. A `--dry` run and a launch that fails record
  nothing, `hire` records nothing (a worker is not a member of the team
  definition), and a record that cannot be written prints one line on stderr and
  leaves the command a success.

- **The launch record's screens, and the team list's last-launch column**: `hergy
  serve` now opens a Home screen, at `/` (the team list moved to `/teams`) — the
  steps of the daily flow, then what moved since the last launch (per document,
  the rules it added, dropped and changed by id, and whether its version has
  moved past the one the launch read, which means the next `hergy up` hands the
  change over) and the rules with no check, each of which opens in the rules
  screen's edit panel. It also opens a Prompt diff screen, at `/diff`: per team
  definition and member, the prompt the last launch handed over against the
  prompt the files build now, as a line diff (members whose prompt did not change
  are folded away). Both screens read the record and the files through the app,
  so no comparison happens in the page. The team list carries one more column:
  when the definition was last launched from this machine, the head of the hash
  that launch wrote, and a mark when the files build a different hash now. On a
  machine that has never launched a team, the home and the diff say so and name
  the command instead; the rules with no check still show, because they are read
  from the files. A record that cannot be read is one line on the screen, and the
  rest of it still renders.

### Changed

- **The screens have their own paths, and the home is the way in**: `hergy serve`
  opens on the home at `/`, and the team list answers `/teams`. The home is the
  first thing a person sees, because that is where the daily flow starts
  (d-ff49). A link from the old address (`/` for the team list) now lands on the
  home.

- **Build with Go 1.26.9**: `go.mod` names the toolchain, so a build downloads
  and uses it. The toolchain this repository was built with before it (1.26.7)
  carries fixed vulnerabilities in `net/http` and `html/template`, which
  `govulncheck` reports as soon as hergy serves or renders anything: a machine
  whose Go is older now gets 1.26.9 on the first `go build`.

## 0.4.0 - 2026-10-15

### Added

- **A skill for the rules of a team**: `skills/hergy-rules`, installed by
  `npx skills add aq2bq/hergy` next to the hergy skill. It asks one thing at a
  time and writes the YAML for the person: a new team, an existing team, and
  moving rules out of Markdown. It does not ask anyone to remember an origin:
  moving rules leaves the origins empty, and later the skill builds a candidate
  for each from the git history and the gy ledger and asks only whether that
  candidate is right.

### Changed

- **`hergy validate` ends with a count of what the rules carry**: after the
  per-file lines, it counts the rules of every file the members of the team
  definitions read (a file two definitions share counts once): the rule count,
  how many say where they came from (incident, instruction or derived) and how
  many do not, how many carry a reason (whether or not the file sends it) and
  how many do not, and how many have a report field or a test and how many do
  not. Then it lists the id of every rule that says where it came from nowhere,
  by file. That count is not a break: only a broken file makes the exit 1. It is
  where a team sees which of its rules came from nothing, before they harden
  into something nobody dares to touch. The Japanese output uses the words the
  product owner chose for it: インシデント for an incident, and 検証方法 for a check.

- **A team's agreements and role files are YAML, not Markdown**: `protocol.md`
  becomes `protocol.yaml`, and `roles/<role>.md` becomes `roles/<role>.yaml`.
  A rule file holds `sections`, each with the rules under it, and each rule has
  an `id`, its `text`, and, when the file gives them, `why` (whether it is sent
  is chosen per rule with `send_why`), `origin` (`incident`, `instruction` or
  `derived`), `note`, `report` and `tests`. A document may also write the shape
  of a report under `reports`. `hergy up` renders each file to Markdown and
  puts it where `%{role_doc}` and `%{docs}` were; `hergy new` writes the
  starter team in the new form. Markdown is no longer read: a `docs` entry
  that points at a `.md` file, a directory that holds only `.md`, or a role
  that has only `<role>.md` stops with the way to move it over. Before:

  ```markdown
  - C1: Work along the gy-loop skill, so that work can resume from gy when a conversation ends.
  <!-- why: Sessions end. An agreement that lives only in a conversation is lost. -->
  ```

  After:

  ```yaml
  version: 1
  sections:
    - heading: Team agreements
      style: ids
      rules:
        - id: C1
          text: Work along the gy-loop skill, so that work can resume from gy when a conversation ends.
          why: Sessions end. An agreement that lives only in a conversation is lost.
          send_why: false
  ```

  Move a team over with the hergy-rules skill: the reasons, the
  origin and the way each rule is checked cannot be read out of the Markdown,
  so the move is a conversation, not a conversion.

- **CI asks more of a change**: `go run ./tools/check` runs every gate CI runs
  (formatting, `go mod tidy`, `go fix`, golangci-lint, the direction of
  imports, `go test -race`, coverage, dead code, govulncheck), and a daily
  workflow runs mutation testing and fuzzing. What hergy prints and accepts
  does not change.

## 0.3.1 - 2026-10-15

### Changed

- **`hergy <command> --help` prints that command's own page**: what it does,
  how to type it, its arguments, and every flag it takes, each with what the
  flag means for that command. `hergy new --help` says that `--local` is where
  the starter team is written (the global layer without it), and that the
  global `config.yaml` is written when it is missing. `-h` prints the same.
  Before, every command's `--help` printed the one list of all commands, and
  `--local` of `hergy new` was written only in the README and in
  `hergy cheat --ai`.
- **`hergy`, `hergy --help` and `hergy cheat` point at the new pages**: the
  list of all commands is as it was, with one line added at its end, and the
  last line of `hergy cheat` names `hergy <command> --help`.
- **`hergy cheat --ai` is written from the same table as `--help`**: each
  command's line, and the commands that take each flag, are read from it, so
  they cannot fall behind what hergy accepts. The lines change shape: values
  are `<team>` rather than `team`, `hire` shows `--purpose <purpose> --task
  <task>`, `--lang` is no longer repeated on every command's line, and each
  line of the Flags section ends with the commands that take the flag. The
  Flags section also lists `--all`, which it lacked. What hergy accepts does
  not change, so there is nothing to move.
- **An unknown option, or a flag the command does not take, points at that
  command's page**: ``unknown option "--foo" (see `hergy new --help`)`` and
  ``--as cannot be used with hergy ls (see `hergy ls --help`)``. The first used
  to point at `hergy help`, the second at nothing. An unknown command still
  points at `hergy help`. When an error is raised, its exit code, and what
  goes to stderr are as before.

## 0.3.0 - 2026-10-13

This release tells a team definition apart from a running team. A team
definition is `teams/<name>/team.yaml`; a team is what `hergy up` starts from
one, under its own name. Commands that act on a running team now take the team
name, so two teams started from one definition can be handled one at a time.

### Breaking

- **Commands on a running team take the team name**: `status`, `down`,
  `refresh`, `reprompt` and `hire` name a team (`hergy down dev2`,
  `hergy refresh lead --team dev2`), not the team definition it was started
  from. `--as` belongs to `up` alone; on the other commands it is an error.
  hergy finds the definition from the team's tab in herdr, which `up` now
  labels `team:<team>@<definition>`; a tab labelled the older way,
  `team:<team>`, is read as a team whose definition has the same name.
- **One word for the people in a team**: everyone written in `team.yaml` is a
  member, and everyone `hergy hire` adds is a worker. `seat` is gone from the
  output and the docs, and the commands that take one say `<member>`.
- **`may_spawn` pairs each purpose with the profile to hire for it**: the
  separate `purposes` and `profiles` lists are gone, and a `team.yaml` that
  still has them is refused with the new form in the message. `hergy hire`
  can leave out `--profile`; the profile paired with `--purpose` is used.
- **The starter team's second member is `builder`**: `hergy new` writes
  `roles/builder.md` instead of `roles/worker.md`, so the member is not
  mistaken for a hired worker. Teams already written are not touched.

### Moving from 0.2

1. Name the running team, not its definition, on `status`, `down`,
   `refresh`, `reprompt` and `hire`. For a team started without `--as` the
   two names are the same, and nothing changes. For one started with `--as`:

   ```sh
   # 0.2
   hergy down dev --as dev2
   hergy refresh lead --team dev --as dev2
   # now
   hergy down dev2
   hergy refresh lead --team dev2
   ```

2. Teams already running keep working: their tabs carry the older label, and
   hergy reads the team name from it as the definition name. A team started
   with `--as` under 0.2 cannot be traced back to its definition that way;
   run `hergy down <definition> --as <team>` with 0.2 before upgrading, or
   close its panes in herdr, then start it again with `hergy up`.
3. Rewrite each `may_spawn` in `team.yaml` as pairs. Every purpose gets the
   one profile its workers should run on:

   ```yaml
   # 0.2
   may_spawn:
     purposes: [research, build]
     profiles: [seeker, developer]
   # now
   may_spawn:
     workers:
       - purpose: research
         profile: seeker
       - purpose: build
         profile: developer
   ```

   `hergy validate` names every `team.yaml` that still has the old form.
   `hergy hire` still takes `--profile`; leave it out to use the pair.
4. Nothing else to do in `team.yaml`, `profiles.yaml` or `config.yaml`:
   teams written by `hergy new` before keep their `worker` member.

### Added

- **`hergy status` with no name lists every running team**: each with its
  team definition, how many members and workers it has, and whether someone
  waits for a human or nobody is working. A team started from a definition in
  another project is listed with a note instead of stopping the list.
- **`hergy ls` names the running teams**: each team definition shows the
  teams started from it.

### Changed

- **`hergy --help` is grouped by what you want to do**, and the long rules
  moved to the README.
- **`hergy cheat` is a short table of what to type**, and its columns line up
  in Japanese too.
- **The Japanese messages read as plain Japanese.**
- **`hergy show` and `hergy up --dry` print the team definition and the team
  name on separate lines.**

## 0.2.0 - 2026-10-13

### Breaking

- **`hergy refresh` restarts a member**: it ends the member's agent and starts
  it again in the same pane with the settings as they are now, so the
  conversation starts empty and an edited `profile`, `kind` or `args` takes
  effect. A member whose agent has stopped is started again in its pane. What
  `refresh` did before, sending the bootstrap prompt again and keeping the
  conversation, is now `hergy reprompt`.

### Moving from 0.1

- Where you typed `hergy refresh <member>` to resend the bootstrap prompt and
  keep the conversation, type `hergy reprompt <member>`. Type
  `hergy refresh <member>` only when the conversation should start over.
- Workers hired with 0.1 are not closed by `hergy down`: their tabs carry the
  older label. Close them in herdr once.

### Added

- **`hergy status`** shows each member and worker with its state, and says who
  waits for a human or that nobody is working.
- **`hergy cheat`** prints a cheatsheet for people, and `hergy cheat --ai` the
  whole of hergy for an agent. Neither reads the settings.
- **A skill for agents**: `npx skills add aq2bq/hergy` installs `hergy`, which
  tells an agent when to use hergy and when herdr, and points at
  `hergy cheat --ai` for the exact commands.
- **The bootstrap prompt says to add teammates with `hergy hire`**, not
  `herdr agent start`, so every teammate shows up in `status` and `down`.

### Changed

- **`hergy down` also closes the workers the team hired**: `hire` labels a
  worker's tab `worker:<team>/<name>`, and `down` closes the agents in those
  tabs.

## 0.1.0 - 2026-10-13

The first tagged release.

- `hergy up`, `down`, `refresh`, `hire`, `ls`, `show`, `new`, `validate` and
  `version`.
- Team definitions in two layers, global (`~/.config/hergy`) and local
  (`<project>/.hergy`), with `profiles.yaml` in either.
- `hergy up <definition> --as <team>` starts one definition as several teams.
- Member names are written short (`lead`); hergy adds the team name in herdr
  (`dev-lead`).
- `hergy new` writes the global `config.yaml` when it is missing.
- `--dry` prints the plan, and `--json` prints it as JSON.
