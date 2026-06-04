#!/bin/sh
set -e

export XDG_CONFIG_HOME=/tmp/xdg
mkdir -p /tmp/xdg/dot-dagger
printf 'dotfiles: /fixture\n' > /tmp/xdg/dot-dagger/config.yaml

printf 'y\n' | dotd teardown \
  --files /fixture \
  --env-file /fixture/env.yaml

test ! -f /tmp/xdg/dot-dagger/config.yaml \
  || { printf 'FAIL: config.yaml should be removed after teardown\n'; exit 1; }

printf 'PASS: teardown-confirm test\n'
