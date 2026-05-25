#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"

# install via install.sh
sh /repo/install.sh --version "${TAG}"

# assert binary present before proceeding
test -x "${HOME}/.local/bin/dotd"   || { printf 'FAIL: dotd not installed\n'; exit 1; }

# create isolated home dirs
mkdir -p /home/e2e/bin /tmp/generated

# apply against fixture using the installed binary
"${HOME}/.local/bin/dotd" apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux

# assertions
test -L /home/e2e/.zshrc          || { printf 'FAIL: .zshrc symlink missing\n'; exit 1; }
grep -q "base.sh"  /tmp/init.sh    || { printf 'FAIL: base.sh not in init.sh\n'; exit 1; }
grep -q "linux.sh" /tmp/init.sh    || { printf 'FAIL: linux.sh not in init.sh\n'; exit 1; }
! grep -q "macos.sh" /tmp/init.sh  || { printf 'FAIL: macos.sh should not be in init.sh\n'; exit 1; }

printf 'PASS: combined test\n'
