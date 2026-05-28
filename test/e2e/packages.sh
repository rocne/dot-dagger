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

grep -q "needs-fake.sh" /tmp/init.sh \
  || { printf 'FAIL: needs-fake.sh should be in init.sh (@require satisfied)\n'; exit 1; }

grep -q "optional-tool.sh" /tmp/init.sh \
  || { printf 'FAIL: optional-tool.sh should be in init.sh (@request is soft)\n'; exit 1; }

printf 'PASS: packages test\n'
