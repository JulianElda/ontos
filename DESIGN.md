# ontos: design

A knowledge base for Claude Code. Claude reads it at session start and writes to it as it works,
through a Go CLI. It replaces a per-repo, gitignored `CLAUDE.local.md` + `.claude-docs/` setup
(a routing table plus task docs, symlinked into a cloud-synced folder) with one store that the
CLI owns.

Status: v1 implemented; this is the contract.

## Background

- **Inspiration:** Notion's Lore (`github.com/makenotion/lore`, cloned at `~/temp-repos/lore`), an
  AI memory backed by Notion with an MCP server, a CLI and hooks. ontos keeps its three-layer
  shape (one core, and thin CLI and hook front ends) and leaves out almost everything else: the
  Notion backend, the facts graph, history and supersede, tasks, decisions, digests, conflict
  and debt scans, and the background autosave agent.
- **Worth reading in Lore:** `src/hooks/prompts.ts` (`buildExtractionFilter`, the "inferability
  test" and the precision bar), `docs/hooks-wakeup.md`, `docs/hooks-autosave.md`,
  `src/notion/schema.ts` (the overloaded Memories table, as a warning).
- **Replaced setup:** a skill that researched a repo with parallel forks, wrote 5–9 task docs
  (50–150 lines) verified against the code, and kept a routing table ("doing X → read Y") that
  was loaded every session with standing rules to keep the docs current. The docs get ported
  into ontos.
- **Neighbors:** `~/workspace/pragma` owns plans, tasks, open work and its own history. Claude
  Code's built-in auto memory stays, holding preferences and feedback. ontos holds neither.

## Principles

1. **Present tense only.** An entry describes how things are now. A change overwrites it, and an
   entry that stops being true is deleted. No history, no supersede chains, no digests.
2. **Built for Claude.** Categories follow the moment Claude needs the knowledge, not the
   source folder it came from.
3. **Store only what's costly to work out again.** Anything a single `rg` answers doesn't belong.
   Explanations verified against code are fine; that's the main thing the old docs held.
4. **One fact, one home.** Knowledge shared by several repos belongs to a shared subject, not
   to copies in each repo.
5. **The CLI is the only writer.** The storage format is internal.

## Categories

| Category | Claude reads it when… | Contains | Generic example |
| --- | --- | --- | --- |
| `map` | Arriving in a repo, package or subject | What exists, what owns what, who calls whom, where things live, what a part does *not* do | "The web app consumes `display.*` events from the broker; it never writes to the DB" |
| `mechanism` | Reasoning about behavior | End-to-end flows, lifecycles, sequences, timing, verified against code | "Startup: settings → auth → broker connect → first layout fetch" |
| `invariant` | Before editing | Rules the code depends on, and code that looks wrong but is deliberate, each with *because* | "Handlers must stay idempotent: the broker redelivers on reconnect" |
| `gotcha` | Before using a tool, library or environment | Surprising behavior, silent failures, obvious approaches that fail here | "The client's `subscribe()` while disconnected silently does nothing" |
| `procedure` | Doing a routine task | Exact commands in order, with expected results | "Prove which build is live on the dev VM: …" |
| `diagnosis` | Something is wrong | The symptom as seen → likely causes → the check that confirms each → the fix. The title is the symptom | "Spinner forever, no layout" |
| `gap` | Judging a bug or trusting tests | Known defects, *suspected* defects (marked), untested areas, deliberate non-coverage, committed docs that contradict the code | "Suspected: a reconnect during a frame swap drops one message" |
| `reference` | Needing something outside the repo | Tools and skills and which question each answers, environments, dashboards, specs, domain terms | "`msg-tail`: live tail of broker messages for a session" |

How to classify something: how things stand or run → `map`/`mechanism`; limits what Claude may
change → `invariant`; will surprise Claude → `gotcha`; steps → `procedure`; starts from a
symptom → `diagnosis`; broken or unreliable → `gap`; points outside → `reference`.

An in-progress migration gets split: the direction becomes an `invariant` ("new code uses the new
design system"), the remaining work goes to pragma, and the list of call sites isn't stored
(`rg` finds them).

## Data model

Two record types, each stored as JSON in the synced store.

**Subject:** what knowledge is about.

| Field | Notes |
| --- | --- |
| `name` | Key, unique within a store. Defaults to the repo name. Entries refer to subjects by name, so renaming a repo only touches `url` |
| `url` | Optional canonical git remote: `github.com/org/repo` (scheme, user and `.git` stripped, `:` → `/`, host lowercased). Resolved from cwd → git toplevel → `remote.origin.url` |
| `path` | For packages: path relative to the repo root (`packages/logger`); name `repo/packages/logger` |
| `uses` | Other subjects this one inherits entries from ("app uses broker-client, auth-lib, dev-vm") |

A subject without a URL (for example `nix`, a protocol, an environment) comes into play through
`uses`, through search, or when named explicitly.

**Entry:** one piece of knowledge, about the size of a doc section (5–40 lines).

| Field | Notes |
| --- | --- |
| `id` | Stable and random, not derived from the title: 8 lowercase base32 characters (`a-z2-7`, ~40 bits), regenerated if taken. Claude types them, and a ULID's sort order would go unused in a store with no history |
| `title` | Worded the way Claude would look it up; for `diagnosis`, the symptom |
| `category` | One of the eight |
| `subjects` | Subject names |
| `trigger` | One line, "read when…"; replaces the old routing-table line |
| `tags` | Topics that cut across categories (`messages`, `auth`) |
| `certainty` | `confirmed` / `inferred` / `suspected` |
| `checked` | sha, date or `dep@version`: what the entry was last verified against. Not history |
| `sources` | Repo paths, no line numbers |
| `related` | Entry ids; point to them instead of copying |
| `body` | Markdown, tables allowed |

Also a schema version on each file.

## Storage and config

- **Store:** a cloud-synced folder with one JSON file per entry, plus subject JSON. The CLI loads
  everything and searches in memory; hundreds of entries is under 1 MB. One file per entry keeps
  a sync conflict limited to that entry. No SQLite in the synced folder, because sync clients
  corrupt live database files.
- **Separate stores for work and personal use,** in different clouds. They're never merged or
  routed between. Each machine has exactly one store.
- **Config:** `~/.config/ontos/config.toml`, local to each machine and not synced. It holds the
  store path and context loading. Nothing machine-specific goes into the store.

```toml
store = "~/Cloud/ontos"

[context]
full  = ["map"]
index = ["mechanism", "invariant", "gotcha", "procedure", "diagnosis", "gap", "reference"]
```

`full` categories are printed in full at session start. `index` categories get one line per
entry (title, trigger, id). Removing a category from both means Claude has to search for it.

## CLI (v1)

| Verb | Does |
| --- | --- |
| `context [path]` | Resolves the subject from cwd (or `path`), adds `uses`, and prints `full` entries, index lines and the maintenance rules |
| `search [query]` | Full-text search over title, trigger and body, with `--subject`, `--category`, `--tag` filters. Returns id, title, category and trigger, **no bodies**. With no query it's a filtered list ("everything on subject X") |
| `get <id>…` | Full entries, several at once |
| `add` | Creates an entry; body through stdin |
| `update <id>` | Changes fields; fields left out stay unchanged; overwrites |
| `delete <id>` | Removes an entry that's no longer true. Required, since there's no history |
| `subject add/update/list/which` | Manages subjects; `which` explains how cwd resolved |

Output is markdown by default, with `--json` as an option. Bodies go through stdin, because
markdown in shell arguments breaks quoting. Writes are atomic (temp file + rename), and the CLI
should notice sync-conflict copies.

## Claude Code integration

- **One hook:** `SessionStart` → `ontos context`. It fires on startup, resume, `clear` and
  `compact`, and its stdout is added to Claude's context.
- **Root `~/.claude/CLAUDE.md`:** a short instruction to use ontos for repo knowledge
  (preferences stay in auto memory, plans in pragma).
- **Permission:** `Bash(ontos:*)`.
- **Writing happens in-session.** Claude writes through the CLI whenever the work calls for it,
  without asking. There's no background agent in v1.

### Maintenance rules (printed by `context`)

Ported from the replaced skill; one copy lives in the tool (`internal/ontos/rules.go`) rather than in each repo:

- An entry is wrong or out of date: `update` or `delete` it in the same session.
- You worked out something the next session would have to rediscover: `search` first, then
  `update` the matching entry or `add` a new one.
- A new kind of task that no entry covers: `add` one with a trigger worded as the situation.
- An entry grew past one question: split it into several `add`s plus a `delete`.
- Be precise: state the exact trigger and the exact rule. If you can't, don't write it.
- `confirmed` only for what you ran or read in code; otherwise `inferred` or `suspected`. Never
  write a suspected bug as fact.
- Never store secrets, tokens, credentials, or URLs with credentials in them.
- Plans and open work go to pragma; personal preferences go to auto memory.

## Bootstrap skill

A separate skill in the ontos repo, `skills/ontos-bootstrap/SKILL.md`, symlinked into
`~/.claude/skills/` the way pragma does it and run as `/ontos-bootstrap`. It builds a subject's
entries from scratch: researches the repo with parallel forks, verifies against code, and writes
through the CLI. It's also used to port existing `.claude-docs`, sorting each doc section into
categories. That sorting is a judgment call, not parsing, so a skill does it rather than an
`import` command.

- **Two modes, chosen by what the store holds.** A subject with no entries is built. A subject
  with entries is re-verified against HEAD first (each entry updated, deleted, or its `checked`
  refreshed), then filled in. This is the manual form of the v2 `stale` candidate.
- **Forks return candidates and never write.** Only the main agent calls the CLI, so duplicate
  checks and certainty are decided in one place: `confirmed` only for what it read or ran itself.
- **One approval.** All candidate entries, verdicts and new subjects are shown as one table and
  written after a single yes. This is the exception to "writing happens without asking": a batch
  of dozens of entries in a store without history costs too much to clean up by hand.
- **Inferability.** Lore skips anything a competent agent could reproduce by reading the code.
  Bootstrap works from the code by definition, so it takes principle 3's line instead: skip what
  a single `rg` answers, keep explanations that cost a real read to work out.

## Implementation

- Go, laid out like pragma: `github.com/JulianElda/ontos`, `cmd/ontos/`, `internal/ontos/`,
  `BurntSushi/toml` for config, stdlib `encoding/json`, `flake.nix`, `scripts/setup.sh`.
- A single static binary; starts fast, which matters because the hook runs on every session.

## Monorepo loading

Sessions don't always start in the package directory, so `context` has to decide how much of a
monorepo to load up front. The store and the verbs are the same in every variant below; only
`context` output and timing change.

| Variant | At session start | Moving into a package |
| --- | --- | --- |
| a. Everything up front | Root + every package's `map` | Nothing |
| **b. Root + package index (chosen)** | Root + one line per package pointing to `ontos context <path>` | Claude runs it |
| c. On file access | Root only | A `PostToolUse` hook on Read/Edit adds the package's context the first time a file there is touched (mirrors how nested `CLAUDE.md` loads) |

**v1 implements (b).** At a repo root, `context` prints the root subject plus a "Packages" list,
`- <name>: ontos context <path>`, one line per subject with the same url and a non-empty path.
In a package, it prints the package subject in full plus the root subject's index lines (not
its `full` entries). (b) is the cheapest: it costs one line per package and no extra hook. (a)
and (c) stay open for later without changing the store.

## v2 candidates

- Background transcript pass (`SessionEnd` + `PreCompact` → detached `claude -p` limited to
  `Bash(ontos:*)`), if Claude turns out to forget to write in-session. It would need: untrusted-
  transcript framing, a secrets filter, a certainty cap (discussed ≠ confirmed), search before
  adding, a recursion guard in the environment, and a smaller model.
- `stale`: entries whose `sources` changed since `checked`
  (`git diff --name-only <sha>..HEAD -- <sources>`).
- Operation log (per-machine JSONL like pragma's), for debugging, never read by Claude.
- An optional `review_by` field for `reference` entries, which diffs can't catch going stale.
- A local SQLite FTS index cache (`modernc.org/sqlite`), only if loading gets slow on a slow
  filesystem.
- An MCP server, only if the CLI's scale calls for typed schemas. It would be a thin layer over
  the same core, like Lore's.
