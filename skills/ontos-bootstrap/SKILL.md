---
name: ontos-bootstrap
description: Build or re-verify a subject's ontos entries from the code, or port existing .claude-docs into them.
argument-hint: "[path | .claude-docs dir]"
disable-model-invocation: true
---

# /ontos-bootstrap

Fill ontos for one subject: research the repo, check every claim against the code,
show the user everything once, then write it through the `ontos` CLI. If the subject
already has entries, check those first. If `$ARGUMENTS` names a `.claude-docs`
directory or a `CLAUDE.local.md`, port those docs instead of researching from scratch.

Nothing is written to the store before step 5's approval. The store keeps no
history, so a bad batch can only be cleaned up one `delete` at a time.

## What an entry is

One question, answered in 5–40 lines, true now. Pick the category by the moment
Claude would need the answer:

| Category | Read when… | Holds |
| --- | --- | --- |
| `map` | arriving in a repo, package or subject | what exists, what owns what, who calls whom, what a part does *not* do |
| `mechanism` | reasoning about behavior | end-to-end flows, lifecycles, sequences, timing |
| `invariant` | before editing | rules the code depends on, and code that looks wrong but is deliberate, each with *because* |
| `gotcha` | before using a tool, library or environment | surprising behavior, silent failures, obvious approaches that fail here |
| `procedure` | doing a routine task | exact commands in order, with expected results |
| `diagnosis` | something is wrong | symptom → likely causes → the check that confirms each → the fix; the title is the symptom |
| `gap` | judging a bug or trusting tests | known defects, *suspected* defects (marked), untested areas, docs that contradict the code |
| `reference` | needing something outside the repo | tools, skills, environments, dashboards, specs, domain terms |

How things stand or run → `map`/`mechanism`; limits what Claude may change →
`invariant`; will surprise Claude → `gotcha`; steps → `procedure`; starts from a
symptom → `diagnosis`; broken or unreliable → `gap`; points outside → `reference`.
An in-progress migration splits: the direction is an `invariant`, the remaining
work belongs in a task tracker, and the list of call sites is not stored.

**Precision bar.** State the exact trigger and the exact rule, falsifiably: "the
client's `subscribe()` while disconnected silently does nothing", never "be careful
with subscriptions". A vague entry misleads worse than none. If you can't state the
rule precisely, don't propose it.

**Skip:**

- anything a single `rg` answers (call-site lists, one function's signature, what a
  file contains)
- anything the repo's committed docs (`CLAUDE.md`, `README`, `docs/`) already state:
  one fact, one home
- plans and open work (a task tracker), preferences and feedback (auto memory)
- secrets, tokens, credentials, and URLs with credentials in them

A fact that several repos share (an environment, a library, a protocol) becomes its
own subject, which each repo lists in `uses`. Don't copy it into each repo.

`map` entries are printed in full at every session start. Keep them short, and point
from them to the deeper entries with `related`.

## 1. Orient

Work in `$ARGUMENTS` if it is a directory in a repo, otherwise in the current one.

```
ontos subject which [path]
ontos context [path]
git -C <path> rev-parse --short HEAD
```

- `which` shows whether a subject resolves. If none does, its last line is the
  subject `ontos subject add` would create in that directory (name, url, path), or
  why it can't create one and what to pass instead. That subject goes into step 5's
  table. If the directory is a package, propose its subject and the repo root's;
  `ontos subject which <toplevel>` gives the root's.
- The short sha is what every entry written or re-verified in this run gets as
  `--checked`.
- **Port:** if `$ARGUMENTS` names a `.claude-docs` directory or a `CLAUDE.local.md`,
  read all of it now. The docs replace step 3's research: every section becomes a
  candidate, and step 4 checks it against the code like any other.

## 2. Re-verify what exists

Skip this step if step 1 found no subject: `search --subject` fails on a subject
that doesn't exist yet. Otherwise, only if `ontos search --subject <subject>` lists
entries (and the same for every subject in `uses` that belongs to this repo).

`ontos get` them, several ids per call, and check each against its `sources` at
HEAD. Give each one verdict:

- **still true**: `update <id> --checked <sha>`
- **changed**: `update <id>` with the corrected body (and title, trigger or sources
  if they moved)
