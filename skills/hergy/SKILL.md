---
name: hergy
description: "Use when you start or close a team with hergy, write a hergy team definition, or work inside a team hergy started. The exact shape of every command and flag is in hergy cheat --ai (one command's page: hergy <command> --help); read it there."
---

# Working with hergy

Use hergy to choose the team and its rules. Read `hergy cheat --ai` before starting or joining a team, and use `hergy <command> --help` for a command's syntax.

## Before you run one

Show the person a dry plan before starting a team or applying changes. Keep a starting command in the background and read its output so you can tell the person which pane needs an answer. Do not answer startup questions yourself, send keys, change trust settings, or move the person's focus.

Start settings work in the project's `.hergy/`; ask the person before changing `~/.config/hergy/`. Before removing a worktree, check its uncommitted changes and commits not in main. When a document is too long, look for duplicate or unnecessary rules before cutting meaning.

When displaying environment variables, pass one name per `printenv` call. Check the herdr environment with the exit status of `test "${HERDR_ENV:-}" = 1`.

