# Changelog

## 0.9.3 - 2026-10-29

### Changed

- **`hergy serve` answers without waiting**: the screens' answers took 0.4–0.7 s,
  and a page up to 1.3 s on a few projects with a large team (n-9be4). serve now
  keeps what it read of the definitions under the version of their files (size
  and time of change) and reads again only a file that changed, by hergy or by
  hand, so the next answer always carries the change; the checks and the prompt
  counts are worked out once per version. herdr's state is read in one call over
  herdr's own socket instead of four `herdr` commands one after another, and the
  five seconds serve used to remember herdr's state are gone. Measured on three
  projects with a 13-agent team: every API answer is under 25 ms (was up to
  650 ms). A serve started before herdr finds herdr's socket on its next read.
  The commands that change herdr (starting, sending, closing) still run `herdr`.
- **The activity figure shows whose worker each is**: workers stand under the
  member that hired them, as a tree — a line down from the member and a branch to
  each worker, the name right of its dot — so no line crosses another member's
  column, and the turns between members run above the members (n-b82e).

## 0.9.2 - 2026-10-29

### Breaking

- **Rule ids are hergy's**: every rule id is now four characters — one uppercase
  letter and three uppercase letters or digits, without I, L, O and U (for
  example `K7QF`), with no meaning. People and screens write none: a rule is
  written without an `id`, and the next write gives it one and writes it back,
  one line saying how many it gave (d-4aab, d-a873). The writes are `hergy new`,
  `up`, `apply`, `reprompt`, `refresh` and `hire` (never with `--dry`) and a save
  on the screens; the screens show 保存で付く for a rule until it is saved, and
  an id cannot be edited there. An id in the older shape (`LEAD-1`, `TEAM-TURN-1`)
  is given a new one the same way, in the project's and the global settings at
  once, and a rule's text that names the old id is rewritten to the new one. An
  id hergy gave is never changed, and an id of a rule that was taken out is
  never used again. Only the `id:` lines change: comments and other lines stay.
- **The first apply after this release resends every rule**: the ids that were
  written by hand (or by the screen) are given hergy's ids once. A running team
  then reads every rule as taken out and added, so the first `hergy apply` (or
  the screens' apply) sends its whole prompt again to every member that has
  drift. Choose a moment when the team is waiting; the conversation stays
  (d-a873).
- **Reading no longer refuses a rule without an id**: `hergy validate`, `--dry`,
  `hergy status` and the screens read it, and `validate` counts the rules the
  next write will give an id.

### Moving from 0.9.1

1. Before the first write, run `hergy validate`: it says how many rules hergy has
   not given an id yet. Every rule whose id was written by hand is among them.
2. Wait until the running teams are waiting, then run `hergy apply` (a `--dry`
   run reads only and gives no ids). hergy first gives the ids in the project's
   and the global settings, then sends the prompt once more to each member with
   drift.
3. The global roles' new ids reach the other projects' teams too: run
   `hergy apply` in each of those projects as well.
4. If you wrote the old ids anywhere outside the rule files (notes, gy, scripts),
   look them up again: hergy rewrites only the rule files.

## 0.9.1 - 2026-10-29

### Breaking

- **Adding a member to a running team is `hergy apply`**: `hergy refresh` restarts
  members that have a live agent, and a member with no pane is refused with the
  way to `hergy apply` (d-36ef, which replaces the adding refresh of 0.9.0,
  d-8fcf). The screens' 起動 for such a member is gone the same way.
- **The web API's change roads are one**: `/api/delivery`, `/api/delivery/count`,
  `POST /api/delivery` and `POST /api/hq/refresh` are gone; `GET /api/apply`,
  `GET /api/apply/count` and `POST /api/apply` take their place. Every answer and
  body now has one name and one shape per idea (n-3f6c), the opened team's
  `GET /api/activity/{team}` no longer carries the pane's last lines or the gy
  questions, and the whole API is written in `internal/apicontract/openapi.yaml`,
  which the tests hold every answer to.

### Added

- **`hergy apply`, one way to put a running team right**: hergy compares the saved
  definition with what each member was started from and decides, member by
  member, how to apply the drift — 送り直す (only the prompt changed; the
  conversation stays), 起動する (a member the definition has and no agent runs
  for), 起動し直す (the profile's way of starting changed; the conversation is
  lost, and hergy says so), or 反映できない with the reason (an env change, a
  member taken out of the definition, a definition that cannot be built).
  `hergy apply [member]... [--team] [--dry] [--json] [--timeout]`; with no member
  it applies every member with drift, and a member that is working or waiting
  for a permission is left for later while the rest go on (d-36ef, d-3d30). A
  changed worker profile is a note on the team: it reaches the next worker hired.
- **A resend says what changed**: the prompt sent again opens with the changes —
  the rules, roles, lineup and turns that differ from the launch — and asks the
  member to follow it in place of the old one, keep its work and conversation,
  and not redo its start-up steps. `hergy reprompt` adds the same lead when
  there is a difference.
- **The WebUI applies too**: the band reads 反映していない変更 N, and the press
  opens a sheet with each member's way of applying, what changed and whether the
  conversation is lost; the members ticked are applied the way `hergy apply`
  does, with the progress and any start-up question in the same sheet.

### Changed

- **The activity screen is two tiers**: the rows of what waits for a human and
  the teams' cards. The panel under them — a second figure, the pane's last
  lines and a member list — is gone, and each card carries the team's own
  チーム定義を開く, 反映していない変更 N (when there is some) and
  チームを終了する…. A waiting row's press selects its team's card, and a
  node's description in the figure names what herdr says the agent is doing
  (n-34e7).
- **The activity figure stays readable for a large team**: names are measured and
  the workers stand in as many rows as they need, so no two names overlap; the
  counts under a card name the members and the workers apart (r-8b04).
- **Words**: activity is アクティビティ (was 活動), and closing a team is
  終了する (was 止める on the screens; 止める is only `hergy stop`'s, a member's
  turn). Applying a changed definition is 反映, with 送り直す / 起動する /
  起動し直す; 再投入 and 変更を伝える are gone. The glossary bans the old words.
- **A definition that cannot be built is an error of the definition**: `/api/team/check`
  and the list's problems name it (a member whose role is not a role name), and
  the 動き column says why a running team cannot be acted on (r-fc16).
- **The assignment tabs write the definition where its card stands**: a
  definition only in the global settings is written there, and the card says
  that a global one changes it for every project (r-f8b9, d-0f32).
- The screens are written in TypeScript and built into the binary; nothing a
  person does changes with it (n-893d).

### Fixed

- **Two running teams of the same name in two projects are told apart**: each is
  read from its own project's launch record, not from the newest record of that
  name (r-87c8).
- **Two messages printed `%!s(MISSING)`**: `hergy new --local`'s answer about the
  gone layer flag, and the message that names the member to refresh when an agent
  does not go away (n-60fd, r-bac8). Every message is now checked against the
  values its call passes.
- **元に戻す on a team with no agreement file no longer breaks the row**: it puts
  back the empty document the row opened with (n-e3de).
- **A workspace with no name leaves no empty mark**: a card's heading no longer
  ends in `·`, and the 終了する sheet shows no empty ワークスペース line (n-55b8).
- `hergy refresh --help` no longer says it adds a member.

### Moving from 0.9.0

1. Where you added a member with `hergy refresh <member>`, write the member in
   `team.yaml` and run `hergy apply` (or `hergy apply --dry` first to see what
   it will do).
2. A team started before 0.9.1 has a launch record without the fingerprints of
   how each member was started. `hergy apply` still sends the prompt again where
   the prompt changed, but says it cannot tell a profile change for that team.
   Start the team again (`hergy down`, then `hergy up`) when you want profile
   changes found too.
3. A script that called `/api/delivery` or `/api/hq/refresh` calls `/api/apply`
   now; `internal/apicontract/openapi.yaml` has the shapes.

## 0.9.0 - 2026-10-28

### Fixed

- **変更を伝える counts every listed project's teams**: the band and its sheet read
  the launch record of the project serve was started in, so a team of another
  project listed in the headquarters found no record to compare and read as none
  of its members 伝えていない — a changed role was never offered to be sent
  (n-a1a4, r-2e49, ac-9cab). Each running team is now counted against its own
  project's record, and a send reaches it in that project.
- **A member's name typed in the figure does not take another member's name**:
  while the typed name is one another member already holds, the figure's name
  field no longer puts a rename into the draft — the letters stay in the field
  and このチームにもういます stands beside it. Before, typing `spec-2` beside a
  `spec` renamed the new member to `spec` at the fifth letter, and the next
  keystroke's rename found the original `spec` by name and rewrote that member's
  name, its turns and its workers in the unsaved draft (ac-5edb, r-4fd7, n-4b24).
- **A role renamed in the role screen rewrites the teams that name it**: the
  screen sent the mark that carries a rename under the name `renameUsers`, where
  the API reads `rename_users`, so the mark arrived off every time and a renamed
  role's old name stayed in the team definitions — `validate` and `up` then
  stopped on them until a person rewrote them by hand (n-b06c, r-b0bc, ac-2abf).
  The mark travels under the field the API reads now, the same one the profile
  screen sends, and a test holds both the screen's body and the server's rewrite.
- **`hergy down` closes what hergy made for a team, even with no agent running**:
  a boot that stopped at a startup question (Ctrl-C, or an agent told not to trust
  the folder) left hergy's tab and panes behind, and no live agent made the team a
  running one, so `hergy down <team>` answered `no running team "<team>"` and the
  panes stayed (ac-1b17, d-da92). Down now finds a team by what hergy wrote into
  herdr: the tab labelled with the team's name, and the panes in it (and in its
  worker tabs) named `<team>-<member>`, whether an agent stands in them or not. A
  pane hergy did not name is a person's own and is left open. The way on the
  startup messages name — `hergy down` and then the same `hergy up` — now works
  the first time. A pane it closes with no agent in it says so, and the counts tell
  a pane standing without an agent from a member that has no pane at all.
- **A message to a teammate arrives as written**: the initial prompt taught
  `herdr agent prompt <name> "<text>"`, so a backquote or `$( )` in the text was
  run by the sending member's shell — the message arrived with holes, and a
  command ran on the sender's machine (ac-ba20, d-e66d). The prompt now teaches
  reading the text into a variable through a quoted here-document
  (`IFS= read -r -d '' body <<'EOF'`) and sending `"$body"`; the shell expands
  nothing in it, in bash (3.2 and 5), zsh and sh alike. Members of a team started
  before this change are told the old way until their prompt is handed over again.

### Added

- **A member can be copied in the figure**: the member's card carries 複製, and
  the press adds the same member again — its role, profile, turns in both
  directions and its worker purposes — under the next free name (`<name>-2`, then
  `-3`, `-4`, … that fits the herdr name). The copy stands picked with its name
  field waiting, nothing is written until the save, and one undo takes the whole
  copy back (ac-ef85, d-48a9, r-c21b).
- **The screens start a member the team definition has and no agent is live
  for**: the 動き column of the teams screen and the opened team on the activity
  screen name those members and offer 起動, and the press starts the member in
  the team's tab — the same launcher refresh the command line runs, so it reads
  the rules it would get from `hergy up` and the start's own sheet shows the
  progress and any question it asks while it starts (ac-d090, r-5d1a).
  `GET /api/hq/running` names them under `not_up`, and `POST /api/hq/refresh`
  answers a start's own state, read at `/api/hq/launches/<id>`. A member that is
  live is refused (starting it again would end its conversation), as is a team
  that is not running. The members already running are told the new lineup with
  変更を伝える.
