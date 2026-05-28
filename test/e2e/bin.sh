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

test -L /home/e2e/bin/hello \
  || { printf 'FAIL: bin/hello symlink missing from bin-dir\n'; exit 1; }

TARGET=$(readlink /home/e2e/bin/hello)
[ "$TARGET" = "/fixture/bin/hello" ] \
  || { printf 'FAIL: bin/hello symlink target wrong: %s\n' "$TARGET"; exit 1; }

test -x /home/e2e/bin/hello \
  || { printf 'FAIL: bin/hello is not executable via symlink\n'; exit 1; }

printf 'PASS: bin test\n'
