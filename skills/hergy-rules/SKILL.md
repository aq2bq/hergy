---
name: hergy-rules
description: "Use when a person wants to write, change or move the rules of a hergy team (its agreements and its role files): the questions are asked one at a time and the person never writes YAML. It covers a new team, an existing team, moving rules out of Markdown, and filling in origins later from the git and gy history, asking only whether a candidate is right. For booting, stopping or inspecting a team, read the hergy skill instead."
---

# hergy-rules

A hergy team's rules are files: a team's agreements in `protocol.yaml`, and one file per role in `roles/<role>.yaml`. They are YAML, but the person you are working with does not write them. You ask, they answer, and you write the file.

This skill is for the person's side of that: writing a new team's rules, changing an existing team's rules, moving rules out of Markdown, and filling in the origins later. The shape of every key is in `hergy cheat --ai`; the format and the checks are what this skill walks through.

## What you keep to

1. **The person writes no YAML and no long prose.** You ask one thing at a time, with the choices and your recommendation: for the text of a rule you offer a draft, and they choose or correct it. **An origin is never asked for from memory.** You build a candidate from the records — the git history and the gy ledger — and they answer yes, different, or I do not know, one rule at a time. Nobody has to remember on the spot.
2. **You invent no origin, and no check.** A candidate you bring is a quotation from the records, never a story you assemble, and an empty origin is honest where a made-up one is a lie the file will carry. `hergy validate` then counts the rule as one nobody can account for, which is what it is. The same goes for the check: if nobody can say which report field or test shows the rule was kept, write neither.
3. **You check what you wrote before you put it down.** Run `hergy validate`, and show the person the prompt of each member with `hergy up <team> --dry`. A rule that no member reads, or a rule whose text lost a sentence, is a bug in the file.
4. **Only the person removes a rule.** Deciding that an old rule is dead, and deleting the Markdown once it has moved, are the person's calls. Ask; do not tidy up on your own. An origin that stays empty is not a duty: it marks a rule nobody can account for, which is a reason to look at that rule, not a reason to invent an origin for it.

## The four flows

### Write a new team

1. Write the starter files with `hergy new <team> --local` (without `--local` it writes into the global layer; start local). You get `team.yaml`, `protocol.yaml`, `roles/lead.yaml` and `roles/builder.yaml`.
2. Ask, one at a time: what is this team for; who are its members and what is each one's role; what does each role do that no other role does. Write the members and the roles into `team.yaml`.
3. For each rule, ask in this order:
   - the text: you draft one or two sentences from what they said, and read it back for a yes or a correction;
   - the origin: is this rule here because something went wrong (an incident, with its date), because someone asked for it (an instruction, whose and when), or because it follows from another rule (which id)? If the person does not know, write no origin: it can be filled in later, and `hergy validate` keeps counting the rule as one nobody can account for;
   - the reason (`why`): the sentence a member needs to judge a case the rule does not cover. Ask whether the members should read it: if not, keep it in `why` and set `send_why: false` — a reason that is not sent is still a reason, and `hergy validate` counts it. A `note` is something else: a memo for whoever maintains the file, never sent, and never counted as a reason;
   - the check: is there a report field that shows the rule was kept (write it under `reports`), a test, or neither? Neither is an answer.
4. Finish with item 3 of "What you keep to": `hergy validate`, then `hergy up <team> --dry` for the person.

### Fix an existing team

1. Run `hergy validate`. It ends with a count of what the rules carry and then lists, file by file, the id of every rule with no origin. Start from that list.
2. Fill in the origins with "Fill in the origins later": you build a candidate out of the records, and the person answers yes, different, or I do not know. Never ask them what they remember — the records answer first. A rule the records say nothing about stays empty, and that emptiness is the mark of a rule nobody can account for: bring it to the person as a rule to keep, rewrite or drop, with your recommendation.
3. Ask the same for the reason and for the check, in that order. Here the text is in front of you, so you are asking what the rule should say, not what happened once.
4. Finish with item 3 of "What you keep to".

### Move rules out of Markdown

Older teams keep their rules in Markdown (`protocol.md`, `roles/<role>.md`). hergy does not read Markdown any more: a `docs` entry that names a `.md` file, a directory that holds only `.md`, or a role that has only `<role>.md` stops with a message that says so. You move the rules by hand, with the person, in this order. Do the mechanical part yourself: everything except the text comes out of the Markdown, so the moving itself is a job you finish without asking, and the origins are left to "Fill in the origins later".