- **`hergy refresh` adds a member the definition has, without a down and up**:
  write the member in `team.yaml` and run `hergy refresh <member>`, and hergy
  makes a pane for it in the running team's own tab and starts it there, the way
  `hergy up` starts a member: the same bootstrap prompt, the same wait on a
  question while it starts, and the same `--timeout`. The members already running
  keep their conversation and are not touched; with `--all`, the live members
  restart as before and the members with no pane are added (ac-bb45, d-8fcf,
  r-7e09). The last line then names `hergy reprompt --all --team <team>`, which
  hands the new lineup to the members that were already running. `--dry` names the
  panes it would make, and `--json` carries them under `new_panes`. A team that is
  not running has no tab to add a member to: refresh refuses and names `hergy up`,
  as before.
- **The headquarters screens**: the band names the number of listed projects and
  opens the sheet that lists and unlists them (a project the launch record knows
  is offered as a candidate), and its Find field (`/`) looks through the running
  teams, team definitions, workspaces, members, roles, rules and profiles of every
  listed project and the global settings, each row saying where it stands and
  going to it. The three lists stand in groups — each project's own in the
  config's order, then the global layer once — with a プロジェクト column, and
  another project's row opens and saves in place (two projects' rows can stand
  open at once). The team definitions' Running column starts and stops a listed
  project's team: the start sheet asks for the workspace in the order `hergy up`
  uses (the one the team last ran in is picked, one with a team cannot be), shows
  the start's steps and a member's startup question with where to answer, and the
  stop leaves the workspace open. The activity screen is three tiers: the members
  waiting for the human and the stalled teams with where to answer, one figure per
  running team (the human, members and workers as points, turns as lines, and a
  look for each state), and the opened team's last screen, members and workers,
  with a member's one line of what it is doing being the terminal title herdr
  reports, unchanged (d-b4b9). Without `projects:` the lists stay the one project
  they were, and the band offers to list more.

- **`hergy hire` keeps who added the worker**: the launch record gains one line
  per hire, naming the team, the worker, the member that added it (the short name
  `--from` resolves to), the purpose and the tab (d-b4b9). The activity screen
  reads it to say "`<member>` が追加" for a worker, and says nothing where a
  worker was hired before those lines existed. The worker's tab label and
  `hergy status` print what they did.

- **`hergy serve` is a headquarters for the projects a machine lists**: the
  global `config.yaml` takes a `projects:` list of directories (absolute, or
  `~/…`), and every listed project's team definitions, roles, profiles and
  running teams show with the project they belong to, wherever serve is started
  (d-73fc). Without the key serve shows the project it was started in, as before;
  an unlisted project started in says so and shows none of its teams. The screens
  list and unlist projects, writing the list back as `~/…` where the home is
  (unlisting changes no file of that project), and a team definition's Running
  column starts and stops a listed project's team in a chosen herdr workspace —
  one team to a workspace — with the same tab, members, prompts, launch record
  and stop as `hergy up` and `hergy down`. A start is a request that returns at
  once; its progress and a member's startup question are read from
  `/api/hq/launches/<id>` and can be given up there. The launch record now keeps
  the workspace a team was started in, so the start sheet offers the workspace it
  last ran in. The activity screen reads every listed project too, and its
  judgement is the one the headquarters' running teams come from. A team is now
  also found by the panes hergy named `<team>-<member>` when no agent runs there
  (d-da92), so a team stopped at a startup question is listed and can be stopped.
  The same reading answers under `/api/hq/`: projects (list, add, remove), teams,
  roles and profiles grouped by project, running, workspaces, launch options,
  launch, launches/<id> and stop. Every existing `/api/` read and write takes an
  optional `project` to read or write that listed project's `.hergy/`.

- **A member that asks a question while it starts is waited on**: when herdr
  refuses a start because the agent is blocked during startup — Claude Code
  asking whether to trust a new folder, and then whether to import a parent
  folder's `CLAUDE.md` — `hergy up`, `hergy hire` and `hergy refresh` no longer
  stop with the raw `agent_not_ready` reply. hergy names the member, the tab, the
  pane and the kind, and waits with no limit while the person answers in that
  pane; a second question in a row prints nothing more. Once the answer is in, it
  says so, submits the bootstrap prompt and goes on, so a `down` and another `up`
  are no longer needed. Ctrl-C stops the wait: the member's state is written as
  waiting for permission (`許可待ち`), and the last line says how to go on. An
  agent that ends after its question (it was told not to trust the folder) stops
  with an error that names the same way on. hergy answers nothing itself, writes
  no agent setting and moves no focus; every other start failure reads as before.
  The state is `許可待ち` in Japanese and "waiting for permission" in English, and
  it is in the glossary.

## 0.8.2 - 2026-10-23

### Breaking

- **A rule holds only its body, its on/off switch, the day it was switched off
  and a note**: `why`, `send_why`, `origin`, `report` and `tests` are gone from a
  rule, and the document-level `reports:` block is gone. A rule is now
  `id`, `text`, `in_prompt`, `off_since` (`YYYY-MM-DD`, only while the rule is
  off) and `note` (the memo for people, never sent). A file that still writes an
  old field stops rather than dropping it: `hergy validate`, `hergy up` (with
  and without `--dry`), `hergy refresh`, `hergy reprompt`, `hergy hire` and the
  screens name the file and the rule, with what to do with the content. The web
  API's rule object drops `why`, `send_why`, `origin`, `report` and `tests`, and
  carries `off_since` and `note`; a document has no `reports`.
- **The agreement every team shares is gone**: hergy no longer reads
  `agreement.yaml` at the top of a place — `.hergy/agreement.yaml`, or
  `~/.config/hergy/agreement.yaml` for the settings outside the project. Its
  rules reached every member of every team of that place; hergy now reads only a
  team's own agreement (`teams/<definition>/agreement.yaml`) and the role
  documents (`roles/<role>.yaml`).
  While the file is still there, hergy stops rather than silently dropping its
  rules: `hergy up` (with and without `--dry`), `hergy refresh`, `hergy reprompt`
  and `hergy hire` stop with exit 1, name the file and say where the rules go,
  and `hergy validate` names it as a broken file (also exit 1). The team
  definitions screen and the role screen no longer count or show it: the size
  each definition is held to, and the totals `hergy validate` prints, leave it
  out.

### Moving from 0.8.1

1. Open `agreement.yaml` at the top of `.hergy/` (and at
   `~/.config/hergy/agreement.yaml` if that file is there). Decide where each
   rule goes now:
   - a rule about how one team works — its turns, its review, its checks, its
     ledger — goes into that team's `teams/<definition>/agreement.yaml`;
   - a rule every member of one role must follow goes into that role's document,
     `roles/<role>.yaml`;
   - a rule every team read but none of them owns: write it into the agreements
     of the teams that must still read it, or into the role documents that need
     it. hergy does not copy it for you: one rule in four files would make it
     unclear which copy to keep.
2. Remove the file.
3. Run `hergy validate`. It must no longer name the file, and the amounts and the
   warnings must be counted from the files that are left.
4. Run `hergy up <team> --dry` and check that the rules you moved stand in the
   members' prompts (by their ids). A rule left in no file is in no prompt.

### Moving a rule file to the new shape

`hergy validate` names each old field it finds, the file, the rule id, and where
the content goes. For each one:

1. A `why` a member read (no `send_why: false`) moves to the end of `text`: the
   body is what a member reads, so the prompt keeps the sentence. A `why` with
   `send_why: false` was not sent; move it to `note`.
2. An `origin` moves to `note`. It was a record for people, not a sentence a
   member read.
3. `report`, `tests` and a document's `reports:` are deleted, not moved to the
   note: the per-rule check fields are gone.
4. Keep `in_prompt` as it is. A rule that is off may also carry `off_since:
   YYYY-MM-DD`; the screen writes the day when a person switches the rule off
   and removes it when they switch it back on. A hand-written `in_prompt: false`
   without a day is read as off on an unknown day and is not filled in on its
   own. An `off_since` left on a rule that is on is ignored and is dropped the
   next time hergy writes that rule.
5. Remove `why`, `send_why`, `origin`, `report`, `tests` and `reports:` and run
   `hergy validate` again. It must stop naming them.

### Added

- **The topology tab is a canvas**: select a member or a worker to change its
  role and profile, its turns and its workers in the column beside the figure;
  draw a turn from a node's edge to another node; double-click the empty canvas,
  or press "+ ワーカー" with nothing selected, to add a member or a worker there;
  remove with Delete and take an operation back with ⌘Z ("取り消す"). A member
  just added can be saved before it hands the turn to anyone; it stays the
  definition's problem until a turn is drawn.
- **Every line a member is handed can be read on a screen**: a member's column on
  the canvas shows the lines this team definition hands it (the template, the
  roster, the turns, the workers and hergy's contact text) and folds the role
  document and the team's rules into a line each that opens them on their own
  screens. Together they are `hergy up --dry`'s prompt, character for
  character; no screen lays the whole prompt out in one place.
- **Hand a changed definition to the members already running**: a button beside
  the screens' tabs counts the running members whose last prompt differs from
  what the definition builds now. It opens a sheet that lists them by running
  team with their state and what changed for them (a rule, a turn, a worker, a
  document); the waiting ones can be picked and sent the new prompt, the way
  `hergy reprompt` does. Saving sends nothing. The activity screen marks a
  member whose definition changed.

### Changed

- **`reprompt`, `nudge`, `stop` and `tell` plan without herdr**: their `--dry`
  now builds the plan from the team definition and this project's launch
  record, like `hire --dry`, instead of reading herdr's live agents and tabs.
  A team that is not running can be planned for; name it with `--team` (a team
  name, or a team definition name the launch record does not know), and with no
  `--team` the one team in the launch record is used. A `--dry` without a team
  and without a record asks for `--team`. The real runs are unchanged, and
  `down --dry` and `refresh --dry` still read the live state they check.
- **A bootstrap prompt now says that sending a message hands the turn**: the
  contact guide (`herdr.guide_member` and `herdr.guide_worker`) says that
  sending to a teammate hands them the turn and that writing in one's own pane
  and stopping hands the turn to the human; that the sender stops and waits and
  promises no work after handing over; that a member holding the turn does not
  stop until the work it was given is done; and that the receiver holds the
  turn and hands on the next work or closes it. The three lines are in Japanese
  and English and reach every member's prompt and every hired worker's prompt
  (r-e54b, ac-a516).
- **`hergy validate` and the screens check one definition at a time**: a role
  document is checked once, by itself, and a team definition once, with the
  documents its members read together. A role document's own warnings (a model
  name in a rule, its size) no longer appear under every team that uses it; a
  turn or an id that only a team's documents make wrong stays on the team
  definition, naming the role document and the rule. The role screen shows only
  that role document's own problems; a team definition's screen shows the
  combination beside the role document it came from, with a link to that
  document. `hergy validate` prints the same units.
- **The size warning is each definition's own**: 3,000 characters for a team
  definition (the part of its rules that goes into the prompt and the words of
  its templates) and 3,000 for a role document (the part that goes into the
  prompt), both provisional. The per-member rule and character totals and the
  team's source and file-count limits are gone. The team definitions list and
  the role list have a 大きさ column, and `hergy validate` ends with a 大きさ
  list of the effective definitions (a definition whose check stops still shows
  its size; a hidden definition is left out).
- **The role screen shows who uses a role, not how much each is handed**: the
  読んでいる人 column is now 使っているチーム (the teams and members by name
  only), the turn marks under a rule are gone (a turn a team does not hold is
  the team definition's problem), and the assignment tab's team header names
  only the team and its place.
- **The screens' tabs read Activity · Team definitions · Roles · Profiles**:
  profiles, the screen opened least, come last.
- **The place outside the project is called global**: `~/.config/hergy` is
  "グローバル" (no longer "共通") on the screens' labels and the "グローバルにコピー"
  button, in `hergy cheat`, README.ja and the hergy skill.
- **hergy gives a new rule its id**: "+ 規則" on a role or a team's agreement
  numbers the rule after the document's last id (LEAD-9 is followed by
  LEAD-10); in a document with no rule yet the prefix comes from the document's
  name. The id box is gone, and the cursor lands in the rule's text.

