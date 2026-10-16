---
name: hergy
description: "Use when you start or close a team with hergy, write a hergy team definition, or work inside a team hergy started. The exact shape of every command and flag is in hergy cheat --ai (one command's page: hergy <command> --help); read it there."
---

# hergy

`hergy` boots a team of AI agents onto herdr from YAML team definitions. It sits one level above herdr: it boots the team from your team definition and hands each member its first prompt, while herdr runs the panes and carries the messages.

The shape of every command and flag lives in `hergy cheat --ai`. Read that before you type one; this file holds only the judgement. `hergy <command> --help` prints one command's page: its flags, each with what it means for that command.

## Choose between hergy and herdr

- Starting a team, adding a worker, closing a team: hergy.
- Reaching a teammate, driving a pane: herdr.
- Whether anyone is waiting on the human, or nobody is working: `hergy status`.

## Team definitions and team names

- A team definition is the directory `teams/<name>/` with its `team.yaml`; `hergy up`, `hergy new` and `hergy show` take a team definition name.
- `team.yaml` may write `mission:` — one line saying what the team is for. The commands that handle a team print it at the top of their header, and so do the web UI's team list and team screen; `hergy validate` warns when a definition writes none.
- A team is a running team, named by `hergy up --as` or, without it, by the team definition name; `hergy down`, `hergy status`, `hergy refresh`, `hergy reprompt` and `hergy hire` take a team name and find the definition from this machine's launch record. The team's herdr tab is labelled with the team's name alone, so a tab a person sees says the team, not the definition it came from; a team booted by an older hergy still has the definition in its tab's label, and is still found.
- `hergy status` with no team name lists every running team, with its team definition name.

## Before you run one

- Prove the plan with `--dry` before `hergy up`, `hergy down`, `hergy refresh` or `hergy hire`; each acts on live panes.
- `hergy reprompt` keeps each member's conversation; `hergy refresh` ends the agent and starts it again, so the conversation starts empty. After editing a document the member reads, `reprompt`; after editing a profile (its `kind`, `args` or `env`), `refresh`. `refresh` also starts a member that is no longer running, when the pane hergy named after it is still at its shell; if the pane is gone or busy, it changes nothing and points at `hergy down` then `hergy up`.
- A member cannot `refresh` itself: hergy runs inside a member's pane, and stopping that member would stop hergy before it could start the agent again. Run it from another pane, or as the human.
- `env` is fixed when a pane is made: after changing `env`, run `hergy down` then `hergy up`; `hergy refresh` keeps the pane and its env.
- `hergy down` closes the panes of the team you are in, your own pane included, and that pane last.
- To read output, add `--lang en`, or use `--dry --json`; the `next:` line names what to type next.

## While you are inside a team

- The lead adds workers within its `may_spawn` (purpose-to-profile pairs) and gives each a short name. With no `--profile`, `hergy hire` uses the profile `may_spawn` pairs with `--purpose`.
- A worker reports back to the member that hired it when the work is done.

## While you write settings from outside

- Write with `hergy new --local` (without `--local` it writes to the global layer), edit, `hergy validate`, then show `hergy up --dry` to the person before `hergy up`.
- `hergy validate` counts what the rules carry and then the size of what each member is handed (rules and characters) and of each team definition's rule files, and warns over the provisional limit (20 rules and 6,000 characters per member; 8 files and 20,000 characters per team definition). A warning is not a failure: it names a team to look at, and the exit code stays 0. When a team is over, look for rules that are duplicates, that are no longer needed, or that could be read only when they are needed, before cutting the meaning.
- To write, change or move a team's rules, read the `hergy-rules` skill first: it asks the person the questions one at a time, so they answer and never write YAML.
- Start at the `--local` layer; ask the person before you change the global layer.

## The web UI

- `hergy serve` serves the screens on 127.0.0.1, prints the URL and opens it in this machine's default browser; it holds the foreground until you stop it. Run it with `--no-open` when nothing should take the person's screen, and hand them the URL yourself instead.
- It answers this machine only. A write is a POST from a page it rendered, carrying that page's token, so driving it with curl means reading the token out of a form first. Do not work around it.
- The screens read and write the same files the commands do. `hergy validate` still decides whether the settings hold.
- The team definition is at `/team`: the documents every member reads, the members, and the workers each member may call, each member's prompt opening in the panel on the right. The running teams are at `/running`: what herdr reports, read the way `hergy status` reads it, a row per member with its terminal title and pane, and the button that moves herdr's focus to that pane — the one write these screens hold; it reads herdr again every 5 seconds. The figure is at `/figure`: one team definition drawn as a picture — the documents, the role files, the members, the workers, the human, and the turn directions read out of the rules — with herdr's state on a team that is running, and a document or a rule id leading to the rules screen.
- The profiles are at `/profiles`: the profile names, kinds, args and env of the layer being shown, each env variable a name and its value (shown as it is: env holds no API key), as a form that adds, changes and deletes them. The save writes that layer's own profiles.yaml through the same check `hergy validate` runs, so nothing is written when the file would not read back, or a team definition that reads it would be left without a profile it names.
- Opening `hergy serve` lands on the home at `/`: the step bar of the daily flow, what moved since the last launch, and the rules with no check. The team list is at `/teams`, the rules screen at `/rules` edits the rule files a layer's team definitions read, the prompt diff at `/diff` shows, member by member, the prompt the last launch handed over against the prompt the files build now, and the version screen at `/bump` raises the version of each document whose content moved. A bump is refused unless the document moved since the launch that read it and its version has not been raised since, so a page left open cannot raise a version twice. Point the person at the home before the next `hergy up`.
