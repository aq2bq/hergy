# hergy

English | [日本語](README.ja.md)

Running several coding agents side by side is no longer the hard part. With [herdr](https://github.com/herdrdev/herdr) you can split panes, start an agent in each, and even let the agents send instructions to one another.

The hard part comes next: getting those agents to work **as a team**. Give one agent the job of steering, another the job of pinning down requirements, a third the job of building, and the work really does start to move. But the better it runs, the more rules you add, and the thicker the prompts grow. Every session you ask the agents to "coordinate among yourselves," and every day you swap models by hand depending on how much quota is left. Then one day you notice you can no longer explain why the team works at all.

hergy is a tool for people who want to build things with a team of AI agents, with as little effort as possible. Write down in YAML who is on the team, in which role, on which model, and hergy brings the whole team up on herdr at once.

## What hergy does, and what it leaves to others

hergy owns two things: how a team is put together, and a way to layer rules and show which ones are in effect.

Everything else goes to tools that do it better:

- Running the agents: each agent CLI, such as Claude Code, Codex or OpenCode
- Where the agents live, and how they talk to each other: herdr
- Recording what has been decided and what is still open: [gy](https://github.com/aq2bq/gy)

LLMs and agent CLIs change shape month by month. The more a tool takes on, the faster those changes leave it behind. That is why hergy insists on staying thin.

For the same reason, hergy ships none of the rules themselves. How each role behaves and what the team agrees on are things you write in your own settings. hergy layers them, hands them to the agents, and shows you which file each rule came from.

A team pays off most on work you cannot plan in advance, not on a fixed pipeline: pinning down requirements, checking records against reality, drawing out a human's judgement. If what you want is to run a fixed sequence of steps automatically, other tools fit better.

## Status

hergy is at an early stage. It can start a team, re-send a role, hire a worker and show you the resolved settings, but compatibility is not promised yet. Settings files carry a format version (`version`); when the format changes, hergy stops on an old version instead of reading it silently. A `migrate` command to move between versions is still to come.

The default text of the first prompt sent to each agent (the bootstrap prompt) is in Japanese, and it is provisional. To use English, or to decide its content yourself, write your own template in `prompt` in `team.yaml`.

## Install

```sh
go install github.com/aq2bq/hergy@latest
```

You also need [herdr](https://github.com/herdrdev/herdr) and the agent CLIs your team uses (`claude`, `codex`, `opencode` and so on).

## Getting started

Settings are two kinds of YAML file plus one Markdown file per role. Start with the model setups (profiles) you use across the machine.

```yaml
# ~/.config/hergy/profiles.yaml
version: 1
profiles:
  opus:  { kind: claude, args: ["--model", "claude-opus-5-5"] }
  codex: { kind: codex,  args: [] }
```

Then write the team.

```yaml
# ~/.config/hergy/teams/myteam/team.yaml
version: 1
docs: [protocol.md]      # documents every member gets (optional)
members:
  - name: lead
    role: lead
    profile: opus
  - name: impl
    role: impl
    profile: codex
```

Each role is described in `roles/<role>.md` inside the team directory; here that is `roles/lead.md` and `roles/impl.md`. Agreements shared by the whole team go in `protocol.md`.

Inside a herdr shell pane, check the plan first without starting anything.

```sh
hergy up myteam --dry
```

For each member you see which role file and which documents are used, and the fully expanded prompt. If it looks right, start the team.

```sh
hergy up myteam
```

A new tab opens with one pane per member, each agent starts, and each receives its bootstrap prompt.

## Commands

| Command | What it does |
|---|---|
| `hergy up [team]` | Start a team. With `--dry`, print the plan as JSON without touching herdr |
| `hergy refresh <member>` / `--all` | Re-send the bootstrap prompt to members that are already running. Use it after editing a role file or resetting an agent's conversation |
| `hergy hire <name> --profile P --purpose X --task "…"` | Start one worker in its own tab and hand it a task. Usually run by a member, not by you |
| `hergy ls` | List the teams hergy can see, with the layer each one lives in |
| `hergy show <team>` | Print the resolved settings, and which file each value and document came from |

Every command takes `--global` / `--local` to pick a layer and `-C <dir>` to name the project. `refresh` and `hire` take `--team` to name the team.

`refresh` only sends when the member is waiting for input (herdr reports `idle` or `done`) and its agent kind matches the settings. It never sends to a member that is working.

A worker started with `hire` reports back over herdr to the member that hired it when it is done. `may_spawn` states which purposes and profiles a member may hire for. This is only written into that member's prompt: a hire outside it prints a note and goes ahead anyway.

## Where settings live

Team settings can live in two places.

| Layer | Location |
|---|---|
| Global | `$XDG_CONFIG_HOME/hergy/` (`~/.config/hergy/` when unset) |
| Local | `<project>/.hergy/` |

Each holds `teams/<team>/team.yaml`. Profiles (`profiles.yaml`) live in the global layer only.

You call a team by name alone. When both layers have a team with the same name, the local one wins. Leave the name out and hergy uses the only local team if there is exactly one, and otherwise stops and lists the candidates. The project is the git root, or the current directory outside git.

Once settings are split across two places, it is easy to lose track of which one is in effect. So every `hergy up` prints, for each member, the role file and documents it read, down to the layer and file. `hergy ls` marks teams where a local team overrides a global one.

## team.yaml

```yaml
version: 1              # required
team: myteam            # defaults to the directory name
cwd: .                  # where members work, relative to the project root
docs: [protocol.md]     # documents for every member, relative to this file; a directory means its *.md in name order
roles: [roles]          # where to look for role files; defaults to roles/
prompt: "…"             # bootstrap prompt template (optional)
members:
  - name: lead          # the herdr name; starts with a lowercase letter
    role: lead          # reads roles/lead.md
    profile: opus       # a name from profiles.yaml; or write kind / args / env directly
    cwd: .              # optional; overrides the team value
    prompt: "…"         # optional; overrides the team value
    may_spawn:          # optional; purposes and profiles this member may hire
      purposes: [research]
      profiles: [codex]
```

`docs` are joined in the order you list them. Without `docs`, no documents go in. To try the team without a rule, take that document out of `docs`.

Role files are searched for in the order of `roles`, and the first one found is used whole. If none is found, hergy stops and lists where it looked.

Values in `args` and `env` must be strings. Quote numbers.

## profiles.yaml

```yaml
version: 1
profiles:
  opus:
    kind: claude                      # herdr's agent kind
    args: ["--model", "claude-opus-5-5"]
  dev:
    kind: opencode
    args: []
    env:
      OPENCODE_CONFIG_CONTENT: '{"model":"…"}'
```

Each element of `args` becomes one argument to the agent CLI as is; no shell is involved. How to name a model or a reasoning level differs from one agent CLI to another, so follow the help of the CLI you have installed.

`env` is passed as environment variables when the member's pane is created, for agents that take their model from the environment.

## How the bootstrap prompt is built

A template can use these placeholders.

| Placeholder | Content |
|---|---|
| `%{team}` / `%{role}` / `%{name}` | The team name, the role, and the member's name |
| `%{role_doc}` | The role file (HTML comments removed) |
| `%{docs}` | The `docs` documents, joined in order |
| `%{roster}` | The team roster |
| `%{herdr}` | How to reach the others over herdr |
| `%{spawn}` | Only for members with `may_spawn`: what they may hire, and a hire command ready to run |

An unknown placeholder is an error. A document the template never reads does not count as used, and `hergy up` does not list it.

## What's next

Everything from here on is a plan.

- **Who holds the turn right now**: herdr knows whether each agent is working or stopped, and gy knows what is still undecided. Put the two together and you should be able to see whether the team's loop has stalled (`hergy turn`)
- **A fixed shape for messages between members**: from whom, to whom, under which rule, and of what kind. With a record of that, you can count which rules are actually used (`hergy pass`, `hergy log`)
- **Rules as parts**: list them, look up why each exists, take one out and run a few times to compare (`hergy rules`)
- **A way in for the human**: take pending questions one at a time, talk to a role by its name, leave a one-line note (`hergy ask`, `hergy tell`, `hergy note`)
- **Looking after settings**: create, open, try locally and promote to global, migrate between format versions (`hergy new`, `edit`, `fork`, `promote`, `migrate`)
- **Nested teams and a TUI**

The fewer the rules, the easier they are to look after. How far they can shrink while the team still runs, though, nobody knows yet. Making that measurable is hergy's next job.

## License

[MIT](LICENSE)
