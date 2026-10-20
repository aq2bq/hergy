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
- Whether anyone is waiting on the human, or nobody is working: `hergy status`. A member whose screen has not changed while it works is marked with how long it has stood.

## Team definitions and team names

- A team definition is the directory `teams/<name>/` with its `team.yaml`; `hergy up`, `hergy new` and `hergy show` take a team definition name.
- `team.yaml` must write `mission:` — one line saying what the team is for. A definition that writes none, or writes one over more than one line, does not read. The commands that handle a team print it at the top of their header, and so do the web UI's team list and team screen.
- A team is a running team, named by `hergy up --as` or, without it, by the team definition name; `hergy down`, `hergy status`, `hergy refresh`, `hergy reprompt`, `hergy hire`, `hergy focus`, `hergy nudge`, `hergy stop` and `hergy tell` take a team name and find the definition from this machine's launch record. The team's herdr tab is labelled with the team's name alone, so a tab a person sees says the team, not the definition it came from; a team booted by an older hergy still has the definition in its tab's label, and is still found.
- `hergy status` with no team name lists every running team, with its team definition name.

## Before you run one

- Prove the plan with `--dry` before `hergy up`, `hergy down`, `hergy refresh` or `hergy hire`; each acts on live panes.
- `hergy reprompt` keeps each member's conversation; `hergy refresh` ends the agent and starts it again, so the conversation starts empty. After editing a document the member reads, `reprompt`; after editing a profile (its `kind`, `args` or `env`), `refresh`. `refresh` also starts a member that is no longer running, when the pane hergy named after it is still at its shell; if the pane is gone or busy, it changes nothing and points at `hergy down` then `hergy up`.
- A member cannot `refresh` itself: hergy runs inside a member's pane, and stopping that member would stop hergy before it could start the agent again. Run it from another pane, or as the human.
- `env` is fixed when a pane is made: after changing `env`, run `hergy down` then `hergy up`; `hergy refresh` keeps the pane and its env.
- `hergy` sets `GY_ACTOR=<herdr name>` in every pane it makes, so an agent's records in gy carry the name herdr knows it by; the bootstrap prompt tells the agent not to change it. A profile that writes `GY_ACTOR` does not win, and `hergy validate` warns when one does.
- `hergy down` closes the panes of the team you are in, your own pane included, and that pane last.
- To read output, add `--lang en`, or use `--dry --json`; the `next:` line names what to type next.

## Reaching a member

- `hergy focus <member>` moves herdr's focus to that member's pane, so a person can answer it there.
- `hergy nudge <member> [text]` sends a short push to carry on, only when the member is idle or done: a working or blocked member is refused, and nothing is sent. With no text it sends a short "carry on". The text is headed by who sent it: the member whose pane you type in, else the human.
- `hergy stop <member>` sends esc, ending the member's current turn and keeping its conversation; a member that is not working is left alone, and the command still succeeds. Some agents need esc twice, the first only asking them to interrupt: hergy watches the member's state after each esc and sends a second only while the member is still working, so an agent that stopped on the first is never sent a second. A member still working after two escs is reported as not stopped (exit 1), and `hergy refresh <member>` is named as the next step — the same step a stop that worked names. A member cannot stop itself: hergy runs in that member's pane.
- `hergy tell <member> "<text>" [--from <member>]` sends a message headed by `--from`, else the member whose pane you type in, else the human. It does not read the member's state, so the message reaches an agent in any state. A `--from` name outside the team is refused.
- All four take a member by short member name or herdr name, and `--team <team>` picks the running team; none needs a herdr-managed pane of its own. `--dry` prints what would be sent and sends nothing (focus has no `--dry`).

## While you are inside a team

