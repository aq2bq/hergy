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
```

You also need [herdr](https://github.com/herdrdev/herdr) and the agent CLIs you use (`claude`, `codex`, `opencode` and so on).

## Getting started

Write out a starter team of two, a lead and a worker.

```sh
hergy new myteam        # writes to ~/.config/hergy/teams/myteam/ (--local writes to the project's .hergy/)
```

You get `team.yaml`, `protocol.md` (the team's agreements), `roles/lead.md` and `roles/worker.md`. Each rule is one line, with its "why" in an HTML comment right below it. HTML comments are not sent to the agents.

Run it in a herdr shell pane.

```sh
hergy up myteam --dry   # check each member's prompt and the files it reads, without starting
hergy up myteam         # start
```

## Commands

| Command | What it does |
|---|---|
| `hergy new <team>` | Write out a starter team of two, a lead and a worker |
| `hergy up [team]` | Start a team. `--dry` prints the plan as JSON without touching herdr |
| `hergy refresh <member>` / `--all` | Re-send the bootstrap prompt to waiting members |
| `hergy hire <name> --profile P --purpose X --task "…"` | Start a worker in its own tab and hand it a task (mostly used by members) |
| `hergy ls` | List the available teams and their layers |
| `hergy show <team>` | Print the final settings and which file each value came from |
| `hergy validate` | Check every settings file and show each broken one |

Common options: `--global` / `--local` (pick a layer), `-C <dir>` (pick the project), `--lang ja` (show Japanese; English by default). `refresh` and `hire` take `--team` to pick the team.

## Where settings live

| Layer | Location |
|---|---|
| Global | `$XDG_CONFIG_HOME/hergy/` (`~/.config/hergy/` when unset) |
| Local | `<project>/.hergy/` |

- A team lives in `teams/<team>/team.yaml`. When both layers have the same name, the local one wins
- `profiles.yaml` lives in the global layer only
- Put `version: 1` and `lang: ja` in the global `config.yaml` to get Japanese without `--lang`
- `hergy up` shows the files each member read

## team.yaml

```yaml
version: 1              # required
team: myteam            # defaults to the directory name
cwd: .                  # working directory (relative to the project root)
docs: [protocol.md]     # documents for every member, joined in order; a directory means its *.md in name order
roles: [roles]          # where to look for role files (defaults to roles/)
prompt: "…"             # bootstrap prompt template (optional)
members:
  - name: lead          # the herdr name
    role: lead          # reads roles/lead.md
    profile: opus       # or write kind / args / env directly
    may_spawn:          # purposes and profiles this member may hire for (written into the prompt, not enforced)
      purposes: [research]
      profiles: [codex]
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
| `%{team}` / `%{role}` / `%{name}` | Team name, role, member name |
| `%{role_doc}` | The role file |
| `%{docs}` | The `docs` documents |
| `%{roster}` | The roster |
| `%{herdr}` | How to reach others over herdr |
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

## License

[MIT](LICENSE)