### Fixed

- **`hergy validate` does not warn about a definition or a role document that is
  not used**: the project's own hides the global one, so validate says only that
  it is not used (and shows what stops it when it is broken); warnings a person
  cannot fix there no longer bury the ones they can.
- **The team definition row's badge counted a rule twice**: the row, the
  validate column and the YAML head now count the same one check.
- **The YAML tab's file list hid the warning count when a file had an error**:
  it now shows both the problems and the warnings, as the opened YAML head does.
- **The assignment tab wrote a global team's path as the project's**: it now
  names the place the team stands in.
- **`hergy validate` names the role document at a combination warning**: a
  warning from a role document's rule now reads "writer.yaml の規則 WRITE-7
  が…", so a person knows which document to fix.
- **"+ ワーカー" on a member's card keeps its size**: the button spread across
  the card and the browser logged a Content-Security-Policy error when the card
  opened.
- **The team definitions screen uses the same fonts as the others**: it was the
  one screen drawn in the system's fallback fonts.
- **The team definitions' YAML tab says what to do when there is none**: with no
  team definition it said only "pick a file"; it now says how to add one and
  links back to the list. The roles' YAML tab does the same when there is no
  role document.

- **`hire --dry` plans for a team that is not running**: the plan and the
  worker's prompt are built from the team definition, and the command reads no
  herdr, as the cheatsheet said. `--team` names the team (a team name, or a team
  definition name the launch record does not know); without it the one team in
  this project's launch record is used, and with no record the command asks for
  `--team` instead of guessing. A real `hire` still stops on a team that is not
  running.

- **Error messages say what to do next**: every error a command or a screen
  prints now names the next step — the command to run, the screen to open or
  the place in the file to fix. A sentence that stops because of a bug in hergy
  says so, says that no file changed, and names the way around it.

- **A section with no rules saves from the rules screen**: taking a section's
  last rule out leaves a heading with no `rules:` key, and saving that
  agreement or role document from the screen was refused as unreadable even
  when nothing was changed. A section the draft leaves with no rule is written
  without the key, the way hergy writes it.

## 0.8.1 - 2026-10-21

A section's rules are laid out one way now: a bullet list with the id first.
The `style:` key is gone from a rule file, so what a member's prompt carries no
longer depends on a key. A rule can now also stay out of the prompt by itself:
`in_prompt: false` keeps it in the file and out of what a member is handed.

### Added

- **The team-definition screen (`/teams`)**: the band's チーム定義 leads to the
  screen now (it was a struck-through name). A row opens in place: its mission is
  edited where it stands, and the open row has the two tabs トポロジー and
  ルール. トポロジー draws team.yaml as a figure — the human and the
  members as points, the turns as lines — and lets a turn be added or taken away
  by pressing a point or a line; a card beside it shows what the change would do
  to the prompt before it is made, and the checks are the ones `hergy validate`
  prints. ルール edits the team's own agreement.yaml with the same shared
  part the role screen uses (the sections, the rules, the `in_prompt` switch, the
  reordering, the Markdown box), and a save writes the one line that changed. A
  YAML tab keeps every team.yaml and agreement.yaml of both layers. A name the
  project and the shared layer both hold shows the shared one as hidden, and the
  band carries the team-definition count.
- **The team screen makes and unmakes definitions (版 7)**: `＋ チーム定義を追加`
  stands above the list and lays down the same starter `hergy new` writes; the
  place is picked first (`プロジェクト` or `共通`), and the name it is given is
  checked while it is typed — the shape, a definition already standing there, none
  written — with the files a save would write named beside the save (the
  definition's own, the role documents and the starter profile the place lacks).
  A row carries `複製` (the next free `<name>-2`, `-3` …), a copy into the other
  place (`プロジェクトにコピー` / `共通にコピー`, offered while that place has none
  of the name), and `削除`, which takes the definition's team.yaml and
  agreement.yaml away, naming both before it does and leaving the folder when
  anything else is in it. The topology tab is two columns: the figure with the
  checks beside the team.yaml the row would write, line by line, with the lines
  one edit changed marked and the line the picked point, turn, worker or mission
  stands for picked out — so a change reads as YAML while it is made, and its
  行数 and `+N −M` are the save's own. At 1100 or under, an `編集` / `YAML`
  switch shows one column at a time, and pressing a line picks what it stands for
  in the figure.
- **A rule can stay out of the prompt**: `in_prompt: false` keeps the rule in its
  file and out of every member's prompt — neither the one `hergy up` sends nor
  the one `hergy reprompt` rebuilds — and out of the amount counted for that
  member, its rules and its characters. The rule is a rule still: its id cannot
  stand twice, another rule may still be derived from it, and its origin, its
  reason and its check are counted and checked. `hergy validate` says how many
  rules are not handed over (`not in the prompt N`) beside each member and each
  team definition. A file that leaves the key unsaid hands the rule over, so
  files that do not use it are unchanged, and a save through the role screen
  keeps the key. `hergy cheat --ai` and the `hergy` skill name it.
- **The role screen's rule document (版 7.1)**: the part of a screen that edits a
  rule document — the title and revision, the sections, the rules, a rule's own
  `in_prompt` switch and its reason's `send_why` switch, the heading depth, the
  reordering and the Markdown box a member reads — is one shared part now, so the
  team screen can mount the same one. The role screen draws what 版 7.1 agreed: a
  rule's `プロンプトに含める / 含めない` switch under the field `規則` (the
  excluded rule dims with a dashed `含めない` mark, the section's head and the
  list's 規則 column count it, the reader's amount falls by the rule, and the
  Markdown draws it struck through with no line number), the why's own switch
  under `なぜ` (its reason struck through, the rule itself still in the prompt),
  the `##` mark at a section's head opening `##` / `###` / `####`, a section's
  ↑ ↓ moving the whole section, and a rule dragged by its ⋮ handle. The
  Markdown box shows the reason a rule holds beside it, and a changed line is
  marked without the server's answer being mistaken for the text before it. A
  save writes the one line that changed and leaves every other line of the file
  as it was; a move writes the same lines in another order.
- **The activity screen (`/activity`)**: the band's 活動 is a link now (it was a
  struck-through name) and carries the two numbers a person watches — 活動 N is
  how many teams of this project are running and 待ち N how many of them wait
  for a human or a permission, neither counted while herdr cannot be read or no
  team is running. A row is one running team as it is doing: 許可待ち /
  人間の番 / 止まっている / 仕事中, who the row is about (the member the line
  stands with, or the members working, with the workers below ワーカー), and how
  long it has been that way with the moment it began — `以上` and より前 where
  only a bound is known, and nothing at all where a team is working. A row opens
  under itself: the newest question the row's own member asked, with its
  options, then one line each for the older ones, `ほか N 件` letting the rest
  out, and 画面の最後, the last lines of that member's pane, with `▸ 上に N 行`
  letting the lines held back out and `▾ 上の行をしまう` putting them back.
  Beside them stands how the team is doing: every member with 人間に渡した /
  止まっている / 仕事中 and when each stopped, newest stop first, and the
  workers under ワーカー. The screen reads itself again every ten seconds
  without moving the cursor, the scroll, the open rows or the let-out panes; it
  stands at 2000, 1440 and 500 px in both lights; and it is one a person reads
  alone — its three writes answer 405 with the read-only sentence. Every
  length, every moment and every 何分前 is the server's, so the screen's own
  script writes no time at all. It shows this project's own teams: one booted
  from another project is not listed. `GET /` carries a person to the activity
  screen while this project has a team running, and to the team definitions
  when none is. `hergy cheat`, `hergy cheat --ai` and the `hergy` skill move
  with it.
- **The lists of checks and the gate that holds them**: `go run ./tools/check`
  gained a `checks` gate. Every screen carries its list, `checks/<画面>.md`, one
  row to a check, with the ID the check is known by and the layer it is tested
  at; the gate holds each item to a test of its own — an E2E item to a `t.Run`
  standing directly under `TestE2E*` in `internal/app/e2e_*_test.go` whose name
  starts with the ID, a 受け口 item to a test of `internal/web/*_test.go` whose
  name holds the ID (a function name writes its `-` as `_`) — and fails while an
  item has no test yet, naming every one of them, or while an e2e subtest
  belongs to no item of the lists.

### Changed

- **The screens and the commands say ルール and トポロジー (d-0e10, d-dded,
  d-d38b)**: 約束 is ルール wherever a person reads it — the team screen's tabs
  (`トポロジー` / `ルール`), its list column and the check column's heading
  (`チームのルール`), an empty rule document's title (`<name> チームのルール`),
  and the shared layer's own (`共通のルール`) — while one rule is still 規則, and
  チームの姿 is トポロジー. The sentences move with it: `error.docs_gone`,
  `error.prompt_missing_part`, `validate.old_team_agreement`,
  `validate.old_common`, `effect.preview.agreements`, `hergy cheat` and
  `hergy cheat --ai`, the `hergy` and `hergy-rules` skills, `README.ja.md` and
  the starter rule section `hergy new` writes
  (`internal/scaffold/files/ja/agreement.yaml`). The English words are
  unchanged, and so is 規則 for one rule. A prompt built from a starter
  agreement now reads `## チームのルール` where it read `## チームの約束`, so
  `hergy up --dry` differs on that line (the way to move is the last step of
  Moving from 0.8 below).
- **`hergy stop` sends esc a second time when the member keeps working**: some
  agents need two escs — the first only asks them to interrupt and they carry on
  (opencode answers `esc interrupt`) — so `hergy stop` watches the member's state
  for a few seconds after each esc and sends a second esc only while the member
  is still working. An agent that stops on the first esc is never sent a second,
  because on some harnesses the second esc does something else. A member still
  working after two escs is reported as not stopped (exit 1), where it used to be
  reported as stopped, and `hergy refresh <member>` is named as the next step,
  the same step a stop that worked names. hergy carries no rule per harness. The
  wording of `hergy stop --help`, `hergy cheat --ai` and the `hergy` skill moves
  with it.
- **The screens read and write through one boundary**: every screen `hergy serve`
  serves now answers JSON under `/api/` — `GET /api/{kind}`, `GET
  /api/{kind}/{name}`, `POST /api/{kind}/check`, `PUT /api/{kind}/{name}` and
  `DELETE /api/{kind}/{name}`, where `kind` is `profile`, `role` or `team` — with
  the run's token on the `X-Hergy-Token` header, and draws its rows, cards and
  YAML sections in the page's own script from one island of JSON, where it used
  to receive HTML fragments. Nothing a person sees or does changes: the screens
  look and behave as they did, and `hergy validate` still decides whether the
  settings hold. The old per-screen endpoints (`/profiles/save`,
  `/profiles/yaml/check`, `/roles/assign/save` and their pairs) are gone. Every
  answer carries the shape the boundary's table names: `GET /api/{kind}` answers
  `items` — each name with its `places` (each place's `doc`, `text`, `hash`,
  `path` and the problems it holds), its `effective` and `hidden` places, its
  `summary` (the columns its list shows) and its `problems` counts — and
  `hashes`; `GET /api/{kind}/{name}` answers `{doc, text, hash, path, effective,
  hidden}`; a check answers `{problems: [{sev, at, text}], preview?, effect?,
  readers?}`, with `at` the document's field path or `line` for a file's text; a
  save answers the document and the list, a delete the list. A screen's island
  is that same list answer, `api.js` is the one place that reads it — no screen
  reads an answer's fields itself — and the role screen's YAML tab is drawn by
  the shared `yamltab.js` part, as the profile screen's already was.
