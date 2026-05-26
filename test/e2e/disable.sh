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

! grep -q "disabled.sh" /tmp/init.sh \
  || { printf 'FAIL: disabled.sh should not appear in init.sh\n'; exit 1; }

grep -q "base.sh" /tmp/init.sh \
  || { printf 'FAIL: base.sh missing from init.sh (control check)\n'; exit 1; }

printf 'PASS: disable test\n'
