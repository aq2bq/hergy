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
- `team.yaml` must write `mission:` — one line saying what the team is for. A definition that writes none, or writes one over more than one line, does not read. The commands that handle a team print it at the top of their header, and so do the web UI's team list and team screen.
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

- The lead adds workers within its `workers:` (from, purpose and profile) and gives each a short name. `hergy hire --purpose` must be a purpose the hiring member's `workers:` names, and the profile comes from that entry (a hire does not choose one); a purpose outside it is refused, naming the purposes the member may hire for. With no `--from`, it uses the one member that hires for that purpose (two or more stop the hire and are listed).
- `turns:` says who hands the turn to whom (a member's name, or `human`). Each member and the human hand the turn on to at least one party, and at least one turn reaches the human; a definition that does not say so stops hergy the way a file that does not read does, so `up`, `refresh`, `reprompt`, `hire` and `validate` all stop with the same sentences. A rule text that still writes "a → b" is compared with `turns:` and warned about when they disagree.
- A worker reports back to the member that hired it when the work is done.

## While you write settings from outside

- Write with `hergy new` (it writes into the project's `.hergy/` inside a git repository, and into `~/.config/hergy/` outside one), edit, `hergy validate`, then show `hergy up --dry` to the person before `hergy up`.
- `hergy validate` counts what the rules carry and then the size of what each member is handed (rules and characters) and of each team definition's rule files, and warns over the provisional limit (20 rules and 6,000 characters per member; 8 files and 20,000 characters per team definition). A warning is not a failure: it names a team to look at, and the exit code stays 0. When a team is over, look for rules that are duplicates, that are no longer needed, or that could be read only when they are needed, before cutting the meaning.
- To write, change or move a team's rules, read the `hergy-rules` skill first: it asks the person the questions one at a time, so they answer and never write YAML.
- Start in the project's `.hergy/`; ask the person before you change `~/.config/hergy/`.

## The web UI

- `hergy serve` serves the screens on 127.0.0.1, prints the URL and opens it in this machine's default browser; it holds the foreground until you stop it. Run it with `--no-open` when nothing should take the person's screen, and hand them the URL yourself instead.
- It answers this machine only. A write is a POST from a page it rendered, carrying that page's token, so driving it with curl means reading the token out of a form first. Do not work around it.
- The screens read and write the same files the commands do. `hergy validate` still decides whether the settings hold.
- The band on every screen is four: 活動 and チーム定義 are drawn struck through and are not pressable yet, プロファイル and ロール are the two that open, and `/` moves to `/profiles`. The profiles are at `/profiles` (the list, with a row that opens and is written in place, the members' and workers' picks at `/profiles/assign`, and both profiles.yaml files as text at `/profiles/yaml`), and the roles at `/roles` (three tabs: 一覧, where a role's row opens in place and is written with its sections, rules, origins, reasons and checks, beside a live preview of the Markdown a member's prompt will carry; 割り当て, where each team definition's members take a role; and YAML, where a role document stands as text with validate's problems beside it).
- The band carries a dot beside a screen that holds changes no save wrote, and walking to the other screen with the band carries those changes over and puts them back when that screen comes round again.
- The screens the old WebUI had are gone: `/definitions`, `/activity` and `/agreement`, and every path under them, answer 404 with the band on the page, as do `/teams`, `/team`, `/running`, `/rules`, `/diff`, `/bump`, `/figure`, `/panel` and the old `/roles/<role>` pages. What can be written today is a role document and the two profiles.yaml files.
- Opening `hergy serve` lands on `/profiles`.