- **Every screen's YAML tab saves and answers the same way**: which text may be
  saved, where `保存しました` stands and how tall the box is are the shared
  `yamltab.js` part's now, not each screen's. A save is stopped only when a
  problem the written file did not hold has appeared, so a file that already has
  a problem can still be edited and saved; a text that changed while the page was
  open is refused with the server's sentence beside the save; `保存しました` is
  drawn by the part, and the role screen's own copy is gone. The box stands
  between 6 and 34 lines on every screen.
- **A team.yaml names a profile `profiles.yaml` does not hold**: it reads. A
  member or a worker that writes such a name used to make the whole file
  unreadable, so a screen could only show its checks and the text; now the
  definition is read and drawn — the mission, the numbers, the figure and the
  agreement — so the name can be fixed where it stands, and the check names the
  profile as a problem that stops the definition (`members[i].profile`,
  `workers[i]`), which `hergy validate` prints where it printed the reading's
  refusal (its exit code is unchanged). `hergy up` refuses for such a member in
  the same sentence as before; a worker's missing profile no longer stops the
  boot — it is refused when that worker is hired, as the hire has always been
  checked.

### Fixed

- **Built with Go 1.26.10**: the toolchain moves from go1.26.9 to go1.26.10,
  which fixes GO-2026-6809 in `crypto/x509` (reachable from `hergy serve`).

- **A screen's problem sentence writes a path the way a person reads it**: a file
  inside the project reads `.hergy/profiles.yaml`, a file of the layer every team
  shares reads under the home directory (`~/.config/hergy/…`), and the machine's
  own path reads nowhere. The figure's check column, a team's own agreement's, a
  role document's, the profiles screen's YAML tab and the save the shared YAML
  tab answers with all name a file the way `hergy` prints it on the command
  line, so the same problem reads the same sentence wherever it is shown. A
  sentence about the file it stands in no longer opens with that file's name as
  well: the place a screen draws the problem — a panel's head, the YAML tab's
  file — gives the name already, and the sentence keeps the location it gives at
  its end and the names of the other files it is about. `hergy validate` and
  `hergy up` are unchanged: the file's own location still leads the line there
  (spec/32 D2).
- **The 割り当て saves and the shared YAML tab**: a save from a 割り当て tab
  (`PUT /api/team/{name}`) carries the picks as the team kind's own ops
  (`memberRole`, `memberProfile`, `workerProfile`) — the rows the role screen
  still sends are read as before — so the save writes the one line each pick
  names, and the team kind refuses an op it does not know or a profile the
  definition does not read instead of writing. The shared YAML tab
  (`yamltab.js`) sends the file's name with the text it saves, so a kind whose
  file is the name it was handed (a role's file is the row's own name) writes
  that file rather than an empty one.
- **The team screen's figure, tabs and cards**: the figure is drawn at the width
  of its own viewBox, so an open card stands beside the points instead of over
  them; the band's `一覧` / `YAML` tabs carry the address again when they are
  pressed; the agreement panel's head is its path alone; `元に戻す` stands in the
  agreement's foot only while the draft differs from the file; a line a change
  takes away is struck through once, in the prompt's own words, with the checks
  it adds beside it; a worker's purpose can be typed to its end with the cursor
  staying in the field, and the worker it wrote counts as one change; the card
  of the last turn a member hands out names the problem the press would add and
  keeps the line that would leave beside it, though the change itself is what
  stops the definition reading; and the checks of the team's own agreement stand
  under `チームのルール` and open that rule in the ルール tab, brought into the window
  above the save's band with the cursor in the rule's body — the same from the
  address the press writes.
- **A document-wide sentence marks the first line alone**: the Markdown box of
  the shared part (the role screen and the team screen alike) marks a check
  about a document as a whole — a member's reading volume, say — on the first
  line, not on every heading and blank line beside it.
- **`hergy new` adds the starter profile to a `profiles.yaml` that is already
  there**: the layer keeps every other line — the order of the profiles, a
  comment, the way a person wrote a value or an argument list — and only
  `claude-default` is added, so the file reads as it did with two lines more.
  It used to be written out whole, which sorted the profiles and rewrote their
  values and dropped their comments. The team screen's save of a new definition
  goes through the same code, so both now write the file one way
  (spec-boundary 5).

### Breaking

- **A rule file no longer writes `style:`**: a section's rules are rendered as
  `- **<id>** <text>`, and a rule with no id as `- <text>`. The id is at the
  head of the item, so the ids are in the prompt wherever a section used to say
  `style: bullets`, `numbered` or `paragraphs`. A file that still writes
  `style:` reads — the line is ignored — and `hergy validate` warns once for each
  section that writes one, however many teams read the file, naming the file,
  the heading and the line to delete (`rules.style_gone`, in both languages; it
  moves no exit code). The role
  screen no longer offers a style to pick, and the first save through it removes
  the line. `hergy new` writes no `style:` in the files it lays down.

### Moving from 0.8.0

1. Delete the `style:` line of every section of a role document and an
   agreement. hergy keeps reading the file and warns until the line is gone:

   ```yaml
   # 0.8
   sections:
     - heading: Team agreements
       style: bullets
       rules:
         - id: C1
           text: ...
   # now
   sections:
     - heading: Team agreements
       rules:
         - id: C1
           text: ...
   ```

2. Expect the prompt to change where a section wrote a style other than `ids`.
   The rules keep the file's order in every case, and only the shape of the
   items moves — a `style: bullets` section, before and after:

   ```markdown
   ## Team agreements

   - Hand the turn to whoever acts next.          # 0.8, style: bullets
   - Say what you need from them.
   ## Team agreements

   - **C1** Hand the turn to whoever acts next.   # now
   - **C2** Say what you need from them.
   ```

   - `style: paragraphs` (the default) and `style: bullets`: each rule becomes
     one bullet item, and both gain the id at the head of the item.
   - `style: numbered`: the numbers are gone; the items are the same bullets.

   Nothing else changes: the heading, the `intro`, the text, the reason and the
   reports a member reads are the same.

3. Expect a starter agreement's heading to read ルール, not 約束, and the team
   screen's tabs to read トポロジー and ルール. The words only; a file already
   written keeps its own heading, so a team made before this version reads as it
   read, and a team made by `hergy new` (or added on the team screen) carries the
   new words into the prompt. The line in `hergy up --dry`:

   ```markdown
   ## チームの約束
   - **C1** gy-loop スキルに従い、会話が切れても gy から続けられるようにする。   # 0.8
   ## チームのルール
   - **C1** gy-loop スキルに従い、会話が切れても gy から続けられるようにする。   # now
   ```

   Nothing has to be done to an existing file: 約束 in a heading is a word in the
   text, not a key hergy reads, and `hergy validate` is quiet about it. A file
   you want to read the new words can have its heading changed on the rule
   screen (or in the file) the same way any heading is.

## 0.8.0 - 2026-10-20

### Changed

- **The role document screen is rebuilt around the three things a person does
  with roles**: `/roles` now has three tabs — ロール / 割り当て / YAML. The
  role tab lists every role document of the project and of the shared directory
  (name, title, place, rule count, the origin/reason/check marks of every rule,
  the largest amount one reader is handed, and who reads it), with chips for
  the rules that have no origin, no reason or a problem, and a search over the
  rules. A row opens in place: the people that read it, then one panel per file
  (the project's above the shared one of the same name, which says 効いている /
  隠れている), where a role is renamed, gives sections and rules with their
  origin, reason and check, and is written in place — every field is checked
  while it is typed with the sentences `hergy validate` prints, the Markdown a
  member's prompt will carry is drawn live beside the editor with its line
  count, character count and diff, and a save writes only the lines that
  changed (a move rewrites no line of the rule's own). The 割り当て tab shows
  every team definition's members with their role and the amount they read, and
  writes a new role into that team.yaml. The YAML tab shows a file as text with
  line numbers and validate's problems beside it, and writes the text back as
  typed. The old `/roles/<role>`, `/roles/<role>/new` and
  `/roles/<role>/<rule>` paths move to the opened row or rule with a 301, and
  the old role screen and its write paths are gone.

  The check and the write live in the core, so the screen and the command line
  say the same sentence: `rules.ReadAll` collects every problem of a document
  instead of stopping at the first, `rules.CheckAll` does the same across the
  documents one member reads, `check.Role` brings one role document's problems,
  its order warnings, its arrows and each reader's amount together, and
  `rules.Rewrite` writes a whole set of changes back in one pass.
  `rules.Document.Lines` is the one place the Markdown a member reads is cut
  into lines, so the screen's preview and the prompt cannot drift apart.

- **`hergy validate` warns when a rule body names a model**: a role document
  that says `claude`, `opus`, `gpt-4o` or the like in a rule's text is reported
  (`validate.rule_model_name`), because a role document is meant to read the
  same whatever model runs it. This adds lines to `hergy validate` output for
  documents that name a model.

- **A date a person wrote without quotes is no longer refused**: YAML reads
  `date: 2026-09-14` as a date and hergy writes `date: "2026-09-14"`, and the two
  are the same value now, so a settings file a person wrote by hand — a role
  document, a definition or a profile — writes its one line instead of stopping
  with `error.edit_yaml`.

### Added

- **Four commands reach a member by name, and `hergy status` says more**:
  `hergy focus <member>` moves herdr's focus to that member's pane;
  `hergy nudge <member> [text]` sends a short push when the member is idle or
  done (a working or blocked member is refused and nothing is sent, and with no
  text it sends a short "carry on"); `hergy stop <member>` sends esc to end the
  member's current turn and names `hergy refresh <member>` as the next step (a
  member that is not working is left alone, and the command still succeeds); and
  `hergy tell <member> "<text>" [--from <member>]` sends a message headed by who
  it is from — `--from`, else the member whose pane you type in, else the human —
  without reading the member's state. All four take a member by short member name
  or herdr name, take `--team <team>` to pick the running team, need no
  herdr-managed pane of their own, and refuse a member that is not the team's;
  `--dry` prints what would be sent and sends nothing (focus has no `--dry`), and
  a member cannot stop itself, the same reason refresh refuses it. Alongside,
  `hergy status` now adds what a state carries: `blocked (waiting for
  permission)`, and, when a working member's screen has not changed since the
  last status, `working (unchanged for N min)` — kept under
  `$XDG_STATE_HOME/hergy/screens.json` from what `herdr agent read` shows, with
  the count in whole minutes. `hergy status --json` prints the same rows as JSON
  (`name`, `state`, `unchanged_minutes`, `live`, `worker`) and is the one command
  that takes `--json` without `--dry`.

- **The amount a member is handed has one sentence**: `size.member_rules` and
  `size.member_chars`, used both by `hergy validate` and by the role screen's
  assign tab, so a person reads the same warning in both places.

- **The profile screen is rebuilt around the three things a person does with
  profiles**: `/profiles` now lists every profile from the project's own file
  and the one outside it (name, kind, place, args, env and who uses it), with
  kind chips and a name search above it, and a row that opens in place. An open
  row shows the people that name the profile, then one panel per file, so the
  project's profile and the shared one of the same name stand side by side and
  say which one is in effect. A profile is added, duplicated, copied to the
  other file and deleted from there; every field is checked while it is typed
  with the sentences `hergy validate` prints, they stand beside the field they
  are about, and Save writes only what changed — every other line, comment and
  order of profiles.yaml stays as it was. A rename can write the new name into
  the team definitions that used it. `/profiles/assign` shows every team
  definition's members and workers with their profile, and a pick is written
  into that team.yaml. `/profiles/yaml` shows both files as text with line
  numbers, validate's problems beside the editor on their lines, and writes
  the text back as typed. A rename that splits one name between the project's
  file and the shared one shows both rows at once, in the order a reload
  shows them. A profile that is in use is deleted after the confirm
  names how many people use it, and the assign tab then shows those people
  naming a profile that is not there. The environment rows are written in the
  order the person put them. The two families the screen names — IBM Plex Mono
  and Zen Kaku Gothic New, both under the OFL — are kept beside the page under
  `/static` and read with `@font-face`, so the page's own policy still allows
  them and the text shows the fonts the design asks for. The old
  `/profiles/<name>` path moves to the opened row. `hergy validate` reads
  profiles with the same one-pass check, so a screen and the command line say
  the same sentence, and reports an environment variable written twice.

