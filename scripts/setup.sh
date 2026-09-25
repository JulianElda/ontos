#!/usr/bin/env bash
#
# Bootstrap ontos: the config, the store, the binary, the skill links, and
# what to add to Claude Code's settings. Idempotent — safe to re-run after
# editing the Go sources or adding a skill.
#
# ONTOS_CONFIG points it (and ontos) at another config, which is how a scratch
# store is set up for testing.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN_DIR="${HOME}/.local/bin"
SKILL_DIR="${HOME}/.claude/skills"
CONFIG="${ONTOS_CONFIG:-${HOME}/.config/ontos/config.toml}"

say() { printf '  %s\n' "$*"; }
warn() { printf '  WARNING: %s\n' "$*" >&2; }

# 1. Config first: the store goes where it points. Seeded only when missing,
#    never overwritten.
echo "config"
if [ -f "${CONFIG}" ]; then
  say "${CONFIG} (kept)"
else
  mkdir -p "$(dirname "${CONFIG}")"
  cat >"${CONFIG}" <<'EOF'
# ontos. Local to this machine and not synced: each machine has exactly one
# store, and work and personal stores are never mixed.

# The store: one JSON file per entry and per subject. Point it at a
# cloud-synced folder to share it between machines.
store = "~/ontos"

# What `ontos context` prints at session start: `full` categories in full,
# `index` categories as one line per entry. A category in neither is only
# found by `ontos search`.
[context]
full  = ["map"]
index = ["mechanism", "invariant", "gotcha", "procedure", "diagnosis", "gap", "reference"]
EOF
  say "${CONFIG} (created)"
  say "store defaults to ~/ontos — point it at your sync folder and re-run"
fi

# Read the store back rather than assuming. The seeded config has one quoted
# scalar per line, so sed is enough and no TOML parser is needed here.
STORE="$(sed -n 's/^[[:space:]]*store[[:space:]]*=[[:space:]]*"\([^"]*\)".*/\1/p' "${CONFIG}" | head -n1)"
if [ -z "${STORE}" ]; then
  echo "no \`store\` in ${CONFIG}; add it, naming the folder that holds the entries" >&2
  exit 1
fi
STORE="${STORE/#\~/${HOME}}"

# 2. The store, where the config points.
echo "store"
say "${STORE}"
for sub in entries subjects; do
  mkdir -p "${STORE}/${sub}"
  say "${sub}/"
done

# 3. The binary. Built here rather than packaged in the flake: a nixos-rebuild
#    is password-gated, and this needs to be rebuildable in a session.
echo "binary"
mkdir -p "${BIN_DIR}"
(cd "${ROOT}" && go build -o "${BIN_DIR}/ontos" ./cmd/ontos)
say "${BIN_DIR}/ontos"
case ":${PATH}:" in
*":${BIN_DIR}:"*) ;;
*) warn "${BIN_DIR} is not on PATH; the SessionStart hook will not find ontos" ;;
esac

# 4. Skills, symlinked so there is one copy and it is the versioned one. Each
#    link points at the directory, so a new file inside needs no relink.
echo "skills"
mkdir -p "${SKILL_DIR}"
linked=0
for skill in "${ROOT}"/skills/*/; do
  [ -d "${skill}" ] || continue
  name="$(basename "${skill}")"
  ln -sfn "${skill%/}" "${SKILL_DIR}/${name}"
  say "${name}"
  linked=1
done
[ "${linked}" = 1 ] || say "(none yet)"

# 5. Claude Code integration. Printed, never applied: both files are the
#    user's, and settings.json is managed by chezmoi.
cat <<'EOF'

Add to ~/.claude/settings.json, then `chezmoi add ~/.claude/settings.json`:

  "permissions": { "allow": ["Bash(ontos:*)"] },
  "hooks": {
    "SessionStart": [
      { "hooks": [ { "type": "command", "command": "ontos context" } ] }
    ]
  }

Add to ~/.claude/CLAUDE.md:

  Repo knowledge (how code is laid out, works, and breaks) goes in ontos; `ontos context` loads it at session start.
  Personal preferences and feedback go in auto memory, not ontos.
  Plans and open work don't go in ontos.
EOF
