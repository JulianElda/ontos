# ontos

> To know wisdom and instruction; to perceive the words of understanding.

Repo knowledge for Claude Code: how code is laid out, how it works, what breaks it, and how to
check. Claude reads it at session start through a `SessionStart` hook, and writes to it during
the session through a Go CLI, which is the store's only writer.

[DESIGN.md](DESIGN.md) is the contract: the categories, the data model, and why the store
looks the way it does. [CLAUDE.md](CLAUDE.md) covers working on this repo.

## Setup

```
./scripts/setup.sh
```

It seeds `~/.config/ontos/config.toml` (never overwriting it), creates the store it names
(`~/ontos` by default; point `store` at a synced folder and re-run), builds
`~/.local/bin/ontos`, and prints the `Bash(ontos:*)` permission, the `SessionStart` hook and
the `CLAUDE.md` lines to add by hand.

## The verbs

| Verb | Does |
| --- | --- |
| `ontos context [path]` | The session-start context: the resolved subject's `full` entries and index lines, its `uses`, the packages below a repo root, and the maintenance rules. Always exits 0 |
| `ontos search [query]` | Id, title, category and trigger of matching entries, never bodies. `--subject`, `--category`, `--tag` filter |
| `ontos get <id>…` | Entries in full |
| `ontos add …` | A new entry; the body through stdin with `--body-file -`. Prints the id |
| `ontos update <id> …` | Changes only the fields given; a list flag replaces the list |
| `ontos delete <id>` | Removes an entry and every `related` pointer to it |
| `ontos subject add/update/list/which` | Manages subjects; `which` shows how a directory resolved |

Every read verb takes `--json`. `ontos help` lists every flag.

## A first subject

```
cd ~/workspace/some-repo
ontos subject add                              # name, url and path from cwd
cd packages/logger && ontos subject add        # a package: some-repo/packages/logger
ontos subject add --name dev-vm --url ''       # something with no repo
ontos subject update some-repo --uses dev-vm
echo 'Levels are read once at startup.' | ontos add --title 'Log level changes need a restart' \
  --category gotcha --subject some-repo/packages/logger --trigger 'before changing a log level' \
  --certainty confirmed --body-file -
ontos context
```
