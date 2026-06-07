#!/bin/sh
set -e

# Work on a copy so we don't mutate the read-only /fixture mount.
cp -r /fixture /tmp/dotfiles

TARGET=/tmp/dotfiles/shellrc/aliases.sh

# Input is fed with brief sleeps between lines so that huh's bufio.Scanner
# does not over-read across form boundaries. Each sleep gives the running
# form time to consume its current line before the next arrives.
send() {
  for line in "$@"; do
    printf '%s\n' "$line"
    sleep 0.1
  done
}

# ── Test 1: add @when(os=linux) ─────────────────────────────────────────────
# Accessible-mode: select When (1), enter value, Done (8), confirm Yes (y).
send 1 'os=linux' 8 y | dotd annotate \
  --files /tmp/dotfiles \
  --env-file /tmp/dotfiles/env.yaml \
  "$TARGET"

grep -q '@when(os=linux)' "$TARGET" \
  || { printf 'FAIL: @when(os=linux) not written\n'; exit 1; }

grep -q '@after(shellrc.path)' "$TARGET" \
  || { printf 'FAIL: original @after annotation not preserved\n'; exit 1; }

printf 'PASS: annotate add @when\n'

# ── Test 2: set @action(source) ─────────────────────────────────────────────
# Select Action (5), select source (1), Done (8), Yes (y).
send 5 1 8 y | dotd annotate \
  --files /tmp/dotfiles \
  --env-file /tmp/dotfiles/env.yaml \
  "$TARGET"

grep -q '@action(source)' "$TARGET" \
  || { printf 'FAIL: @action(source) not written\n'; exit 1; }

printf 'PASS: annotate set @action\n'

# ── Test 3: cancel leaves file unchanged ────────────────────────────────────
cp "$TARGET" /tmp/aliases_before.sh

# Done immediately (8), No at confirm (n).
send 8 n | dotd annotate \
  --files /tmp/dotfiles \
  --env-file /tmp/dotfiles/env.yaml \
  "$TARGET"

diff -q "$TARGET" /tmp/aliases_before.sh \
  || { printf 'FAIL: file changed after cancel\n'; exit 1; }

printf 'PASS: annotate cancel unchanged\n'
printf 'PASS: annotate e2e\n'
