---
name: hergy-rules
description: "Use when a person wants to write, change or move the rules of a hergy team (its agreements and its role files): the questions are asked one at a time and the person never writes YAML. It covers a new team, an existing team, and moving rules out of Markdown. For booting, stopping or inspecting a team, read the hergy skill instead."
---

# hergy-rules

A hergy team's rules are files: a team's agreements in `protocol.yaml`, and one file per role in `roles/<role>.yaml`. They are YAML, but the person you are working with does not write them. You ask, they answer, and you write the file.

This skill is for the person's side of that: writing a new team's rules, changing an existing team's rules, and moving rules out of Markdown. The shape of every key is in `hergy cheat --ai`; the format and the checks are what this skill walks through.

## What you keep to

1. **The person writes no YAML and no long prose.** You ask one question at a time, with the choices and your recommendation. The person chooses or answers in a few words. You draft the text of a rule from their words, and they choose it or correct it.
2. **You invent no origin, and no check.** If the person does not say what an accident was, or when, do not write a date, a case number or a story. A rule with no origin is a rule `hergy validate` counts as having none, which is exactly what it is: a rule nobody can account for. Leaving it empty is honest; filling it in is a lie the file will carry. The same goes for the check: if nobody can say which report field or test shows the rule was kept, write neither.
3. **You check what you wrote before you put it down.** Run `hergy validate`, and show the person the prompt of each member with `hergy up <team> --dry`. A rule that no member reads, or a rule whose text lost a sentence, is a bug in the file.
4. **Only the person removes a rule.** Deciding that an old rule is dead, and deleting the Markdown once it has moved, are the person's calls. Ask; do not tidy up on your own.

## The three flows

### Write a new team

1. Write the starter files with `hergy new <team> --local` (without `--local` it writes into the global layer; start local). You get `team.yaml`, `protocol.yaml`, `roles/lead.yaml` and `roles/builder.yaml`.
2. Ask, one at a time: what is this team for; who are its members and what is each one's role; what does each role do that no other role does. Write the members and the roles into `team.yaml`.
3. For each rule, ask in this order:
   - the text: you draft one or two sentences from what they said, and read it back for a yes or a correction;
   - the origin: was there an accident (what happened, when), an instruction (whose, when), or is it derived from another rule (which one — use its id)? "I do not know" is an answer: write no origin;
   - the reason (`why`): the sentence a member needs to judge a case the rule does not cover. Ask whether it should be read by the members: if the reason is for the people maintaining the file, write it as a `note` instead;
   - the check: is there a report field that shows the rule was kept (write it under `reports`), a test, or neither? Neither is an answer.
4. Finish with item 3 of "What you keep to": `hergy validate`, then `hergy up <team> --dry` for the person.

### Fix an existing team

1. Run `hergy validate`. It ends with a count of what the rules carry and then lists, file by file, the id of every rule with no origin. Start from that list.
2. Take one rule at a time and ask what happened that put the rule there. The choices are: an accident (when, what happened), an instruction (whose, when), derived from another rule (which id), or unknown. For a rule nobody can account for, ask whether to keep it, rewrite it, or drop it — with your recommendation.
3. Ask the same for the reason and for the check, in that order.
4. Finish with item 3 of "What you keep to".

### Move rules out of Markdown

Older teams keep their rules in Markdown (`protocol.md`, `roles/<role>.md`). hergy does not read Markdown any more: a `docs` entry that names a `.md` file, a directory that holds only `.md`, or a role that has only `<role>.md` stops with a message that says so. You move the rules by hand, with the person, in this order. Do the mechanical part yourself; ask the person only about what cannot be read out of the text.

1. **Split the Markdown into units.** A unit is one of: a bulleted or numbered item, a table row (its own line, not the header), a bold sentence inside a paragraph, or a paragraph with no bold. Drop HTML comments: they hold reasons that were not sent to the agents. Count the units per file; that count is what you will check at the end.
2. **Group the units into rules and give each one an id.** Short, uppercase, per file: a prefix for the file and section, then a number (`TEAM-TURN-1`, `SPEC-AUD-5`). The order of the file is the order of the sections and of the rules inside them, as in the Markdown. A Markdown heading becomes a section `heading` (its depth, when the structure needs it, becomes `level`) — a heading is not a rule, and a rule is not a heading. A paragraph usually stays one rule even when only its bold sentence is the point: the bold sentence is the unit you will check at the end, not the boundary of the rule. A table becomes one rule per row. If the text of a rule points at another rule by number ("rule 3", "the seven rules above"), rewrite it with the id. If a section's rules are pointed at by id, or the file reads better with them, use `style: ids` so the ids are in the prompt too. Watch for a rule that has grown a reason inside its own sentence: the reason becomes `why` (or `note`) and leaves the text.
3. **Move the origin out of the text.** A date, a case number or "on the instruction of ..." written inside the sentence becomes an `origin` entry: `kind: incident` with the date and a reference, or `kind: instruction`, or `kind: derived` with `refs`. Take it out of the text. A reason in an HTML comment becomes `why` with `send_why: false`, because comments never reached the agents and the reason is not meant to.
4. **Take rules out of `intro`.** An `intro` is a paragraph before the rules and is not a rule. If an intro carries a rule ("communication is the handing over of the turn"), make it a rule of its own with an id, and leave the intro as the lead-in it was meant to be.
5. **Write the new files beside the old ones** — `<name>.yaml` next to `<name>.md` — so nothing is lost while you work. Then point `team.yaml`'s `docs` and `roles` at them: a directory in `docs` now means its `*.yaml` files, in name order, and a role is looked up as `<role>.yaml`.
6. **Check what moved.** Run `hergy validate`, then take `hergy up <team> --dry --json`. For each member, its `sources` name the files it reads; every unit you counted in step 1 must appear in that member's `prompt`. Compare with the markdown stripped: drop `**`, backticks and whitespace before you look. Report the number of units that did not appear. Keep fixing until that number is 0. Two differences are not losses: a table becomes a list, and a line break may move.
7. **Then ask about what you could not read.** The rules that ended up with no origin go through "Fix an existing team", now or later — ask which.
8. **Ask about the old Markdown.** Once `hergy up <team> --dry` is clean and the person is satisfied, ask whether to delete the `.md` files. Deleting them is theirs to decide.

## What good looks like at the end

- `hergy validate` exits 0, and its count says where each rule came from, or says plainly that nobody knows.
- `hergy up <team> --dry` shows each member a prompt that holds every rule that member used to read, with its reason where the reason belongs and without the maintenance notes.
- The person answered questions; they did not write YAML.
