#!/bin/sh
set -e

OUT=$(dotd bundle shellrc/aliases.sh \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --env os=linux \
  --env context=personal)

printf '%s' "$OUT" | grep -q "DOT_BASE_LOADED" \
  || { printf 'FAIL: bundle should contain DOT_BASE_LOADED from base.sh\n'; exit 1; }

printf 'PASS: bundle test\n'
