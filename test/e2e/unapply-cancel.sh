#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated

dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

printf 'n\n' | dotd unapply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

test -L /home/e2e/.zshrc || { printf 'FAIL: .zshrc symlink should be preserved on cancel\n'; exit 1; }

printf 'PASS: unapply-cancel test\n'
