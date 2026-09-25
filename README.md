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
`~/.local/bin/ontos`, links `skills/*/` into `~/.claude/skills/`, and prints the
`Bash(ontos:*)` permission, the `SessionStart` hook and the `CLAUDE.md` lines to add by hand.
The script never edits Claude Code's files itself.

### Setting up with an agent

Point an agent at this section. It needs `go` on `PATH` (`nix develop` in this checkout).

1. Run `./scripts/setup.sh` and fix any warning it prints.
2. If the config was `(created)`, ask the user whether `store` should be a synced folder
   instead of `~/ontos`; if so, edit it and re-run the script.
3. Apply the block the script prints at the end. Merge it into the existing files: keep every
   other permission, hook and line, and skip what is already there.
4. Run `ontos subject list` to check the store is readable; it must exit 0. Tell the user the
   hook loads from the next session or `/clear`, and that `/ontos-bootstrap` builds a repo's
   first entries.

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

## Bootstrapping a subject

In Claude Code, `/ontos-bootstrap` in a repo researches it, checks each claim against the code,
shows every proposed entry in one table, and writes them after one yes. Run it again later and
it re-verifies what is stored before adding anything. `/ontos-bootstrap <path to .claude-docs>`
ports an old doc set the same way.
