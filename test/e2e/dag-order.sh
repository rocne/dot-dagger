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

pos_base=$(grep -n "base.sh"    /tmp/init.sh | head -1 | cut -d: -f1)
pos_path=$(grep -n "path.sh"    /tmp/init.sh | head -1 | cut -d: -f1)
pos_aliases=$(grep -n "aliases.sh" /tmp/init.sh | head -1 | cut -d: -f1)
pos_linux=$(grep -n "linux.sh"  /tmp/init.sh | head -1 | cut -d: -f1)

[ -n "$pos_base" ]    || { printf 'FAIL: base.sh not in init.sh\n';    exit 1; }
[ -n "$pos_path" ]    || { printf 'FAIL: path.sh not in init.sh\n';    exit 1; }
[ -n "$pos_aliases" ] || { printf 'FAIL: aliases.sh not in init.sh\n'; exit 1; }
[ -n "$pos_linux" ]   || { printf 'FAIL: linux.sh not in init.sh\n';   exit 1; }

[ "$pos_base" -lt "$pos_path" ]    || { printf 'FAIL: base.sh must come before path.sh\n';    exit 1; }
[ "$pos_path" -lt "$pos_aliases" ] || { printf 'FAIL: path.sh must come before aliases.sh\n'; exit 1; }
[ "$pos_base" -lt "$pos_linux" ]   || { printf 'FAIL: base.sh must come before linux.sh\n';   exit 1; }

printf 'PASS: dag-order test\n'
