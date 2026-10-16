# hergy

English | [日本語](README.ja.md)

> [!NOTE]
>
> hergy is an experimental project. How you use it may change significantly for a while, and compatibility is not promised yet.

hergy is a small CLI, built on [herdr](https://github.com/herdrdev/herdr) and [gy](https://github.com/aq2bq/gy), for getting through highly uncertain work with a team of AI agents.

hergy is for people who want to build things with a team of AI agents, with as little effort as possible. Write in YAML who runs in which role on which model, and hergy brings the whole team up on herdr at once.

## What hergy does, and what it doesn't

hergy only manages and runs the team's makeup and its rules. The AI agent harnesses that vendors provide, such as Claude Code, Codex and OpenCode, are used as they are. Coordination between agents, status checks and startup are left to herdr as they are.
Needs, questions, requirements, acceptance criteria, decisions and the like that come up during a session are all recorded in gy and used from there.

For now, hergy aims to make putting a team together easy, by managing and running the roles and rules you set for your team as files.


## Install

```sh
go install github.com/aq2bq/hergy@latest

# optional: install the hergy skill, so an AI agent can read it
npx skills add aq2bq/hergy
```

You also need [herdr](https://github.com/herdrdev/herdr) and the agent CLIs you use (`claude`, `codex`, `opencode` and so on).

### Telling the AI about hergy

Put a sentence like the following in a prompt or in AGENTS.md / CLAUDE.md so it gets across, and the AI agent reads the hergy skill before it starts or joins a team.

```
When you start or close a hergy team, write a hergy team definition, or work inside a team hergy started, read the hergy skill.
```

## Getting started

Write out a starter team of two, a lead and a worker.

```sh
hergy new myteam        # writes to ~/.config/hergy/teams/myteam/ (--local writes to the project's .hergy/)
```

You get `team.yaml`, `protocol.yaml` (the team's agreements), `roles/lead.yaml` and `roles/builder.yaml`. A rule file is YAML: each rule has an id and its text, and, when the file gives them, its reason, its origin and the report or test that shows it was kept. Only what a member reads is sent to the agent.

Run it in a herdr shell pane.

```sh
hergy up myteam --dry   # check each member's prompt and the files it reads, without starting
hergy up myteam         # start
```

The members run in herdr as `myteam-lead` and `myteam-builder`: the team name is added to each member name for you (see below).

Lost, type `hergy cheat` for a short cheatsheet (`hergy cheat --ai` prints the whole shape for an agent). `hergy <command> --help` prints one command's flags, each with what it means for that command; `hergy new --help` says where `--local` writes.

## Commands

| Command | What it does |
|---|---|
| `hergy new <definition>` | Write out a starter team definition of two, a lead and a worker |
| `hergy up [definition]` | Start a team from a team definition, named by `--as` (the definition name by default). `--dry` prints the plan (each member's prompt as text) without touching herdr; `--json` prints the plan as JSON instead |
| `hergy down [team]` | Close the live panes of the running team's members and the workers the team hired. `--dry` prints what would close; a tab is never closed as a unit |
| `hergy status [team]` | With a team name: one line per member and per worker, the live `agent_status` (working/idle/done/blocked), or `not present` when the member is not live; a worker carries `(worker)`. Then who is waiting on the human (`blocked`), or that nobody is working. With no team name: every running team, with its team definition, its counts and the same judgement. Reads herdr only |
| `hergy refresh <member>` / `--all` | Restart each member of a running team. A live member's agent is ended and started again in the same pane, so its conversation starts empty; a member that is not live is started again in the pane hergy named after it, when that pane is still at its shell (a pane that is gone or busy is an error, pointing at `down` then `up`). The team definition is read again, so an edited profile (its kind, args and env), or an edited prompt, takes effect (`env` is the exception; see below). A member cannot refresh itself: run it from another pane, or as the human |
| `hergy reprompt <member>` / `--all` | Re-send the bootstrap prompt to waiting members, keeping each conversation |
| `hergy hire <name> --purpose X --task "…" [--profile P]` | Start a worker in its own tab (labelled `worker:<team>/<name>`) and hand it a task (mostly used by members). Without `--profile` the profile may_spawn pairs with `--purpose` is used |
| `hergy ls` | List the available team definitions and their layers, and the running team names each one booted |
| `hergy show <definition>` | Print the final team definition and which file each value came from |
| `hergy validate` | Check every settings file and show each broken one. It then prints what the rules carry and the size of what each member is handed and of each team definition's rule files, warning when one is over the provisional limit; a warning does not change the exit code |
| `hergy serve` | Serve the screens on this machine, print the URL and open it in the default browser (`--port`, default 8787; `0` lets the operating system pick a free one; `--no-open` prints the URL and opens nothing). Now it shows the layer you pick: one row per team definition, with its members and how many of the rules they read have no check. It listens on 127.0.0.1 only, and Ctrl-C stops it after the answers in flight |
| `hergy version` | Print the commit and time of the installed hergy |
| `hergy cheat` | Print a short cheatsheet. `--ai` prints the whole shape for an agent, in English, without reading any settings |

Common options: `--global` / `--local` (pick a layer; for `new`, `--local` picks where to write), `-C <dir>` (pick the project), `--lang ja` (show Japanese; English by default), `--json` (with `--dry`, print the plan as JSON instead of text). `refresh`, `reprompt` and `hire` take `--team` to pick the team by team name, and `refresh` also takes `--timeout`. `up` takes `--as <team>` to run the same team definition under a different team name (see below).

`refresh` and `reprompt` are the two ways to hand a member its prompt again. `reprompt` keeps the conversation, so it is what you want after editing a document the member reads. `refresh` ends the agent and starts it again, so the conversation starts empty and an edited profile (its `kind`, `args` and `env`) or prompt is picked up; a member that is no longer running is started again in the pane hergy named after it, when that pane is still at its shell (if the pane is gone or busy, `refresh` changes nothing and points at `down` then `up`). `env` is fixed when the pane is made, so after editing `env` run `hergy down` then `hergy up`; `refresh` keeps the pane and its env. `refresh` refuses a member running in the pane hergy itself is in (`HERDR_PANE_ID`): stopping it would stop hergy before it could start the agent again. Run `refresh` from another pane, or as the human. `refresh --dry` reads the live agents so it shows the same refusal; `reprompt --dry` reads nothing.

`hergy serve` shows the same settings in a browser, read the same way. It prints the URL and opens it in the default browser of this machine; `--no-open` prints the URL and opens nothing, and a browser that cannot be started is one line on stderr, with the URL to open by hand. It is for this machine only: it listens on 127.0.0.1, refuses a request that was not addressed to its own name, and takes a write only as a POST that carries a token which the running process and the pages it rendered know. It changes no file that a screen does not edit.

## Where team definitions and settings live

| Layer | Location |
|---|---|
| Global | `$XDG_CONFIG_HOME/hergy/` (`~/.config/hergy/` when unset) |
| Local | `<project>/.hergy/` |

- A team definition lives in `teams/<definition>/team.yaml`. When both layers have the same name, the local one wins
- `profiles.yaml` and the global `config.yaml` are settings files. `profiles.yaml` can live in both layers: a local team definition reads the local one over the global one (local wins for the same name); a global one reads only the global one
- Put `version: 1` and `lang: ja` in the global `config.yaml` to get Japanese without `--lang`; `hergy new` writes it, with the language that run showed, when it is missing, and leaves it alone when it is there
- `hergy up <definition> --dry` shows each member's prompt and the files it read

## A team definition (team.yaml)

```yaml
version: 1              # required
mission: "what this team is for"  # optional; one line, shown first by the commands that handle a team and by the web UI
team: myteam            # optional; if written it must match the team definition name (teams/<definition>/)
cwd: .                  # working directory (relative to the project root)
docs: [protocol.yaml]   # documents for every member, joined in order; a directory means its *.yaml in name order
roles: [roles]          # where to look for role files (defaults to roles/)
prompt: "…"             # bootstrap prompt template (optional)
members:
  - name: lead            # the member name (short); the herdr name becomes "<team>-lead"
    role: lead          # reads roles/lead.yaml
    profile: opus       # a named profile from profiles.yaml; kind, args and env are written there, not here
    may_spawn:          # purpose-to-profile pairs this member may hire for (written into the prompt, not enforced)
      workers:
        - purpose: research
          profile: opus
```

### Running one team definition as more than one team

`--as <team>` boots the same team definition under a different team name, so you can run two teams side by side from one `team.yaml`. Without `--as`, the team name is the team definition name (the directory name under `teams/`).

Write each `members[].name` as a short member name (`lead`, `builder`). hergy adds the team name when it boots the team, so the herdr name is `<team>-<member>`: the `demo` definition gives `demo-lead` and `demo-builder`, and `--as dev2` gives `dev2-lead` and `dev2-builder`. Because hergy adds the team name for you, two team definitions that both name a member `lead` never collide in herdr. A member name that still contains `%{team}` is refused: that placeholder is no longer used in names.

The team name must match `[a-z][a-z0-9_-]{0,24}`, and the resulting `<team>-<member>` must match `[a-z][a-z0-9_-]{0,31}`. hergy keeps no record of running teams; a team command finds the team definition from the team's herdr tab (labelled `team:<team>@<definition>`), so `down`, `refresh`, `reprompt` and `hire` all point at a running team by its team name. The members on the command line (`refresh <member>`, `reprompt <member>`, `hire --from <member>`) are the short member names:

```sh
hergy up demo --as alpha            # boot the demo definition as the team "alpha"
hergy up demo --as beta             # and once more as "beta"
hergy reprompt lead --team alpha    # re-send the prompt to alpha's lead, keeping its conversation
hergy down alpha                    # close alpha's members and hired workers only
```

## profiles.yaml

```yaml
version: 1
profiles:
  opus:
    kind: claude                          # herdr's agent kind
    args: ["--model", "claude-opus-5-5"]  # passed to the agent CLI as is
  dev:
    kind: opencode
    env:                                  # environment variables for the pane
      OPENCODE_CONFIG_CONTENT: '{"model":"…"}'
```

## Bootstrap prompt placeholders

| Placeholder | Content |
|---|---|
| `%{team}` / `%{role}` / `%{name}` | Team name, role, herdr name (`"<team>-<member>"`) |
| `%{role_doc}` | The role file |
| `%{docs}` | The `docs` documents |
| `%{roster}` | The roster |
| `%{herdr}` | How to reach others over herdr, and how to add a teammate (`hergy hire`, not `herdr agent start`) |
| `%{spawn}` | What the member may hire, and the command (only for members with `may_spawn`) |

The default template is provisional.

## What's next

- Show which member holds the turn (`hergy turn`)
- A fixed shape and a record for messages between members (`hergy pass`, `hergy log`)
- List the rules, and take one out to compare (`hergy rules`)
- Receive questions for the human, give instructions by role name (`hergy ask`, `hergy tell`)
- Validate the settings (`hergy validate`)
- Promote and migrate settings (`hergy promote`, `migrate`)
- Nested teams, a TUI

## Contributing

Before you send a change, run `go run ./tools/check`. It runs every quality
gate CI runs (`go run ./tools/check -h` lists them).

## License

[MIT](LICENSE)