1. **Split the Markdown into units.** A unit is one of: a bulleted or numbered item; a table cell (a cell, not the whole row, and not the header row); a bold sentence inside a paragraph; or a paragraph with no bold. A heading and an HTML comment are not units: a heading becomes a `heading`, and a comment never reached the agents — the reason it holds is what step 3 moves. Count the units per file; that count is what you will check at the end.
2. **Group the units into rules and give each one an id.** Short, uppercase, per file: a prefix for the file and section, then a number (`TEAM-TURN-1`, `SPEC-AUD-5`). The order of the file is the order of the sections and of the rules inside them, as in the Markdown. A Markdown heading becomes a section `heading` (its depth, when the structure needs it, becomes `level`) — a heading is not a rule, and a rule is not a heading. A paragraph usually stays one rule even when only its bold sentence is the point: the bold sentence is the unit you will check at the end, not the boundary of the rule. A table becomes one rule per row, and its cells are separate units when you check at the end. If the text of a rule points at another rule by number ("rule 3", "the seven rules above"), rewrite it with the id. If a section's rules are pointed at by id, or the file reads better with them, use `style: ids` so the ids are in the prompt too. Watch for a rule that has grown a reason inside its own sentence: the reason becomes `why` (with `send_why: false` when the members should not read it) and leaves the text.
3. **Move the origin out of the text.** A date, a case number or "on the instruction of ..." written inside the sentence becomes an `origin` entry: `kind: incident` with the date and a reference, or `kind: instruction`, or `kind: derived` with `refs`. Take it out of the text. A reason in an HTML comment becomes `why` with `send_why: false`, because comments never reached the agents and the reason is not meant to.
4. **Take rules out of `intro`.** An `intro` is a paragraph before the rules and is not a rule. If an intro carries a rule ("communication is the handing over of the turn"), make it a rule of its own with an id, and leave the intro as the lead-in it was meant to be.
5. **Write the new files beside the old ones** — `<name>.yaml` next to `<name>.md` — so nothing is lost while you work. Then point `team.yaml`'s `docs` and `roles` at them: a directory in `docs` now means its `*.yaml` files, in name order, and a role is looked up as `<role>.yaml`. Write no `revision` unless the person asks for one: moving rules is not raising a version.
6. **Check what moved.** Run `hergy validate`, then take `hergy up <team> --dry --json`. For each member, its `sources` name the files it reads; every unit you counted in step 1 must appear in that member's `prompt`. Compare with the Markdown's marks dropped: `**`, backticks, the `|` of a table and whitespace, all gone. A table cell is checked on its own, because the row it was in is written as a list. Report the number of units that did not appear. Keep fixing until that number is 0. Two differences are not losses: a table becomes a list, and a line break may move.
7. **Leave the origins that are not written in the text empty.** Moving rules does not ask for origins: a date, a case number or "on the instruction of ..." written inside a sentence moves to `origin` (step 3), and nothing else does. The rules that end up with no origin stay that way, and "Fill in the origins later" can bring a candidate for them from the records — later, or not at all. Moving rules is a mechanical job, and a person asked to remember 80 origins will stop halfway.
8. **Ask about the old Markdown.** Once `hergy up <team> --dry` is clean and the person is satisfied, ask whether to delete the `.md` files. Deleting them is theirs to decide.

### Fill in the origins later

An origin that is empty is not a broken rule: the rule works, and the emptiness marks the one thing nobody can judge yet — whether the rule is still needed. Filling origins in is a job you do from the records, not from the person's memory. You bring a candidate; they say whether it is right.

1. Run `hergy validate` and take the rules it lists as having no origin. Work through them one at a time.
2. Build the candidate for one rule out of the records, not out of the conversation:
   - the commit that put the rule there: `git log -S "<a phrase from the rule's text>"` finds the commit that added or changed that phrase, and `git blame` points at the line. The date, the commit message, and any pull request or issue number in them are your material;
   - the ledger: `gy list` around that date, and the decisions whose text uses the same words as the rule, give a case number.
   Write the candidate out in full: `kind` (`incident` or `instruction`; `derived` only when the text itself says which rule it came from), `date`, `refs`, and a one-line `note` that quotes what you found.
3. Show the one candidate and ask one question: is this right, different, or not known? "Different" means the person knows something the records do not — take their words and build the candidate again. "Not known" leaves the origin empty. One rule at a time; never the whole list.
4. A rule whose records say nothing gets no candidate and no question: leave it empty. Do not assemble a story to fill the gap (item 2 of "What you keep to"). An empty origin is a fact about the records, and it is the reason that rule shows up in the list again next time.
5. Stop when the person stops. Origins left empty stay empty, and every rule is usable as it is.

## What good looks like at the end

- `hergy validate` exits 0, and its count says where each rule came from, or says plainly that nobody knows.
- `hergy up <team> --dry` shows each member a prompt that holds every rule that member used to read, with its reason where the reason belongs and without the maintenance notes.
- The origins that are filled in are quotations from the records; the rest are honestly empty, and each empty one names a rule to look at, not a gap to paper over.
- The person answered questions; they did not write YAML.
