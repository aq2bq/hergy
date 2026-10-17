# Changelog

## Unreleased

### Changed

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
  the text back as typed. A profile that is in use is deleted after the confirm
  names how many people use it, and the assign tab then shows those people
  naming a profile that is not there. The environment rows are written in the
  order the person put them. The two families the screen names — IBM Plex Mono
  and Zen Kaku Gothic New, both under the OFL — are kept beside the page under
  `/static` and read with `@font-face`, so the page's own policy still allows
  them and the text shows the fonts the design asks for. The old
  `/profiles/<name>` path moves to the opened row. `hergy validate` reads
  profiles with the same one-pass check, so a screen and the command line say
  the same sentence, and reports an environment variable written twice.

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
