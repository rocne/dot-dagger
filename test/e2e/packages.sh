#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

# @require(fake-installed): binary=sh is always on PATH — requirement satisfied
grep -q "needs-fake.sh" /tmp/init.sh \
  || { printf 'FAIL: needs-fake.sh should be in init.sh (@require satisfied)\n'; exit 1; }

# @request(not-installable): soft dependency — apply must succeed and file must be in init.sh
grep -q "optional-tool.sh" /tmp/init.sh \
  || { printf 'FAIL: optional-tool.sh should be in init.sh (@request is soft)\n'; exit 1; }

printf 'PASS: packages test\n'
