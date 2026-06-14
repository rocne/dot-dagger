#!/bin/sh
set -e

export HOME=/home/e2e
export XDG_BIN_HOME=/home/e2e/bin
export XDG_DATA_HOME=/tmp/xdgdata
mkdir -p /home/e2e/bin /tmp/xdgdata

dotd apply \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux

test -L /home/e2e/.zshrc              || { printf 'FAIL: .zshrc symlink missing\n';      exit 1; }
TARGET=$(readlink /home/e2e/.zshrc)
[ "$TARGET" = "/fixture/conf/dot-zshrc" ] \
  || { printf 'FAIL: .zshrc symlink target wrong: %s\n' "$TARGET"; exit 1; }
grep -q "base.sh"    /tmp/xdgdata/dot-dagger/init.sh     || { printf 'FAIL: base.sh not in init.sh\n';      exit 1; }
grep -q "path.sh"    /tmp/xdgdata/dot-dagger/init.sh     || { printf 'FAIL: path.sh not in init.sh\n';      exit 1; }
grep -q "aliases.sh" /tmp/xdgdata/dot-dagger/init.sh     || { printf 'FAIL: aliases.sh not in init.sh\n';   exit 1; }
grep -q "linux.sh"   /tmp/xdgdata/dot-dagger/init.sh     || { printf 'FAIL: linux.sh not in init.sh\n';     exit 1; }
! grep -q "macos.sh"    /tmp/xdgdata/dot-dagger/init.sh  || { printf 'FAIL: macos.sh should not be in init.sh\n';    exit 1; }
! grep -q "work.sh"     /tmp/xdgdata/dot-dagger/init.sh  || { printf 'FAIL: work.sh should not be in init.sh\n';     exit 1; }
! grep -q "disabled.sh" /tmp/xdgdata/dot-dagger/init.sh  || { printf 'FAIL: disabled.sh should not be in init.sh\n'; exit 1; }

printf 'PASS: apply test\n'
