#!/bin/sh
set -e

OUT=$(dotd dag order \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --env os=linux \
  --env context=personal)

printf '%s' "$OUT" | grep -q "shellrc.base"    || { printf 'FAIL: shellrc.base not in dag order output\n';    exit 1; }
printf '%s' "$OUT" | grep -q "shellrc.path"    || { printf 'FAIL: shellrc.path not in dag order output\n';    exit 1; }
printf '%s' "$OUT" | grep -q "shellrc.aliases" || { printf 'FAIL: shellrc.aliases not in dag order output\n'; exit 1; }

BASE_LINE=$(printf '%s' "$OUT" | grep -n "shellrc.base" | head -1 | cut -d: -f1)
PATH_LINE=$(printf '%s' "$OUT" | grep -n "shellrc.path" | head -1 | cut -d: -f1)
[ "$BASE_LINE" -lt "$PATH_LINE" ] \
  || { printf 'FAIL: shellrc.base (%s) should appear before shellrc.path (%s)\n' "$BASE_LINE" "$PATH_LINE"; exit 1; }

# Pre-rename alias must keep working for existing scripts.
ALIAS_OUT=$(dotd dag check \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --env os=linux \
  --env context=personal)
[ "$ALIAS_OUT" = "$OUT" ] || { printf 'FAIL: dag check alias output differs from dag order\n'; exit 1; }

printf 'PASS: dag-order test\n'