- **Every pane hergy makes gets `GY_ACTOR`**: `hergy up` and `hergy hire` set
  `GY_ACTOR=<herdr name>` in each pane's environment — the member's or worker's
  herdr name, `<team>-<name>` — so an agent's records in gy carry the name herdr
  knows it by. A profile that writes `GY_ACTOR` does not win: hergy's value does,
  and `hergy validate` warns when one writes it. The bootstrap prompt gains one
  sentence (`%{actor}`) telling the member or worker not to change the name. One
  visible difference: a member's `env` in `hergy up --dry --json` is no longer
  absent when the profile writes none, because `GY_ACTOR` is always there.

  Before: a profile could set `env: {GY_ACTOR: builder}` and the pane carried
  `builder`.
  After: the pane carries the member's herdr name (`<team>-<member>`); take
  `GY_ACTOR` out of the profile, which `hergy validate` now warns about.

- **A team definition can write `ledger:` rules, and `hergy validate` checks
  them**: `ledger:` is a list of `<write> by <member>[, <member>...]`, where
  `<write>` is the name of a gy write (the first two words of the `why` gy
  records: `req add`, `req approve`, `req done`, `req revise`,
  `criterion satisfy`, `need add`, `need close`, `decide`, `question add`,
  `question close`) or `question to human` for a question whose decider is
  nobody in the team. `<member>` is a `members[].name`; a worker's name is
  refused, because a worker takes no rules. A line hergy cannot read, a write it
  does not know, and a name that is no member stop the definition, the way a
  broken `turns:` line does. The check itself stops nothing: `hergy validate`
  reads the project's ledger (`gy list --since 0`) and warns once for each write
  a writer of the team made that the rule gives to other members — a writer of
  the team is one whose `GY_ACTOR` begins with the team name, or with a name the
  launch record shows the definition was started as (`--as`) — and says so in
  one line when gy cannot be read. `hergy show` prints `ledger:`; `hergy new`
  does not write it.

### Removed

