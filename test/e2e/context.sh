#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

# --- work context ---
mkdir -p /home/e2e-work/bin /tmp/gen-work
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e-work \
  --bin-dir /home/e2e-work/bin \
  --init-file /tmp/init-work.sh \
  --generated-dir /tmp/gen-work \
  --env os=linux \
  --env context=work

grep -q "work.sh" /tmp/init-work.sh \
  || { printf 'FAIL: work.sh should be in init.sh for context=work\n'; exit 1; }
! test -L /home/e2e-work/.gitconfig \
  || { printf 'FAIL: .gitconfig should not be linked in work context\n'; exit 1; }

# --- personal context ---
mkdir -p /home/e2e-personal/bin /tmp/gen-personal
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e-personal \
  --bin-dir /home/e2e-personal/bin \
  --init-file /tmp/init-personal.sh \
  --generated-dir /tmp/gen-personal \
  --env os=linux \
  --env context=personal

! grep -q "work.sh" /tmp/init-personal.sh \
  || { printf 'FAIL: work.sh should not be in init.sh for context=personal\n'; exit 1; }
test -L /home/e2e-personal/.gitconfig \
  || { printf 'FAIL: .gitconfig should be linked in personal context\n'; exit 1; }
TARGET=$(readlink /home/e2e-personal/.gitconfig)
[ "$TARGET" = "/fixture/conf/dot-gitconfig" ] \
  || { printf 'FAIL: .gitconfig target wrong: %s\n' "$TARGET"; exit 1; }

printf 'PASS: context test\n'
