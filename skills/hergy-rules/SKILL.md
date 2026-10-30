---
name: hergy-rules
description: "Use when a person wants to write, change or move the rules of a hergy team (its agreements and its role documents): the questions are asked one at a time and the person never writes YAML. It covers a new team, an existing team, moving rules out of Markdown, turning a rule off, and writing the note a person leaves with a rule. For booting, stopping or inspecting a team, read the hergy skill instead."
---

# hergy-rules

A hergy team's rules are files: one team's rules in `teams/<team>/agreement.yaml` and one file per role in `roles/<role>.yaml` (under the project's `.hergy/` or, outside it, `~/.config/hergy/`). The agreement every team used to share (`agreement.yaml` beside the teams) is gone: while one is left, `hergy up`, `refresh`, `reprompt` and `hire` stop and name it, and hergy validate reports it, so move its rules into the team that read them or into a role document and remove the file. They are YAML, but the person you are working with does not write them. You ask, they answer, and you write the file.

This skill is for the person's side of that: writing a new team's rules, changing an existing team's rules, moving rules out of Markdown, turning a rule off, and writing a note. The shape of every key is in `hergy cheat --ai`; the format and the checks are what this skill walks through.

The rules can also be changed on the screen: `hergy serve` shows the role documents at `/roles` and a team's own agreement on the team's row at `/teams` (its ルール tab). Both open a rule in place, switch it off and on, write the day it was switched off, reorder sections and rules, and show the Markdown each member reads. Send the person to the screen for a change they want to make themselves; come back to this skill when the change needs the questions (a new team, a move out of Markdown, or moving the rules of an agreement every team used to share).

A rule holds four things you write: its body (`text`), whether it is on (`in_prompt`), the day it was switched off (`off_since`), and a note for people (`note`). Its id is hergy's: hergy writes it — four characters such as `K7QF`, with no meaning — when it next writes the file, and never changes one already there. Do not write an id and do not change one. The body is what a member reads. The note is never sent: it is where a person writes why the rule was written and what they saw. A rule itself has no separate reason, origin or check field, and a team agreement has no `reports:` block.

## What you keep to

1. **The person writes no YAML and no long prose.** You ask one thing at a time, with the choices and your recommendation: for the text of a rule you offer a draft, and they choose or correct it. You never ask anyone to remember where a rule came from: if the person knows why it was written, that goes in the body when a member needs it, or in the note when it is for people.
2. **You invent no reason.** A note is a quotation from the records or from the person's own words — never a story you assemble. An empty note is honest where a made-up one is a lie the file will carry.
3. **You check what you wrote before you put it down.** Run `hergy validate`, and show the person the prompt of each member with `hergy up <team> --dry`. A rule that no member reads, or a rule whose text lost a sentence, is a bug in the file.
4. **Only the person removes a rule.** Deciding that an old rule is dead, and deleting the Markdown once it has moved, are the person's calls. Ask; do not tidy up on your own. When the person wants to try a team without a rule, offer to switch it off first: the rule stays in the file, leaves every member's prompt, and comes back when it is switched on. On the screen hergy writes the day it was switched off; in a file a person writes `in_prompt: false`, and may add `off_since: YYYY-MM-DD` when the day is known.
5. **Put each sentence where it belongs.** A role document says how the work itself is done, and holds nothing that changes when the team, the tools or the model change. Who decides, who hands the turn to whom, who may not overrule whom, which tools the team uses (gy, git, CI, `hergy hire`) and this project's own facts (paths, sizes) go in the team's agreement (`teams/<team>/agreement.yaml`). Who may make which write to gy goes in `team.yaml` as `ledger:` (`req approve by lead`), next to `turns:`, and `hergy validate` checks it against the ledger. An instruction that only one harness can follow (a Claude Code skill to read, say) is not hergy's to hold: write it in the brief the lead sends, not in a rule. When one rule mixes these, split it.

## The four flows

### Write a new team

1. Write the starter files with `hergy new <team>` (inside a git repository it writes into the project's `.hergy/`; outside one, into `~/.config/hergy/`; start in the project). You get the team's `team.yaml` and `agreement.yaml`, and `roles/lead.yaml` and `roles/builder.yaml`.
2. Ask, one at a time: which case is this team meant for; who are its members and what is each one's role; what does each role do that no other role does. Write the members and the roles into `team.yaml`, and the case the team is meant for as its `description:` — one line, near the top of the file.
3. For each rule, ask in this order:
   - the text: you draft one or two sentences from what they said, and read it back for a yes or a correction. When the rule hands the turn between two of the team — a member name, a role name, or the human — write it as `<name> → <name>`, or `<name> ⇄ <name>` when the turn goes both ways: `hergy validate` checks every such arrow against `turns:`. A rule like this usually belongs in the team's agreement, not in a role (item 5 of "What you keep to");
   - whether the rule is on: does every member need to read it now? If not, write `in_prompt: false` and offer to switch it on later; a rule added for a rare case may still be on, because a rule a member does not read is a rule nobody follows;
   - the note (`note`): is there anything a person should know about why the rule was written or what was seen? It is never sent. If the person has nothing to add, leave it out; do not invent one.
4. Finish with item 3 of "What you keep to": `hergy validate`, then `hergy up <team> --dry` for the person.

### Fix an existing team

1. Run `hergy validate`. It names each broken file, then prints the size of each team definition and role document. A rule that is off is still in the file and still counted; a rule that is on is what a member reads. Start from a file that is named, or from a size that is over the provisional budget.
2. Look for rules that are no longer needed, rules that are duplicated, and rules that could be off. Ask the person about them one at a time. When a rule should leave the prompt, offer to switch it off; when it should be gone, ask before removing it.
3. If a rule's body has grown a reason that only people need, move that sentence to `note`. If the reason is what a member needs to judge a case the rule does not cover, keep it in the body.
4. Finish with item 3 of "What you keep to".

### Move rules out of Markdown

Older teams keep their rules in Markdown (`protocol.md`, `roles/<role>.md`). hergy does not read Markdown any more: a `docs` entry that names a `.md` file, a directory that holds only `.md`, or a role that has only `<role>.md` stops with a message that says so. You move the rules by hand, with the person, in this order. Do the mechanical part yourself: everything the move needs is in the Markdown.

1. **Split the Markdown into units.** A unit is one of: a bulleted or numbered item; a table cell (a cell, not the whole row, and not the header row); a bold sentence inside a paragraph; or a paragraph with no bold. A heading and an HTML comment are not units: a heading becomes a `heading`, and a comment never reached the agents, so it belongs in `note`. Count the units per file; that count is what you will check at the end.
2. **Group the units into rules.** The order of the file is the order of the sections and of the rules inside them, as in the Markdown. A Markdown heading becomes a section `heading` (its depth, when the structure needs it, becomes `level`) — a heading is not a rule, and a rule is not a heading. A paragraph usually stays one rule even when only its bold sentence is the point: the bold sentence is the unit you will check at the end, not the boundary of the rule. A table becomes one rule per row, and its cells are separate units when you check at the end. If the text of a rule points at another rule by number ("rule 3", "the seven rules above"), rewrite it to name the rule by what it says: hergy gives each rule its id only when it writes the file, and a person cannot know it before that.
3. **Keep what the members read in the body.** A reason that stood in the Markdown was read by the agents, so it stays in `text`; the members go on reading it. A date, a case number or "on the instruction of ..." that a member needs stays in the text. If it is only provenance for people, move it to `note`. An HTML comment was never sent: move its reason to `note` rather than to the body.
4. **Take rules out of `intro`.** An `intro` is a paragraph before the rules and is not a rule. If an intro carries a rule ("communication is the handing over of the turn"), make it a rule of its own, and leave the intro as the lead-in it was meant to be.
5. **Write the new files beside the old ones** — `<name>.yaml` next to `<name>.md` — so nothing is lost while you work. Then point `team.yaml`'s `docs` and `roles` at them: a directory in `docs` now means its `*.yaml` files, in name order, and a role is looked up as `<role>.yaml`. Write no `revision` unless the person asks for one: moving rules is not raising a version.
6. **Check what moved.** Run `hergy validate`, then take `hergy up <team> --dry --json`. For each member, its `sources` name the files it reads; every unit you counted in step 1 must appear in that member's `prompt`. Compare with the Markdown's marks dropped: `**`, backticks, the `|` of a table and whitespace, all gone. A table cell is checked on its own, because the row it was in is written as a list. Report the number of units that did not appear. Keep fixing until that number is 0. Two differences are not losses: a table becomes a list, and a line break may move.
7. **Ask about the old Markdown.** Once `hergy up <team> --dry` is clean and the person is satisfied, ask whether to delete the `.md` files. Deleting them is theirs to decide.

### Switch a rule off and on

1. On the screen, switch the rule off. hergy writes `in_prompt: false` and the day it was switched off (`off_since`); the rule keeps its body and the id hergy gave it, and leaves every member's prompt. Switch it back on and hergy removes the off day.
2. In a file, write `in_prompt: false` to switch a rule off. Add `off_since: YYYY-MM-DD` when the day is known; a file without the day is read as off on an unknown day, and hergy does not fill the day in on its own. Write `in_prompt: true` (or take the line out) to switch it on; an off day left on a rule that is on is ignored, and hergy drops it the next time it writes the rule.
3. Run `hergy up <team> --dry` and show the person the prompt without the rule. A rule that is off is not deleted: it stays in the file, in `hergy validate`'s count, and in the eye of whoever maintains the file.

## What good looks like at the end

- `hergy validate` exits 0, and the size it prints is the size of what the members read.
- `hergy up <team> --dry` shows each member a prompt that holds every rule that is on, and no rule that was switched off.
- The notes are quotations from the records or the person's words, and no note is sent to a member.
- The person answered questions; they did not write YAML.