- **The old WebUI screens are gone; `hergy serve` shows the profiles and the
  roles**: every page now carries one band with four screens — 活動, チーム定義,
  プロファイル and ロール — and only プロファイル (`/profiles`) and ロール
  (`/roles`) open; 活動 and チーム定義 are drawn struck through and are not
  pressable yet. `/` moves to `/profiles` (it no longer opens the team
  definitions). Walking to the other screen with the band carries the changes
  that were not saved yet, and the band shows a dot beside a screen that holds
  them. The screens the old WebUI had are gone, and every path under them
  answers 404 with the band on the page: `/definitions`, `/activity`,
  `/agreement`, `/teams`, `/team`, `/running`, `/rules`, `/diff`, `/bump`,
  `/figure`, `/panel` and a definition's field paths
  (`/definitions/<name>/members/<member>` and the like). `/static/hergy.css` and
  `/static/htmx-2.0.4.min.js` are gone too. A 404 is a page, not a redirect: a
  bookmark lands on one that says there is no such screen. A page that is still
  there moves: `/profiles/<name>`, and `/roles/<role>` with `/new` or
  `/<rule>` behind it, go to the list with that profile or role opened (a
  bookmark keeps working). `hergy serve` writes nothing now but a role document
  and the two profiles.yaml files.

  Before:
  - `/definitions/core`, fix the mission.
  - `/agreement`, fix a rule every team reads.

  After:
  - edit `.hergy/teams/core/team.yaml`, then `hergy validate`.
  - edit `.hergy/agreement.yaml` (a team's own rules stay in
    `.hergy/teams/<team>/agreement.yaml`), then `hergy validate`.
  - what `/activity` showed (a running team and what it is doing) is
    `hergy status`.
  - what `/definitions/core/members/lead` wrote (a member's role and profile)
    is `members[].role` and `members[].profile` in that team.yaml.

## 0.7.0 - 2026-10-17

### Changed

- **The web UI is rebuilt around the four concepts**: `hergy serve` now shows
  the activity, the team definitions, the roles and the profiles, in four
  bands. `/` and `/definitions` list the team definitions (the shared agreement
  above them), `/definitions/<definition>` shows one definition — its mission
  (the one field edited from the screens), its figure, its members, its workers,
  its turns and its agreement — `/activity` shows what each running team is
  doing and moves herdr's focus to a waiting pane, and `/roles`, `/agreement`
  and `/profiles` show the rest. Every path answers the whole page, so a deep
  link and a reload work without JavaScript.

  The layer is no longer in the screens or their paths; where a file lives is
  said by its path on the one line a save writes. The old paths move with a 301
  (`/teams` → `/definitions`, `/team?name=x` → `/definitions/x`, `/running` →
  `/activity`, and a `?layer=` parameter is dropped), and `/rules`, `/diff`,
  `/bump`, `/figure` and `/panel` are gone (404): the file-by-file rules screen,
  the prompt diff, the version screen and the separate figure are no longer
  concepts of their own. The mission, the members, the turns, the rules and
  the profiles are written from the screens, as the entries below say.

- **The rule screens write**: a team's own agreement, a role's document and the
  agreement every team shares (as well as the rule documents the definitions
  read) now write from the screens. A rule opens in place as a card carrying the
  whole of its text, its reason, its origins, its check and its note, each
  writable from the start; no mode has to be entered first. A rule is added at
  the end of a section, with the id hergy proposes and the person may change,
  and taken out with a second press of Delete in the same place — no dialog. A
  role whose document no layer holds, and a place with no shared agreement,
  offer to make an empty one where hergy puts something new. A save shows the
  line it wrote and what a running team has to take up, as the commands to type:
  a rule document reaches every member that reads it, so a role's document
  reaches the members that play the role in every definition, and the shared
  agreement reaches every member of the place. A rule that would break the
  documents one member reads (an id two of them hold, a report or a rule a
  derived origin points at that is not there) is refused with the sentence hergy
  validate prints, beside the card that brought it in. The report shapes a rule
  may point at are read only in this round.

  A rule document is written a rule at a time: a save writes the lines of the
  one rule it changed, in the shape hergy writes a rule with, and every other
  line — the blank lines, the comments, the quoting a person used, and another
  rule's `refs: [a, b]` — comes back byte for byte. The document hergy makes
  from nothing (a role's document, and the agreement every team shares, when no
  layer holds one) is written whole, as it always was.

- **`hergy validate` warns when a rule text names something the definition has
  no node for**: a rule that writes `lead → impl`, where `impl` is neither a
  member of this definition nor a purpose in its `workers:`, now gets a warning
  that names the unknown name. It moves no exit code, and it does not stop a
  team from booting. A name that is a worker's purpose is not such a name:
  `workers:` ties that worker to its member, so `lead → 実装` is left alone (a
  rule text writes a worker by its purpose, and the worker is still no node of
  the graph). The human is read by both words a rule's text may write it with,
  `human` and `人間`, whatever the language of the text, so a Japanese rule that
  writes `spec → human` is read as a turn to the human rather than warned about.

### Added

- **The screens read a little more easily**: an arrow in the figure is pressed
  where it looks, with a wider line that cannot be seen inside the same link; a
  member's box names its second line (`ロール lead・プロファイル planner`), so
  the role and the profile are told apart; the parties one node hands the turn
  to, and the workers one member hires, stand under one name with a value each
  rather than repeating the name; the edge a press chose to add is drawn in a
  lane beside the members, where the arrow will stand once it is written; and a
  turn the human hands on says what changes on the human's side (`人間が spec に
  ターンを渡せるようになります（プロンプトは変わりません）`) instead of only
  that no prompt moves. A preview chosen with a URL (`?preview=role&role=spec`)
  draws the save and revert buttons without a script, and a draft in a rule's
  origin row keeps its own place when another origin is written above it.

- **The activity says how long a team has waited, and what gy asks**:
  each section of `/activity` says how long the node it names has been where it
  is (`lead が 25 分前から止まっています`), counted from when the screen first
  saw that state — a state that was already there when `hergy serve` started
  reads as a lower bound (`9 分以上`). When the project the screens are served
  from keeps a `gy.toml` and `gy` is on `PATH`, gy's open questions are read
  through gy's own command line and shown at the top of the activity as this
  project's questions, with their options and who decides them; a question
  asked by one of a running team's own herdr names stands on that team's
  section too. A project without gy, or a machine without it, is unchanged: the
  activity is made from herdr alone, and hergy keeps no ledger of its own. A
  team whose definition this project does not hold — a team booted from another
  project — is now read from the project its launch record names, judged and
  counted like any other, with the place that project stands named beside its
  definition and no link to a definition page this serve cannot answer.

- **A team's detail says when it was launched and what it still has to take
  up**: `/activity/<team>` shows the launch record's time, and, when the
  prompts read differently now from the prompt the launch handed out, one line
  per member whose prompt changed (`起動のあとで変わったもの: lead の
  プロンプト`) with the command that hands the change over (`hergy reprompt
  lead --team <team>`). The difference itself is not shown: the record keeps
  only hashes, not the text. A prompt is rebuilt for the comparison from the
  facts the record keeps — the hergy binary, the project root and the language
  it was written in — so an unchanged team says nothing; a record from before
  those were kept says nothing either, rather than something made up.

- **The turns a team hands out are written from `hergy serve`**: a member's field
  and the human's field now list who they hand the turn to, with a press to take
  one turn away and a picker that adds one more. The picker offers the members
  and the human the node does not hand the turn to yet — never itself and never
  a worker, whose two edges are made by `workers:` — and, while a party's field
  is open, the figure draws a "＋" on each of those parties, so a turn can be
  added from the figure as well. A turn is also taken away from the arrow's own
  field.

  A choice made before the write reads the field again with it pending: the
  figure draws the added edge dashed (the edge to be taken away, faint), and the
  row says what a save would do to the members' prompts, so the line comes
  before the save. Taking the last turn out of a node, or the last turn that
  reaches the human, is refused with the sentence `hergy validate` writes, beside
  the row. A save writes one line of `turns:` and leaves every other line as it
  was.

- **A member's role and profile, and the workers it may hire, are written from
  `hergy serve`**: opening a member of a team definition now gives a picker for
  its role (the roles hergy resolves, and the name the definition writes now)
  and for its profile (the profile names alone; the kind of the picked profile
  stands beside the pick as its attribute). The workers the member may hire are
  written the same place: a purpose and a profile to hire one, the profile of a
  worker to change one, and a press to take one away. That last press writes
  nothing at once: it shows what the hire would lose ("`hergy hire --purpose
  <purpose>` は断られます") and offers the write or the way out. The worker's own
  field writes the same profile and takes the same worker away.

  A save writes one place in `team.yaml`, leaves every other line as it was,
  and says what was written (with the key between the language's own quotes),
  what the change does to the members' prompts, and the command a running team
  needs (with a copy button). A change that would stop the definition is refused
  with the sentence `hergy validate` writes, beside the field; a file that moved
  under the page is refused with a link that reads it again. Two fields of one
  member are filled in and saved one at a time, and a half-written field is kept
  while another is opened.

  A pick that has not been written yet shows what it would do before it is
  written: picking another role or profile reads the field again with the pick
  pending and prints the same line the save would print (or the sentence the
  save would be refused with), so the person knows what a save costs before
  paying it. A worker taken away from its own field lands on the member that
  hired it, and the press that takes one away says what the hire loses first.

- **A profile is edited from the screens**: `/profiles/<profile>` writes a
  profile's kind (chosen from the kinds herdr starts, the ones the profiles in
  use run first, the one run by the most profiles before the rest), its args
  (one to a line) and its env (a name and a value each, the value a box so a
  line break in it survives). `/profiles` adds a profile by name and kind, and
  says which `profiles.yaml` it will write before the button is pressed. A
  profile that runs something is not taken away — the screen names the members
  and workers that use it — and one nothing runs is.

  A write changes only the lines it touches: the profile's own lines are written
  where they stand, a new profile goes after the last one (not in name order),
  args and env keep the form the file wrote them in (a flow collection on the
  key's line, or block lines under it), a value that YAML would read as a number
  or a boolean (`1`, `true`, `1.5`, `007`) is written quoted so it comes back as
  the text it is, and the comments, the blank lines, the order of the profiles
  and the quoting of a value nobody touched come back byte for byte — so a save that changes nothing leaves the file exactly as it was.
  Only a file hergy makes (a new `profiles.yaml`) is written in hergy's own form.
  A write says what it wrote, whose launch or pane environment the change moves
  (named by its definition when the change reaches members of more than one
  definition, so the same sentence twice tells two members apart), and what each
  running team has to take up (`hergy refresh` for a kind or an args change,
  `hergy down` and `hergy up` for an env change), and a name that is its kind gets
  the warning `hergy validate` gives.

- **An end-to-end run of the screens**: `go run ./tools/check` gained an `e2e`
  gate that drives the served pages in a real headless Chrome through chromedp
  (the Go test is behind the `e2e` build tag). It checks the four bands, the way
  into a field in two presses, closing a field three ways, the mission being
  written and the saved line, a member's role being written from its field with
  the profile kind following the pick and a worker's removal saying what the
  hire loses first, a draft surviving another field, the pointer cursor never
  standing on something unpressable, the three widths scrolling neither way, Tab
  and Enter reaching a field, and no console errors. A machine with no Chrome
  fails the gate in CI and skips it with one line on a laptop. The profile
  screen's kind, args and env writes are walked too.


## 0.6.0 - 2026-10-17

### Added

- **A team says what it is for, in one line**: a `team.yaml` writes
  `mission:`. The commands that handle a team print it at the top of their
  header, and the web UI's team list and team definition screen show it too.
  It is required: see "`mission:` is now required" under Changed for what a
  definition without one gets, and how to add one.

### Changed

- **The layer is no longer a word a person reads**: `hergy` says where a team
  definition lives by its path (`.hergy/…`, `~/.config/hergy/…`), never by a
  layer name. The commands' headers lost the `layer:` line, `hergy ls` lost its
  `local:`/`global:` lines (a name that is also outside the project is said in
  one line), `hergy new` says where it writes when `--local`/`--global` is
  typed, and a definition that is not found is reported in one shape:
  `there is no team definition X (looked in: <paths>; candidates: …)`. A
  command that is given no team name counts the project's definitions (that is
  what a name that is omitted reads), and with none anywhere it says
  `next: new <name>`. The
  cheatsheet, the bundled skills, the READMEs and every `--help` say the places
  by path too. Every `role file` in the English messages, the cheatsheet, the
  skills and the README is now `role document`. The `--local`/`--global` flags
  keep stopping with the sentence that says a name resolves to the project's
  own first. A broken `turns:` line is named by the line a person counts
  (`line 2`, not `turns[1]`), and the line a member's prompt writes for the
  human says what to do with the turn (hand it over and stop until the person
  answers).

- **`hergy hire` refuses a purpose the hiring member's `workers:` does not
  name, and `--profile` is gone**: the profile comes from the `workers:` entry
  (the member → worker edge), so a hire does not choose one; typing `--profile`
  stops with a sentence that says where the profile comes from now. A purpose
  outside the hiring member's entries stops the hire and lists the purposes that
  member may hire for. When no member hires for the purpose at all, the sentence
  says who may hire for what instead of asking for `--from`. Before, such a
  purpose only printed a note on stderr and the hire went on.

  Before:

  ```sh
  hergy hire researcher --purpose 調査 --profile seeker --task "..."
  ```

  After:

  ```sh
  hergy hire researcher --purpose 調査 --task "..."   # the profile comes from the lead's workers: entry
  ```

- **`mission:` is now required**: every `team.yaml` writes one line saying what
  the team is for. A definition that writes none, writes an empty one, or
  writes one over more than one line does not read, and `hergy validate` reports
  the file broken, with the file's path, the way it reports any other break.
  `hergy new`'s starter team writes an example mission too, and `hergy validate`
  warns (without failing) while a definition still writes that example: it has
  not said what its team is for yet.

  Before:

  ```yaml
  version: 1
  members: [...]
  ```

  After:

  ```yaml
  version: 1
  mission: what this team is for, in one line
  members: [...]
  ```

- **A team definition writes no `docs:` and no `roles:`; where a document
  stands says who reads it**: the agreement every team of a layer shares is
  `<layer>/agreement.yaml` (one file), a team's own agreement is
  `<layer>/teams/<name>/agreement.yaml`, and a member's role document is
  `<layer>/roles/<role>.yaml`. A name is looked up in the project's layer first
  and in the global layer after, so a role document lives once per layer rather
  than once per team. A member's prompt carries them in one order: the role
  document, the shared agreement, then the team's own. A prompt template has to
  keep `%{role_doc}` and `%{docs}`, the two places the documents are put in, and
  `hergy validate` stops a template that dropped either. A `team.yaml` that
  still writes `docs:` or `roles:` stops the read with a message that says where
  the documents went. A member whose role has no document is stopped with a
  message naming the member, the role and every place hergy looked for the
  document. `hergy new` writes `agreement.yaml` into the team's folder and the
  two role documents into
  the layer's `roles/`, and leaves a role document that is already there as it
  is. `hergy validate` counts the shared agreement among a team's sources.

  `hergy validate` also warns, without stopping, about a file left where hergy
  no longer reads it: a `teams/<name>/protocol.yaml` that was not renamed, a
  role document still in the team's folder, or a `<layer>/common/` directory.
  The warning names the file and the place it goes now, so a file that was not
  moved over is not read away in silence.

  Before:

  ```yaml
  # teams/agile-dev/team.yaml
  docs: [../../common/, protocol.yaml]
  roles: [../../roles]
  members:
    - name: lead
      role: lead
  ```

  After:

  ```yaml
  # teams/agile-dev/team.yaml
  members:
    - name: lead
      role: lead

  # the layer root
  agreement.yaml                  # every team of the layer reads it
  roles/lead.yaml                 # a member's role document
  teams/agile-dev/agreement.yaml  # this team's own agreement
  ```

  Move: concatenate the shared files into `<layer>/agreement.yaml`; move the
  role files into `<layer>/roles/`; rename each team's `protocol.yaml` to
  `agreement.yaml`. A definition that read another definition's file
  (`docs: [../other/protocol.yaml]`) copies that file into its own
  `agreement.yaml`.
- **A member names a profile; `kind`, `args` and `env` are no longer written on
  the member**: a `team.yaml` member writes one `profile:` key, naming a profile
  in `profiles.yaml`, and the profile holds the engine. Writing `kind`, `args` or
  `env` on a member stops the read with a message that says to write it in a
  profile and to give the member `profile: <name>`. A member with no `profile:`
  is stopped the same way. `hergy validate` warns when a profile is named after
  its kind (`claude`), because such a name reads as a choice of kind where a
  profile is a choice of what the engine runs; the warning does not stop
  anything. `hergy new`'s starter team names `claude-default`, and `hergy new`
  writes that profile into the layer's `profiles.yaml` when the layer cannot
  already resolve the name. The write is the whole file, in profile-name order,
  so comments in that `profiles.yaml` and the order the profiles were written in
  do not survive it. To move a definition over:

  Before:

  ```yaml
  members:
    - name: builder
      role: builder
      kind: claude
  ```

  After:

  ```yaml
  # teams/<definition>/team.yaml
  members:
    - name: builder
      role: builder
      profile: claude-default

  # profiles.yaml, in the layer root
  version: 1
  profiles:
    claude-default:
      kind: claude
  ```

- **herdr's tab for a team carries the team's name alone**: `hergy up <definition>
  --as <name>` now labels the tab `<name>`, where it used to write
  `team:<name>@<definition>`. The definition behind a running team is read from
  this machine's launch record, so `hergy status`, `down`, `refresh`, `reprompt`
  and `hire` keep finding it. A tab is a team's when the agents in it are named
  after the tab's label, so a tab a person labelled with the name of a team that
  has stopped is no longer taken for one, and `hergy down` closes a team whose
  definition cannot be read (a boot that failed once its tab was made) by the
  names hergy gave its panes. A team booted by an older hergy keeps its older
  label (`team:<name>@<definition>`, or `team:<name>`) and is still found from
  it.
- **A member's role is a "role", and who a member hands the turn to is "hands
  the turn to"**: in Japanese, every message and document now says ロール where
  it said 役割, ロールの文書 where it said 役割ファイル, and ターンを渡す where it
  said 手番 or 話しかけてよい. The English text already said role, so it does not
  change. The glossary in `terms_test.go` holds the words, and the test keeps
  them out of the locales, the cheatsheets, the bundled skills and the READMEs.
  No file or command is renamed, so a team definition keeps working as it is.
- **Who hands the turn to whom, and who may hire a worker, are written in
  `team.yaml`**: a definition writes its turn graph in a top-level `turns:`, one
  `- a -> b` per turn, where each end is `human` or a member's name, and what
  each member may hire in a top-level `workers:` (`from:`, `purpose:`,
  `profile:`). A member's prompt is built from these: `%{turns}` names who it
  hands the turn to, and `%{workers}` what it may hire. Writing `may_spawn:` on a
  member, or `%{spawn}` in a prompt, stops the read with a sentence that says
  what to write instead. A turn graph hergy cannot hold stops the definition the
  way a file that does not read does: a member or the human with no turn out, no
  turn reaching the human, an unknown name, a self loop, the same turn twice, a
  `workers:` `from:` that is not a member. `hergy up`, `refresh`, `reprompt`,
  `hire` and `validate` all stop on it, with the same sentences one line each,
  and `hergy down` still closes such a team by name. A rule text that still
  writes "a → b" is compared with `turns:` and warned about when the two
  disagree (the warning does not stop anything). The figure on the web UI draws
  the turn graph, and nothing else. `hergy hire` may leave `--from` out when
  exactly one member hires for the purpose. To move a definition over:

  Before:

  ```yaml
  members:
    - name: lead
      role: lead
      profile: claude-default
      may_spawn:
        workers:
          - purpose: 実装
            profile: opencode
    - name: spec
      role: spec
      profile: claude-default
  ```

  After:

  ```yaml
  members:
    - name: lead
      role: lead
      profile: claude-default
    - name: spec
      role: spec
      profile: claude-default
  workers:
    - from: lead
      purpose: 実装
      profile: opencode
  turns:
    - human -> lead
    - lead -> human
    - lead -> spec
    - spec -> lead
  ```

  Every member and the human need a turn out, and at least one turn must reach
  the human. A prompt that wrote `%{spawn}` writes `%{workers}`. A rule text that
  wrote a turn as "a → b" is left where it is; where it disagrees with `turns:`,
  `hergy validate` says so, and the definition keeps working.

### Fixed

- **The rules screen opens the row you press**: a row outside the first section
  of a document did not open the rule it named. When the first section held no
  rule, pressing the row did nothing at all; otherwise it opened a rule of the
  first section, and saving that form wrote into the other rule. The
  name the row carries and the name the right panel reads are one again, and the
  whole row is now its button, so pressing the row's padding opens it as well. A
  rule the panel cannot open says so in the right panel, in one line, instead of
  leaving the screen unchanged.

### Removed

- **`--local` and `--global` are gone from the team commands**: a team
  definition name resolves to the project's own definition first, then the
  global one, so nobody picks a layer. `hergy new` writes into the project's
  `.hergy/` when hergy runs inside a git repository, and into the global layer
  (`~/.config/hergy/`) outside one. Typing `--local` or `--global` on any
  command stops with a sentence that says so.

  Before:

  ```sh
  hergy up agile-dev --global
  hergy new agile-dev --local
  ```

  After:

  ```sh
  hergy up agile-dev          # the project's own definition wins, else the global one
  hergy new agile-dev         # into .hergy/ inside a git repository, into ~/.config/hergy/ outside one
  ```

  To pick the global definition when a project definition of the same name
  exists, rename one of the two, or remove the project's copy. A team
  definition of the global layer now reads the project's `profiles.yaml` too
  (under the global one). A member's embedded `hergy hire` command no longer
  carries a layer flag.

## 0.5.0 - 2026-10-16

### Added

- **`hergy validate` counts the size of a team, and warns over a provisional
  limit**: after the count of what the rules carry, it prints what each member of
  every team definition is handed — the rules of the documents that go into its
  prompt, and the characters of the Markdown hergy renders from them, with the
  prompt's own frame left out — and what each team definition keeps (its rule
  files, counted once however many members read them). A member over 20 rules or
  6,000 characters, or a team definition over 8 files or 20,000 characters, gets
  a warning, because a large team is a team to look at rather than a broken one:
  the warning never changes the exit code, and `hergy up` still runs. The numbers
  are provisional and are written in one place for whoever moves them; the review
  comes after hergy-dev, the team definition hergy develops itself with, has gone
  through ten changes.
- **The rules screen finds a rule, shows what it is related to, and offers to
  look into where it came from**: the rules list has a search box that keeps the
  rules whose id, text, reason or note holds what you type (the same narrow as
  the filter chips, and it works beside them). The right panel lists the rules
  the open rule's text points at, the rules whose text points at it, and the
  rules derived from it; pressing one opens it in the panel. The panel also
  offers to copy a prompt that asks an AI to find out when and why the rule was
  added, built from the rule's own id, text and reason, the file's path and the
  layer's root.
- **The profiles of a layer, and the form that edits them, in the web UI**: the
  left list's profiles row is a link to `/profiles`, which shows the profiles of
  the layer you pick — their name, kind, args and environment variables — and
  edits them: the form adds a profile, changes a name, a kind, the args (one to
  a line) and each environment variable as a pair of a name and a value, and
  deletes a profile or one of its variables. The values of `env` are shown and
  edited like any other field: a profile's env holds no API key. Saving writes
  that layer's own `profiles.yaml` in the one format hergy writes, which leaves
  no YAML comment behind, and only once the same check `hergy validate` runs
  passes: that the file reads, and that the team definitions reading it can
  still be built, so a save that would leave a definition without a profile it
  names is refused with the definition and the member or worker that uses it, and
  nothing is written. A file that changed under the screen, or a form that came
  from an older page, is refused with the reason and the rows are kept as they
  were typed. The local layer's page also shows the profiles a local team
  definition ends up with once the local layer is laid over the global one, and
  marks which profiles override which.
- **`hergy serve`: the settings in a browser**: `hergy serve [--port N]` (default
  8787; `0` lets the operating system pick a free one) serves the web UI on
  127.0.0.1 and prints the URL. The team list answers `/teams`: one row per
  team definition in the layer you pick (local or global, one toggle in the top
  bar), with the number of members `team.yaml` writes and how many of the rules
  those members read have no check. A definition that cannot be read keeps its
  row and carries why, in the one line `hergy validate` writes; the other
  definitions keep rendering. The frame marks the global layer, and the right
  panel widens to the window without scrolling the page. It answers 127.0.0.1
  only, refuses a request whose `Host` is not its own, takes a write only as a
  POST that carries the token of the running process, and sets
  `Content-Security-Policy: default-src 'self'`. HTMX 2.0.4 (0BSD) is embedded
  with its license.
- **`hergy serve` opens the URL it prints**: once the socket is open, hergy
  hands the URL to this machine's default browser (`open` on macOS, `xdg-open`
  on Linux, `rundll32 url.dll,FileProtocolHandler` on Windows). `--no-open`
  prints the URL and opens nothing. A browser that cannot be started is one line
  on stderr, and `hergy serve` goes on answering, with the URL to open by hand.
- **The rule files say how to write a turn direction**: `hergy cheat --ai` and
  `skills/hergy-rules` now say that a rule's text can hand the turn between two
  of the team as `<name> → <name>`, or `<name> ⇄ <name>` when it goes both ways.
  The name is a member name, a role name, or the human (`人間` in a Japanese
  file, `human` in an English one). A figure that draws a team reads the turn
  directions out of the text.

- **Every launch leaves a record**: `hergy up`, `hergy refresh` and `hergy
  reprompt` write one line per launch to
  `$XDG_STATE_HOME/hergy/launches/log.jsonl` (or
  `~/.local/state/hergy/launches` when unset): when it ran, which hergy ran it,
  the project, the layer, the team definition, the team name, the hash of the
  team as it was launched, the members that were handed a prompt, and the rule
  files those prompts were built from. The texts themselves — each prompt, and
  each rule file — go to `launches/blobs/<sha256>`, one file per text, so a
  launch that changes one document adds one file. No environment value is
  written, in the record or in a blob: the values count toward the team's hash
  only. The hash covers the whole team definition as it was launched, so it does
  not change when a profile no member reads changes, and does change when a
  profile a member reads changes. A `--dry` run and a launch that fails record
  nothing, `hire` records nothing (a worker is not a member of the team
  definition), and a record that cannot be written prints one line on stderr and
  leaves the command a success.

- **The launch record's screens, and the team list's last-launch column**: `hergy
  serve` now opens a Home screen, at `/` (the team list moved to `/teams`) — the
  four steps of the daily flow, numbered, with the step this screen stands on
  marked, then what moved since the last launch (per document,
  the rules it added, dropped and changed by id, and whether its version has
  moved past the one the launch read, which means the next `hergy up` hands the
  change over) and the rules with no check, each of which opens in the rules
  screen's edit panel. It also opens a Prompt diff screen, at `/diff`: per team
  definition and member, the prompt the last launch handed over against the
  prompt the files build now, as a line diff (members whose prompt did not change
  are folded away). These screens read the record and the files through the app,
  so no comparison happens in the page. It also opens a Version screen, at
  `/bump`: the documents whose content moved since the last launch, each with the
  version it holds and the one a bump writes (`n → n+1`; a document that writes
  no version is at 0, so the first bump writes 1). Raising one writes the document
  back with `rules.Emit`, through the same writer the rules screen uses, so a file
  that changed under the screen is refused and a document keeps its rules; the
  screen then names the command that hands the change over (`hergy up <name>`).
  A write is refused unless the document moved since the launch that read it and
  its version has not been raised since that launch, so a page left open, a
  resend or a double press raises nothing. `hergy cheat` and `hergy cheat --ai`
  name the screens, one line each.
  The team list carries one more column:
  when the definition was last launched from this machine, the head of the hash
  that launch wrote, and a mark when the files build a different hash now. On a
  machine that has never launched a team, the home and the diff say so and name
  the command instead; the rules with no check still show, because they are read
  from the files. A record that cannot be read is one line on the screen, and the
  rest of it still renders.

- **`hergy serve` shows the rules, and changes them**: the Rules screen lists the
  rule files the team definitions of the layer read, with the counts
  `hergy validate` counts (rules, and how many carry an origin, a reason and a
  check) and three filters: no origin, no reason, no check. A rule opens in the
  right panel, split into what the prompt carries (its text, its reason) and what
  is kept for maintenance (its note, its origin, its report field, its tests),
  and the panel previews the section as a member reads it while a person types,
  with the break `hergy validate` would name. Saving writes the file back in the
  one format (below); a file that changed under the screen is refused, not
  overwritten. A rule can be added at the end of a section with an id the
  document does not hold, deleted after a second press, and a rule with no check
  can be given a report field where the person already stands. Another screen
  reaches a rule with `/rules?doc=<file>&rule=<id>`, which opens it in the panel.
- **`hergy serve` shows the team definition, the running teams, and the figure**:
  a definition screen lists the documents every member reads, then the members
  and the workers they may hire (each with its role, its engine and its profile),
  and opens one member in the right panel with its Herdr name, its state, the
  workers it may call and the prompt hergy hands it. A running screen reads Herdr
  the way `hergy status` does, and adds what hergy status does not print: what
  each member's terminal says it is doing, and the pane it runs in. The one
  button on these screens moves Herdr's focus to that pane (`herdr agent focus`),
  and it is the only thing they change. A figure screen draws one definition as
  one SVG: the documents, the role files, the members coloured by role, the
  workers as dotted boxes, the human, and one arrow for every turn direction a
  rule writes — named on both ends, on one end (dotted, to the word the rule
  wrote), or listed under the figure with the rule and the names that matched
  nothing. Pressing a document or a rule id opens the Rules screen. The team list
  marks the definitions Herdr reports a running team for.
- **The running teams read Herdr again every five seconds**: the running screen
  keeps itself up to date while it is open, on by default, and the box above the
  table turns that off and on. The choice travels in the screen's own URL
  (`?auto=0` is off), so it survives a reload and a link.
- **The home shows how a rule changed, against which launch**: a document's
  panel on the home and the version screen names the launch the changes are
  against (`前回の起動: <team definition> · <when> · <hash>`, from the launch
  record), and each rule that moved shows what moved — 追加, 削除 or 変更, the
  section it stands in, the parts that moved, and the text before and after (the
  text, the reason and the origin). The home also ends with a card for the work
  a person does now and then, with a link to the team list and one to the
  profiles.
- **A Copy button**: the panels that hand a person text to carry somewhere (the
  YAML of a document, the prompt that asks an AI to look into a rule's origin)
  put it on the clipboard with one press. The script is one file of the binary,
  so a page still carries none of its own.
- **The rules screen has four tabs**: one document opens as 規則, プロンプト,
  前回の起動からの変更, or YAML・検査, chosen with `?tab=prompt` (and the other
  tabs) on the screen's own URL, so the tabs work with no script. プロンプト
  draws the document the way a member reads it, counts the characters hergy
  hands over, can show the Markdown as it is (`&raw=1`) and can draw what the
  prompt leaves out under each rule (`&overlay=1`); pressing a rule opens it in
  the right panel, the same way the rules list does. 前回の起動からの変更 draws
  what moved since the launch that last read the document — the same rows the
  home shows — and raises the version with the write `/bump` makes. YAML・検査
  shows the file as it stands on disk, with a Copy button, the break `hergy
  validate` names for the whole document, and the promise that hergy writes the
  file back in one fixed shape when it saves it, so YAML comments do not
  survive: a maintenance note belongs in a rule's note. A rule file that moved
  since its launch also says so on the document's own line, as `前回の起動から変更
  n 件`.

### Changed

- **`hergy serve` comes in one stylesheet, in the look the prototype settled
  on**: every screen wears the same frame — the two-layer header with the layer
  toggle, the navigation and the three meters (with an origin, with a reason,
  with a check), the list of documents and team definitions down the left, and
  the panel on the right — and the same colours in a light and a dark scheme,
  chosen by what the operating system asks for, in fonts the machine already
  has. The navigation reads Home, Running teams, Teams, Figure, Rules, Prompt
  diff, Raise a version: the map and the triage have no entrance. A document in
  the left list shows how many rules it holds and how many of them carry an
  origin, and how many rules moved since the last launch of that layer.

- **The screens have their own paths, and the home is the way in**: `hergy serve`
  opens on the home at `/`, and the team list answers `/teams`. The home is the
  first thing a person sees, because that is where the daily flow starts
  (d-ff49). A link from the old address (`/` for the team list) now lands on the
  home.

- **Build with Go 1.26.9**: `go.mod` names the toolchain, so a build downloads
  and uses it. The toolchain this repository was built with before it (1.26.7)
  carries fixed vulnerabilities in `net/http` and `html/template`, which
  `govulncheck` reports as soon as hergy serves or renders anything: a machine
  whose Go is older now gets 1.26.9 on the first `go build`.

- **A rule file hergy writes back is written in one format**: `hergy` writes a
  document with the keys in the order the format lists them, leaves out a value
  that is only the default (`send_why: true`) and an empty value, and writes a
  value of more than one line as a block. What is written is read back as the
  same document, and the Markdown a member reads does not change. Two things a
  file may hold are not carried over: a comment, and a default spelled out by
  hand. The first time hergy saves a file it rewrites the whole file, so `git`
  shows one large diff.

- **The header carries the hergy logo (d-fb33)**: the words "Rule editor" beside
  `hergy` are gone, and the top-left corner of every screen shows the mark — a
  small flock of squares, the one in front in the origin colour, and the word
  `hergy` — linking to the home of the layer being shown, so a person reading
  the global layer stays there. The mark is written into the page itself, so it
  costs no second request, and it takes its colours from the page, so it follows
  the light and the dark scheme. It names itself `hergy` to a screen reader, and
  the header keeps the height it had.


## 0.4.0 - 2026-10-15

### Added

- **A skill for the rules of a team**: `skills/hergy-rules`, installed by
  `npx skills add aq2bq/hergy` next to the hergy skill. It asks one thing at a
  time and writes the YAML for the person: a new team, an existing team, and
  moving rules out of Markdown. It does not ask anyone to remember an origin:
  moving rules leaves the origins empty, and later the skill builds a candidate
  for each from the git history and the gy ledger and asks only whether that
  candidate is right.

### Changed

- **`hergy validate` ends with a count of what the rules carry**: after the
  per-file lines, it counts the rules of every file the members of the team
  definitions read (a file two definitions share counts once): the rule count,
  how many say where they came from (incident, instruction or derived) and how
  many do not, how many carry a reason (whether or not the file sends it) and
  how many do not, and how many have a report field or a test and how many do
  not. Then it lists the id of every rule that says where it came from nowhere,
  by file. That count is not a break: only a broken file makes the exit 1. It is
  where a team sees which of its rules came from nothing, before they harden
  into something nobody dares to touch. The Japanese output uses the words the
  product owner chose for it: インシデント for an incident, and 検証方法 for a check.

- **A team's agreements and role files are YAML, not Markdown**: `protocol.md`
  becomes `protocol.yaml`, and `roles/<role>.md` becomes `roles/<role>.yaml`.
  A rule file holds `sections`, each with the rules under it, and each rule has
  an `id`, its `text`, and, when the file gives them, `why` (whether it is sent
  is chosen per rule with `send_why`), `origin` (`incident`, `instruction` or
  `derived`), `note`, `report` and `tests`. A document may also write the shape
  of a report under `reports`. `hergy up` renders each file to Markdown and
  puts it where `%{role_doc}` and `%{docs}` were; `hergy new` writes the
  starter team in the new form. Markdown is no longer read: a `docs` entry
  that points at a `.md` file, a directory that holds only `.md`, or a role
  that has only `<role>.md` stops with the way to move it over. Before:

  ```markdown
  - C1: Work along the gy-loop skill, so that work can resume from gy when a conversation ends.
  <!-- why: Sessions end. An agreement that lives only in a conversation is lost. -->
  ```

  After:

  ```yaml
  version: 1
  sections:
    - heading: Team agreements
      style: ids
      rules:
        - id: C1
          text: Work along the gy-loop skill, so that work can resume from gy when a conversation ends.
          why: Sessions end. An agreement that lives only in a conversation is lost.
          send_why: false
  ```

  Move a team over with the hergy-rules skill: the reasons, the
  origin and the way each rule is checked cannot be read out of the Markdown,
  so the move is a conversation, not a conversion.

- **CI asks more of a change**: `go run ./tools/check` runs every gate CI runs
  (formatting, `go mod tidy`, `go fix`, golangci-lint, the direction of
  imports, `go test -race`, coverage, dead code, govulncheck), and a daily
  workflow runs mutation testing and fuzzing. What hergy prints and accepts
  does not change.

## 0.3.1 - 2026-10-15

### Changed

- **`hergy <command> --help` prints that command's own page**: what it does,
  how to type it, its arguments, and every flag it takes, each with what the
  flag means for that command. `hergy new --help` says that `--local` is where
  the starter team is written (the global layer without it), and that the
  global `config.yaml` is written when it is missing. `-h` prints the same.
  Before, every command's `--help` printed the one list of all commands, and
  `--local` of `hergy new` was written only in the README and in
  `hergy cheat --ai`.
- **`hergy`, `hergy --help` and `hergy cheat` point at the new pages**: the
  list of all commands is as it was, with one line added at its end, and the
  last line of `hergy cheat` names `hergy <command> --help`.
- **`hergy cheat --ai` is written from the same table as `--help`**: each
  command's line, and the commands that take each flag, are read from it, so
  they cannot fall behind what hergy accepts. The lines change shape: values
  are `<team>` rather than `team`, `hire` shows `--purpose <purpose> --task
  <task>`, `--lang` is no longer repeated on every command's line, and each
  line of the Flags section ends with the commands that take the flag. The
  Flags section also lists `--all`, which it lacked. What hergy accepts does
  not change, so there is nothing to move.
- **An unknown option, or a flag the command does not take, points at that
  command's page**: ``unknown option "--foo" (see `hergy new --help`)`` and
  ``--as cannot be used with hergy ls (see `hergy ls --help`)``. The first used
  to point at `hergy help`, the second at nothing. An unknown command still
  points at `hergy help`. When an error is raised, its exit code, and what
  goes to stderr are as before.

## 0.3.0 - 2026-10-13

This release tells a team definition apart from a running team. A team
definition is `teams/<name>/team.yaml`; a team is what `hergy up` starts from
one, under its own name. Commands that act on a running team now take the team
name, so two teams started from one definition can be handled one at a time.

### Breaking

- **Commands on a running team take the team name**: `status`, `down`,
  `refresh`, `reprompt` and `hire` name a team (`hergy down dev2`,
  `hergy refresh lead --team dev2`), not the team definition it was started
  from. `--as` belongs to `up` alone; on the other commands it is an error.
  hergy finds the definition from the team's tab in herdr, which `up` now
  labels `team:<team>@<definition>`; a tab labelled the older way,
  `team:<team>`, is read as a team whose definition has the same name.
- **One word for the people in a team**: everyone written in `team.yaml` is a
  member, and everyone `hergy hire` adds is a worker. `seat` is gone from the
  output and the docs, and the commands that take one say `<member>`.
- **`may_spawn` pairs each purpose with the profile to hire for it**: the
  separate `purposes` and `profiles` lists are gone, and a `team.yaml` that
  still has them is refused with the new form in the message. `hergy hire`
  can leave out `--profile`; the profile paired with `--purpose` is used.
- **The starter team's second member is `builder`**: `hergy new` writes
  `roles/builder.md` instead of `roles/worker.md`, so the member is not
  mistaken for a hired worker. Teams already written are not touched.

### Moving from 0.2

1. Name the running team, not its definition, on `status`, `down`,
   `refresh`, `reprompt` and `hire`. For a team started without `--as` the
   two names are the same, and nothing changes. For one started with `--as`:

   ```sh
   # 0.2
   hergy down dev --as dev2
   hergy refresh lead --team dev --as dev2
   # now
   hergy down dev2
   hergy refresh lead --team dev2
   ```

2. Teams already running keep working: their tabs carry the older label, and
   hergy reads the team name from it as the definition name. A team started
   with `--as` under 0.2 cannot be traced back to its definition that way;
   run `hergy down <definition> --as <team>` with 0.2 before upgrading, or
   close its panes in herdr, then start it again with `hergy up`.
3. Rewrite each `may_spawn` in `team.yaml` as pairs. Every purpose gets the
   one profile its workers should run on:

   ```yaml
   # 0.2
   may_spawn:
     purposes: [research, build]
     profiles: [seeker, developer]
   # now
   may_spawn:
     workers:
       - purpose: research
         profile: seeker
       - purpose: build
         profile: developer
   ```

   `hergy validate` names every `team.yaml` that still has the old form.
   `hergy hire` still takes `--profile`; leave it out to use the pair.
4. Nothing else to do in `team.yaml`, `profiles.yaml` or `config.yaml`:
   teams written by `hergy new` before keep their `worker` member.

### Added

- **`hergy status` with no name lists every running team**: each with its
  team definition, how many members and workers it has, and whether someone
  waits for a human or nobody is working. A team started from a definition in
  another project is listed with a note instead of stopping the list.
- **`hergy ls` names the running teams**: each team definition shows the
  teams started from it.

### Changed

- **`hergy --help` is grouped by what you want to do**, and the long rules
  moved to the README.
- **`hergy cheat` is a short table of what to type**, and its columns line up
  in Japanese too.
- **The Japanese messages read as plain Japanese.**
- **`hergy show` and `hergy up --dry` print the team definition and the team
  name on separate lines.**

## 0.2.0 - 2026-10-13

### Breaking

- **`hergy refresh` restarts a member**: it ends the member's agent and starts
  it again in the same pane with the settings as they are now, so the
  conversation starts empty and an edited `profile`, `kind` or `args` takes
  effect. A member whose agent has stopped is started again in its pane. What
  `refresh` did before, sending the bootstrap prompt again and keeping the
  conversation, is now `hergy reprompt`.

### Moving from 0.1

- Where you typed `hergy refresh <member>` to resend the bootstrap prompt and
  keep the conversation, type `hergy reprompt <member>`. Type
  `hergy refresh <member>` only when the conversation should start over.
- Workers hired with 0.1 are not closed by `hergy down`: their tabs carry the
  older label. Close them in herdr once.

### Added

- **`hergy status`** shows each member and worker with its state, and says who
  waits for a human or that nobody is working.
- **`hergy cheat`** prints a cheatsheet for people, and `hergy cheat --ai` the
  whole of hergy for an agent. Neither reads the settings.
- **A skill for agents**: `npx skills add aq2bq/hergy` installs `hergy`, which
  tells an agent when to use hergy and when herdr, and points at
  `hergy cheat --ai` for the exact commands.
- **The bootstrap prompt says to add teammates with `hergy hire`**, not
  `herdr agent start`, so every teammate shows up in `status` and `down`.

### Changed

- **`hergy down` also closes the workers the team hired**: `hire` labels a
  worker's tab `worker:<team>/<name>`, and `down` closes the agents in those
  tabs.

## 0.1.0 - 2026-10-13

The first tagged release.

- `hergy up`, `down`, `refresh`, `hire`, `ls`, `show`, `new`, `validate` and
  `version`.
- Team definitions in two layers, global (`~/.config/hergy`) and local
  (`<project>/.hergy`), with `profiles.yaml` in either.
- `hergy up <definition> --as <team>` starts one definition as several teams.
- Member names are written short (`lead`); hergy adds the team name in herdr
  (`dev-lead`).
- `hergy new` writes the global `config.yaml` when it is missing.
- `--dry` prints the plan, and `--json` prints it as JSON.
