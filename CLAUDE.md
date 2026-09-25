# CLAUDE.md

This repo holds the `ontos` command and, later, its skills. It does not hold the knowledge:
that lives in the store named by `store` in `~/.config/ontos/config.toml`, usually a
cloud-synced folder. [DESIGN.md](DESIGN.md) is the contract; [README.md](README.md) is the
tour.

Commits are single-line conventional-commit titles. Scope is `ontos` for the command, `setup`
for `scripts/setup.sh`, `design` for `DESIGN.md`, or the skill name.
Branch is `master`.

## Setup

`flake.nix` pins the toolchain the checks below need (Go, `golangci-lint`, `shellcheck`, `shfmt`)
in a dev shell, and `.envrc` loads it on `cd`. `./scripts/setup.sh` is idempotent: it seeds the
config, creates the store, builds `~/.local/bin/ontos`, links `skills/*/` into
`~/.claude/skills/`, and prints the settings to add by hand. **Re-run it after every edit to
the Go sources**: the hook runs the installed binary, not this checkout.

## The Go code

```
cmd/ontos/main.go          dispatch, usage, run() (main without the exit), flag helpers
cmd/ontos/entries.go       search, get, add, update, delete
cmd/ontos/subjects.go      subject add, update, list, which
cmd/ontos/context.go       context, which never fails
internal/ontos/config.go   the config, the eight categories, ONTOS_CONFIG
internal/ontos/store.go    Load, schema check, atomic writes, Writable
internal/ontos/entry.go    Entry, validation, ids, delete with related cleanup
internal/ontos/subject.go  Subject, file-name escaping, NormalizeURL
internal/ontos/git.go      Locate: toplevel, path in repo, origin remote
internal/ontos/resolve.go  directory -> subject, recording every step
internal/ontos/search.go   term matching, scoring, filters
internal/ontos/context.go  what a session starts with, and its markdown
internal/ontos/rules.go    the maintenance rules context prints
internal/ontos/conflicts.go sync-conflict file names, copied from pragma
```

Rules that hold the design in place:

- **The CLI is the only writer.** The storage format is internal; nothing else reads or writes
  the store's files. Every write goes through `writeAtomic` (temp file in the same directory,
  sync, rename), so a sync client never picks up half a file.
- **Facts in the store are present tense.** No history, no supersede, no digests: a change
  overwrites, and an entry that stops being true is deleted. Do not add fields that only make
  sense with history.
- **`context` always exits 0.** The `SessionStart` hook runs it on every start, resume, `clear`
  and `compact`, so a failure there would get in the way of every session. `run` returns 0 for
  it before any error handling. `BuildContext` has no error return, and anything missing
  becomes one line naming the fix. Keep both properties; `TestContextWithoutConfigExitsZero`
  and `TestContextBadArgumentsExitZero` pin them.
- **Reads survive a bad file; writes do not.** A file that fails to load becomes a
  `Store.Problems` line, which `context` prints and other verbs send to stderr. Every write
  checks `Writable` first, so nothing is written over a store that isn't fully read.
- **Errors name the command that fixes them.** An agent reads them, and a bare failure gets
  improvised around.
- **Standard library over new dependencies.** The only one is `BurntSushi/toml`.
- **Go's `flag` stops at the first positional argument**, so every verb parses through
  `parseArgs`. A verb calling `fs.Parse` directly silently ignores flags after the id.
- **A flag not given leaves the field alone.** `update` and `subject update` decide that with
  `given(fs)`, never by comparing against a zero value, so an empty value can clear a field.
- Test against a scratch store: `ONTOS_CONFIG=<file> ontos …`, and `scripts/setup.sh`
  honours it too. The CLI tests run `run()` in-process against a temp store.

## Checks

```
test -z "$(gofmt -l .)"
go vet ./...
go test ./...
golangci-lint run ./...
shellcheck -s bash scripts/setup.sh
shfmt -i 2 -d scripts/setup.sh
```

`gofmt -l` lists unformatted files but exits 0, so the emptiness test is what makes it fail.
`shfmt` with no path reads stdin, so it needs the file named.
