---
name: hergy
description: "Use when you start or close a team with hergy, write hergy settings, or work inside a team hergy started. The exact shape of every command and flag is in hergy cheat --ai; read it there."
---

# hergy

`hergy` boots a team of AI agents onto herdr from YAML settings. It sits one level above herdr: it boots the team from your settings and hands each member its first prompt, while herdr runs the panes and carries the messages.

The shape of every command and flag lives in `hergy cheat --ai`. Read that before you type one; this file holds only the judgement.

## Choose between hergy and herdr

- Starting a team, adding a worker, closing a team: hergy.
- Reaching a teammate, driving a pane, checking status: herdr.

## Before you run one

- Prove the plan with `--dry` before `hergy up`, `hergy down` or `hergy hire`; each acts on live panes.
- `hergy down` closes the panes of the team you are in, your own pane included, and that pane last.
- To read output, add `--lang en`, or use `--dry --json`; the `next:` line names what to type next.

## While you are inside a team

- The lead adds workers within its `may_spawn` and gives each a short name.
- A worker reports back to the member that hired it when the work is done.

## While you write settings from outside

- Write with `hergy new`, edit, `hergy validate`, then show `hergy up --dry` to the person before `hergy up`.
- Start at the `--local` layer; ask the person before you change the global layer.