- **no longer true**: `delete <id>`
- **can't tell** from the code: leave it, and say so in the table

Record the verdicts for step 5. Run none of them yet. Step 3 researches only what
the existing entries don't cover.

## 3. Research in parallel

Read the repo's committed docs and its tree, then split it into 2–5 areas: packages
in a monorepo, top-level directories or layers otherwise. Leave out areas with
nothing costly to work out again (generated code, vendored code, fixtures).

Fork one research agent per area, all in one message: the Agent tool with
`subagent_type: "fork"`. A fork inherits this conversation, this file included, so
its prompt only has to name its area, the entries that already cover it, and the
shape to return. Each fork:

- reads the code in its area, runs what is cheap and safe to run (tests, `--help`),
  and looks for what a new session would get wrong or take long to find
- returns candidates only, and never runs an `ontos` write verb
- returns each candidate in this shape:

```
category:  <one of the eight>
title:     <worded the way Claude would look it up>
trigger:   <one line: read this when…>
tags:      <topics that cut across categories, or none>
sources:   <repo paths, no line numbers>
basis:     read | ran | reasoned     # what the claim rests on
body:
<markdown, citing the file and function behind each claim>
```

Porting skips this step. Research only the areas the docs leave out, if the user
wants them covered.

## 4. Verify and sort

For each candidate, from a fork or from a ported doc section:

1. **Check it yourself.** Open the files and functions it cites. Certainty follows
   what you did, not what the candidate claims: `confirmed` only if you read it in
   the code or ran it; `inferred` if it follows from the code but you didn't see it
   directly; `suspected` for a possible defect, filed as a `gap` whose body says
   "Suspected:". A claim the code contradicts is dropped, or rewritten to what the
   code does.
2. **Apply the skip list** and the precision bar above.
3. **Merge duplicates** across forks. Split a candidate that answers more than one
   question.
4. **Search before adding.** For each title, run `ontos search` with one or two
   distinctive terms from it, then again with a synonym for the main one: every
   term has to match, so more terms find less. If an entry already answers the same
   question, the candidate becomes an update of that entry.
5. **Link.** Note which candidates point to each other; `related` is set in step 6,
   once they have ids.

When porting, sort each doc section by the "how to classify" rules above. A section
that holds a flow and a pitfall becomes a `mechanism` and a `gotcha`. A routing-table
line ("doing X → read Y") becomes the `trigger` of the entry made from Y. A doc's
list of call sites or open tasks is not carried over.

## 5. Ask once

Show everything in one reply, as text:

- **Subjects** to add or change: name, url, path, uses.
- **Re-verified**: id, title, verdict.
- **Entries**: one row each, with new or `update <id>`, category, title, trigger,
  subject, certainty and sources. Put the bodies under the table, each headed by
  its title, so the user can read what will be stored.
- **Dropped**, one line each with the reason, so the user can bring one back.

Ask, and **stop there**. End the turn and run nothing until the user answers. Fold in
what they change; if the change is large, show the table again.

## 6. Write

In this order, so every reference exists before it is used:

1. Subjects: `ontos subject add` in the directory (or `--name <n> --url ''` for a
   subject with no repo), then `ontos subject update <name> --uses <s>…`.
2. The step 2 verdicts.
3. Entries, one `add` each, the body on stdin:

   ```
   ontos add --title '<t>' --category <c> --subject <s> --trigger '<t>' \
     --certainty <c> --checked <sha> --source <path> --tag <t> --body-file - <<'EOF'
   <body>
   EOF
   ```

   The quotes are single so backticks in a title or trigger stay literal; write an
   apostrophe inside them as `'\''`. `add` prints the new id; keep each one next to its title. For an update, run
   `ontos update <id>` with the flags that changed plus `--checked <sha>`.
4. Links: `ontos update <id> --related <id> --related <id>` for each entry that
   points to others. `--related` replaces the whole list, so name every id each time.

If a write fails, its error names the fix. Run that, not a workaround, and carry on
with the rest.

Finish with `ontos search --subject <subject>` and `ontos context [path]`. Report the
ids written and what the next session will load at start.
