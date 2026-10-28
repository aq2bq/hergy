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
hergy new myteam        # writes into the project's .hergy/ inside a git repository, into ~/.config/hergy/ outside one
```

You get the team's `team.yaml` and `agreement.yaml` (its agreements), and `roles/lead.yaml` and `roles/builder.yaml`. A rule file is YAML: each rule has an id, its body, whether it is on, the day it was switched off and a note for people. Only the body of a rule that is on is sent to the agent; a note is never sent.

Run it in a herdr shell pane.

```sh
hergy up myteam --dry   # check each member's prompt and the files it reads, without starting
hergy up myteam         # start
```

The members run in herdr as `myteam-lead` and `myteam-builder`: the team name is added to each member name for you (see below).

Lost, type `hergy cheat` for a short cheatsheet (`hergy cheat --ai` prints the whole shape for an agent). `hergy <command> --help` prints one command's flags, each with what it means for that command; `hergy new --help` says where it writes.

## Commands

| Command | What it does |
|---|---|
| `hergy new <definition>` | Write out a starter team definition of two, a lead and a worker |
| `hergy up [definition]` | Start a team from a team definition, named by `--as` (the definition name by default). `--dry` prints the plan (each member's prompt as text) without touching herdr; `--json` prints the plan as JSON instead |
| `hergy down [team]` | Close the live panes of the running team's members and the workers the team hired. `--dry` prints what would close; a tab is never closed as a unit |
| `hergy status [team]` | With a team name: one line per member and per worker, the live `agent_status` (working/idle/done/blocked), or `not present` when the member is not live; a worker carries `(worker)`. Then who is waiting on the human (`blocked`), or that nobody is working. With no team name: every running team, with its team definition, its counts and the same judgement. Reads herdr only |
| `hergy apply [<member>…]` | Compare the saved definition with the running teams and apply each member's drift: send its prompt again when the prompt changed (the conversation stays), start a member no agent is live for, or restart a member whose profile changed (the conversation goes). With no member named, every member with drift is applied. A member that is working or waiting for permission is left, with the reason, and the rest go on; the command answers 1 while one is left. `--dry` prints each member's way of applying and what changed, and changes nothing |
| `hergy refresh <member>` / `--all` | Restart each selected member of a running team: end its agent and start it again in the same pane, so its conversation starts empty. A member that is not live is started again in the pane hergy named after it, when that pane is still at its shell (a pane that is busy is an error, pointing at `down` then `up`). A member with no pane is refused, and `hergy apply` is named: adding a member to a running team is the apply's road. The team definition is read again, so an edited profile (its kind, args and prompt) takes effect (`env` is the exception; see below). A member cannot refresh itself: run it from another pane, or as the human |
| `hergy reprompt <member>` / `--all` | Re-send the bootstrap prompt to waiting members, keeping each conversation. When the launch record shows that the definition changed, the prompt is headed with what changed and with the words that make it the new bootstrap prompt |
| `hergy hire <name> --purpose X --task "…" [--from <member>]` | Start a worker in its own tab (labelled `worker:<team>/<name>`) and hand it a task (mostly used by members). `--purpose` must be a purpose the hiring member's `workers:` names, and the profile comes from that entry; anything else is refused, naming the purposes the member may hire for. `--from` may be left out when exactly one member hires for that purpose |
| `hergy ls` | List the available team definitions and the running team names each one booted |
| `hergy show <definition>` | Print the final team definition and which file each value came from |
| `hergy validate` | Check every settings file and show each broken one. A team definition is checked once and a role document once (not once per team that uses it); it then prints the count of rules and the size of each team definition's rules and templates and of each role document, warning when one is over the provisional budget (3,000 characters each); a warning does not change the exit code |
| `hergy serve` | Serve the screens on this machine, print the URL and open it in the default browser (`--port`, default 8787; `0` lets the operating system pick a free one; `--no-open` prints the URL and opens nothing). It shows the profiles and the roles: each profile's name, kind, arguments and environment with the members and workers that run it, and the role documents behind the members' rules. It listens on 127.0.0.1 only, and Ctrl-C stops it after the answers in flight |
| `hergy version` | Print the commit and time of the installed hergy |
| `hergy cheat` | Print a short cheatsheet. `--ai` prints the whole shape for an agent, in English, without reading any settings |

Common options: `-C <dir>` (pick the project), `--lang ja` (the command line's output in Japanese; English by default; the WebUI that serve opens is Japanese whatever this says), `--json` (with `--dry`, print the plan as JSON instead of text). `apply`, `refresh`, `reprompt` and `hire` take `--team` to pick the team by team name, and `apply` and `refresh` also take `--timeout`. `up` takes `--as <team>` to run the same team definition under a different team name (see below).

`hergy apply` is the one operation for a change to a running team: hergy compares the saved definition with the prompt each member last read, decides the way for each member (send the prompt again, start it, restart it, or say it cannot be applied), and a person only picks whom to apply. The command line names the members (or applies every member with drift); the screens read the same plan at `GET /api/apply` and apply it with `POST /api/apply`. `reprompt` and `refresh` stay as the single-purpose tools: `reprompt` sends the prompt again (with a preamble saying what changed), and `refresh` restarts a member (for one that is stuck). `env` is fixed when the pane is made, so after editing `env` run `hergy down` then `hergy up`; apply says so and does not restart the member.

`hergy serve` shows the same settings in a browser, read the same way. It prints the URL and opens it in the default browser of this machine; `--no-open` prints the URL and opens nothing, and a browser that cannot be started is one line on stderr, with the URL to open by hand. It is for this machine only: it listens on 127.0.0.1, refuses a request that was not addressed to its own name, and takes a write only as a POST that carries a token which the running process and the pages it rendered know. It changes no file that a screen does not edit: a role document, the two profiles.yaml files, a team definition's two files, and the `projects:` list of the global `config.yaml`.

`hergy serve` is a headquarters for the projects you list: list them under `projects:` in the global `config.yaml` (absolute paths, or `~/…`), and every listed project's teams, roles and profiles show, each with the project it belongs to, wherever `hergy serve` is started. Without `projects:` it shows the project it was started in and the global settings, as before. The sheet that Projects at the top of the screen opens lists and unlists projects (unlisting changes no file of that project), and a team definition's 動き column starts and stops a listed project's team, choosing a herdr workspace (one team to a workspace) with the same result as `hergy up` and `hergy down`. The band says 反映していない変更 N when a running team differs from its saved definition; the press opens the apply sheet, which shows each member's way of applying, what changed, and whether the conversation goes, and applies the members a person ticks, with the same result as `hergy apply`. The sheet shows the progress and any question the agent asks while a member starts. A running team is found even when no agent runs in it: a tab labelled with the team's name that holds an agent, or a pane named `<team>-<member>`, is that team. The activity screen at `/activity` shows waiting and stalled rows above one tile per running team. Each tile includes its diagram and member and worker counts, and links to the team definition, apply sheet and stop sheet. The REST API under `/api/hq/` answers the same reading.

## Where team definitions and settings live

| Where | Location |
|---|---|
| Outside the project | `$XDG_CONFIG_HOME/hergy/` (`~/.config/hergy/` when unset) |
| The project | `<project>/.hergy/` |

- A team definition lives in `teams/<definition>/team.yaml`. When a name is in both places, the project's one wins
- `profiles.yaml` and the global `config.yaml` are settings files. `profiles.yaml` can be in both places: a team definition in the project reads the project's over the one outside it (the project's wins for the same name); a definition outside reads only the one outside
- Put `version: 1` and `lang: ja` in the global `config.yaml` to get Japanese without `--lang`; `hergy new` writes it, with the language that run showed, when it is missing, and leaves it alone when it is there
- `projects:` in the global `config.yaml` is the headquarters' list: absolute paths or `~/…` paths, expanded when read. Without the key, `hergy serve` shows the project it was started in; with an empty list, only the global settings. A relative path is a broken setting, and `hergy validate` and `hergy serve` say so
- `hergy up <definition> --dry` shows each member's prompt and the files it read

## A team definition (team.yaml)

```yaml
version: 1              # required
mission: "what this team is for"  # required; one line, shown first by the commands that handle a team and by the web UI
team: myteam            # optional; if written it must match the team definition name (teams/<definition>/)
cwd: .                  # working directory (relative to the project root)
                        # no key points at the documents: teams/<definition>/agreement.yaml
                        # (this team) and roles/<role>.yaml are read (the project's .hergy/
                        # first, then ~/.config/hergy/)
