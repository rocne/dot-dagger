#!/bin/sh
set -e

# fixture is read-only; copy env.yaml so env set can write to it
cp /fixture/env.yaml /tmp/env.yaml

OUT=$(dotd env show \
  --env-file /tmp/env.yaml \
  --files /fixture)
printf '%s' "$OUT" | grep -q "context" \
  || { printf 'FAIL: env show missing context\nOutput:\n%s\n' "$OUT"; exit 1; }

VAL=$(dotd env get context \
  --env-file /tmp/env.yaml \
  --files /fixture)
[ "$VAL" = "personal" ] \
  || { printf 'FAIL: env get context = %s, want personal\n' "$VAL"; exit 1; }

dotd env set context staging \
  --env-file /tmp/env.yaml \
  --files /fixture

VAL=$(dotd env get context \
  --env-file /tmp/env.yaml \
  --files /fixture)
[ "$VAL" = "staging" ] \
  || { printf 'FAIL: env get after set = %s, want staging\n' "$VAL"; exit 1; }

# env diff compares env.yaml values against DOTD_* shell vars.
# DOTD_CONTEXT is unset in Docker, so "context" appears as a diff entry.
OUT=$(dotd env diff \
  --env-file /tmp/env.yaml \
  --files /fixture)
printf '%s' "$OUT" | grep -q "context" \
  || { printf 'FAIL: env diff missing context\nOutput:\n%s\n' "$OUT"; exit 1; }

# env show text output is uniform key=val (no inline source column).
# Shell expressions are surfaced through --json instead.
OUT=$(dotd env show \
  --env-file /tmp/env.yaml \
  --files /fixture)
printf '%s' "$OUT" | grep -q '\[\$(' \
  && { printf 'FAIL: env show text must be uniform key=val\nOutput:\n%s\n' "$OUT"; exit 1; }

JSON=$(dotd env show --json \
  --env-file /tmp/env.yaml \
  --files /fixture)
printf '%s' "$JSON" | grep -q '"expression": "\$(hostname)"' \
  || { printf 'FAIL: env show --json missing expression for hostname\nOutput:\n%s\n' "$JSON"; exit 1; }

# env path prints the env file path
OUT=$(dotd env path \
  --env-file /tmp/env.yaml \
  --files /fixture)
[ "$OUT" = "/tmp/env.yaml" ] \
  || { printf 'FAIL: env path = %s, want /tmp/env.yaml\n' "$OUT"; exit 1; }

printf 'PASS: env-cmds test\n'
