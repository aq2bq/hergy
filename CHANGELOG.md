# Changelog

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