- The lead adds workers within its `workers:` (from, purpose and profile) and gives each a short name. `hergy hire --purpose` must be a purpose the hiring member's `workers:` names, and the profile comes from that entry (a hire does not choose one); a purpose outside it is refused, naming the purposes the member may hire for. With no `--from`, it uses the one member that hires for that purpose (two or more stop the hire and are listed).
- `turns:` says who hands the turn to whom (a member's name, or `human`). Each member and the human hand the turn on to at least one party, and at least one turn reaches the human; a definition that does not say so stops hergy the way a file that does not read does, so `up`, `refresh`, `reprompt`, `hire` and `validate` all stop with the same sentences. A rule text that still writes "a → b" is compared with `turns:` and warned about when they disagree.
- `ledger:` says which member may make which write to gy: each entry is `<write> by <member>[, <member>...]`, where `<write>` is the name of a gy write (`req approve`, `decide`, and the like) or `question to human` for a question whose decider is nobody in the team, and `<member>` is a `members[].name` (a worker's name cannot be written). It stops nothing: `hergy validate` warns once for each write a writer of the team (`GY_ACTOR` begins with the team name, or the name the launch record shows) made that the rule gives to other members, and says so in one line when gy cannot be read.
- A worker reports back to the member that hired it when the work is done.

## While you write settings from outside

- Write with `hergy new` (it writes into the project's `.hergy/` inside a git repository, and into `~/.config/hergy/` outside one), edit, `hergy validate`, then show `hergy up --dry` to the person before `hergy up`.
- `hergy validate` counts what the rules carry and then the size of what each member is handed (rules and characters) and of each team definition's rule files, and warns over the provisional limit (20 rules and 6,000 characters per member; 8 files and 20,000 characters per team definition). A warning is not a failure: it names a team to look at, and the exit code stays 0. When a team is over, look for rules that are duplicates, that are no longer needed, or that could be read only when they are needed, before cutting the meaning.
- A rule may also write `in_prompt: false`: it stays in the file and out of every member's prompt. Its id, its origin and its checks are still counted and checked, another rule may still be derived from it, and `hergy validate` says how many rules a member is not handed.
- To write, change or move a team's rules, read the `hergy-rules` skill first: it asks the person the questions one at a time, so they answer and never write YAML.
- Start in the project's `.hergy/`; ask the person before you change `~/.config/hergy/`.

## The web UI

- `hergy serve` serves the screens on 127.0.0.1, prints the URL and opens it in this machine's default browser; it holds the foreground until you stop it. Run it with `--no-open` when nothing should take the person's screen, and hand them the URL yourself instead.
- It answers this machine only. A write is a POST, PUT or DELETE from a page it rendered, carrying that page's token in the `X-Hergy-Token` header, so driving it with curl means reading the token out of a page first. Do not work around it.
- The screens read and write the same files the commands do. `hergy validate` still decides whether the settings hold.
- The band on every screen is four: 活動 is drawn struck through and is not pressable yet, チーム定義, プロファイル and ロール are the three that open, and `/` moves to `/profiles`. The team definitions are at `/teams` (the list, with a row that opens in place and is written where it stands: チームの姿 draws team.yaml as a figure — the human and the members as points, the turns as lines — and a turn is added or taken away by pressing a point or a line, with a card that shows what the change would do to the prompt before it is made; and チームの約束 edits the team's own agreement.yaml with the same part the role screen uses; a tab at `/teams/yaml` keeps every team.yaml and agreement.yaml, the project's and the shared one, as text). The profiles are at `/profiles` (the list, with a row that opens and is written in place, the members' and workers' picks at `/profiles/assign`, and both profiles.yaml files as text at `/profiles/yaml`), and the roles at `/roles` (three tabs: 一覧, where a role's row opens in place and is written with its sections, rules, origins, reasons and checks, beside a live preview of the Markdown a member's prompt will carry; 割り当て, where each team definition's members take a role; and YAML, where a role document stands as text with validate's problems beside it).
- The band carries a dot beside a screen that holds changes no save wrote, and walking to the other screen with the band carries those changes over and puts them back when that screen comes round again.
- The screens the old WebUI had are gone: `/definitions`, `/activity` and `/agreement`, and every path under them, answer 404 with the band on the page, as do `/team`, `/running`, `/rules`, `/diff`, `/bump`, `/figure` and `/panel`. A page that is still there moves instead: `/profiles/<name>`, and `/roles/<role>` with `/new` or `/<rule>` behind it, go to the list with that profile or role opened. What can be written today is a team definition (its team.yaml and its agreement.yaml), a role document and the two profiles.yaml files.
- Opening `hergy serve` lands on `/profiles`.
