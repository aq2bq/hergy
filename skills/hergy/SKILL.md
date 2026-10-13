---
name: hergy
description: "Use when you start or close a team with hergy, write a hergy team definition, or work inside a team hergy started. The exact shape of every command and flag is in hergy cheat --ai; read it there."
---

# hergy

`hergy` boots a team of AI agents onto herdr from YAML team definitions. It sits one level above herdr: it boots the team from your team definition and hands each member its first prompt, while herdr runs the panes and carries the messages.

The shape of every command and flag lives in `hergy cheat --ai`. Read that before you type one; this file holds only the judgement.

## Choose between hergy and herdr

- Starting a team, adding a worker, closing a team: hergy.
- Reaching a teammate, driving a pane: herdr.
- Whether anyone is waiting on the human, or nobody is working: `hergy status`.

## Team definitions and team names

- A team definition is the directory `teams/<name>/` with its `team.yaml`; `hergy up`, `hergy new` and `hergy show` take a team definition name.
- A team is a running team, named by `hergy up --as` or, without it, by the team definition name; `hergy down`, `hergy status`, `hergy refresh`, `hergy reprompt` and `hergy hire` take a team name and find the definition from the team's herdr tab.
- `hergy status` with no team name lists every running team, with its team definition name.

## Before you run one

- Prove the plan with `--dry` before `hergy up`, `hergy down`, `hergy refresh` or `hergy hire`; each acts on live panes.
- `hergy reprompt` keeps each member's conversation; `hergy refresh` ends the agent and starts it again, so the conversation starts empty. After editing a document the member reads, `reprompt`; after editing `kind`, `args` or `profile`, `refresh`. `refresh` also starts a member that is no longer running, when the pane hergy named after it is still at its shell; if the pane is gone or busy, it changes nothing and points at `hergy down` then `hergy up`.
- A member cannot `refresh` itself: hergy runs inside a member's pane, and stopping that member would stop hergy before it could start the agent again. Run it from another pane, or as the human.
- `env` is fixed when a pane is made: after changing `env`, run `hergy down` then `hergy up`; `hergy refresh` keeps the pane and its env.
- `hergy down` closes the panes of the team you are in, your own pane included, and that pane last.
- To read output, add `--lang en`, or use `--dry --json`; the `next:` line names what to type next.

## While you are inside a team

- The lead adds workers within its `may_spawn` (purpose-to-profile pairs) and gives each a short name. With no `--profile`, `hergy hire` uses the profile `may_spawn` pairs with `--purpose`.
- A worker reports back to the member that hired it when the work is done.

## While you write settings from outside

- Write with `hergy new`, edit, `hergy validate`, then show `hergy up --dry` to the person before `hergy up`.
- Start at the `--local` layer; ask the person before you change the global layer.
