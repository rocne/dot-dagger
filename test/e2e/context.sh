#!/bin/sh
set -e

# --- work context ---
mkdir -p /home/e2e-work/bin /tmp/xdg-work
export HOME=/home/e2e-work
export XDG_BIN_HOME=/home/e2e-work/bin
export XDG_DATA_HOME=/tmp/xdg-work

dotd apply \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=work

grep -q "work.sh" /tmp/xdg-work/dot-dagger/init.sh \
  || { printf 'FAIL: work.sh should be in init.sh for context=work\n'; exit 1; }
! test -L /home/e2e-work/.gitconfig \
  || { printf 'FAIL: .gitconfig should not be linked in work context\n'; exit 1; }

# --- personal context ---
mkdir -p /home/e2e-personal/bin /tmp/xdg-personal
export HOME=/home/e2e-personal
export XDG_BIN_HOME=/home/e2e-personal/bin
export XDG_DATA_HOME=/tmp/xdg-personal

dotd apply \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=personal

! grep -q "work.sh" /tmp/xdg-personal/dot-dagger/init.sh \
  || { printf 'FAIL: work.sh should not be in init.sh for context=personal\n'; exit 1; }
test -L /home/e2e-personal/.gitconfig \
  || { printf 'FAIL: .gitconfig should be linked in personal context\n'; exit 1; }
TARGET=$(readlink /home/e2e-personal/.gitconfig)
[ "$TARGET" = "/fixture/conf/dot-gitconfig" ] \
  || { printf 'FAIL: .gitconfig target wrong: %s\n' "$TARGET"; exit 1; }

printf 'PASS: context test\n'
