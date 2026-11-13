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

Start by writing a team definition.

```sh
hergy new myteam
```

Run it in a herdr shell pane.

```sh
hergy up myteam --dry
hergy up myteam
```

When unsure, read `hergy cheat` or `hergy <command> --help`.

## A team definition (team.yaml)

```yaml
version: 1
description: "which case this team is meant for"
team: myteam
cwd: .

prompt: "…"
members:
  - name: lead
    role: lead
    profile: opus
workers:
  - from: lead
    purpose: research
    profile: opus
turns:
  - human -> lead
  - lead -> human
```

## profiles.yaml

```yaml
version: 1
profiles:
  opus:
    kind: claude
    args: ["--model", "claude-opus-5-5"]
  dev:
    kind: opencode
    env:
      OPENCODE_CONFIG_CONTENT: '{"model":"…"}'
```

## What's next

- Show which member holds the turn (`hergy turn`)
- A fixed shape and a record for messages between members (`hergy pass`, `hergy log`)
- List the rules, and take one out to compare (`hergy rules`)
- Receive questions for the human, give instructions by role name (`hergy ask`, `hergy tell`)
- Validate the settings (`hergy validate`)
- Promote and migrate settings (`hergy promote`, `migrate`)
- Nested teams, a TUI

## Contributing

Before you send a change, run `go run ./tools/check diff` once. It runs the
quick checks for the changes since the upstream, in about a minute, and names
the browser walks the change selects (`go run ./tools/check e2e tests/<name>.spec.ts`
runs those alone). CI runs every quality gate on every push, the browser walks
and the race detector among them. After fixing a failed gate, rerun that gate
only, unless the fix affects other gates. `go run ./tools/check -h` lists the checks.

## License

[MIT](LICENSE)