prompt: "…"             # bootstrap prompt template (optional); it must keep %{role_doc} and %{docs}
members:
  - name: lead            # the member name (short); the herdr name becomes "<team>-lead"
    role: lead          # reads roles/lead.yaml
    profile: opus       # a named profile from profiles.yaml; kind, args and env are written there, not here
workers:                # purpose-to-profile pairs a member may hire for (written into the prompt, not enforced)
  - from: lead          # the member that may hire this worker
    purpose: research   # what the worker is for
    profile: opus       # the profile that worker runs
turns:                  # who hands the turn to whom (the human is written "human")
  - human -> lead
  - lead -> human
```

### Running one team definition as more than one team

`--as <team>` boots the same team definition under a different team name, so you can run two teams side by side from one `team.yaml`. Without `--as`, the team name is the team definition name (the directory name under `teams/`).

Write each `members[].name` as a short member name (`lead`, `builder`). hergy adds the team name when it boots the team, so the herdr name is `<team>-<member>`: the `demo` definition gives `demo-lead` and `demo-builder`, and `--as dev2` gives `dev2-lead` and `dev2-builder`. Because hergy adds the team name for you, two team definitions that both name a member `lead` never collide in herdr. A member name that still contains `%{team}` is refused: that placeholder is no longer used in names.

The team name must match `[a-z][a-z0-9_-]{0,24}`, and the resulting `<team>-<member>` must match `[a-z][a-z0-9_-]{0,31}`. hergy keeps no record of running teams; a team command finds the team definition from the team's herdr tab (labelled `team:<team>@<definition>`), so `down`, `apply`, `refresh`, `reprompt` and `hire` all point at a running team by its team name. The members on the command line (`apply <member>`, `refresh <member>`, `reprompt <member>`, `hire --from <member>`) are the short member names:

```sh
hergy up demo --as alpha            # boot the demo definition as the team "alpha"
hergy up demo --as beta             # and once more as "beta"
hergy apply --team alpha            # apply alpha's drift, showing what changed
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
| `%{role_doc}` | The role document |
| `%{docs}` | The `docs` documents |
| `%{roster}` | The roster |
| `%{herdr}` | How to reach others over herdr, and how to add a teammate (`hergy hire`, not `herdr agent start`) |
| `%{turns}` | Who the member hands the turn to, from the definition's `turns:` |
| `%{workers}` | What the member may hire, and the command (only for members `workers:` names one for) |

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
