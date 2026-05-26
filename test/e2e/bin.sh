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

# bin/hello should be symlinked into --bin-dir
test -L /home/e2e/bin/hello \
  || { printf 'FAIL: bin/hello symlink missing from bin-dir\n'; exit 1; }

# symlink target must point back to the fixture file
TARGET=$(readlink /home/e2e/bin/hello)
echo "$TARGET" | grep -q "bin/hello" \
  || { printf 'FAIL: bin/hello symlink target wrong: %s\n' "$TARGET"; exit 1; }

# following the symlink must yield an executable
test -x /home/e2e/bin/hello \
  || { printf 'FAIL: bin/hello is not executable via symlink\n'; exit 1; }

printf 'PASS: bin test\n'
