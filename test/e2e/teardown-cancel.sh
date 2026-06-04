#!/bin/sh
set -e

export XDG_CONFIG_HOME=/tmp/xdg
mkdir -p /tmp/xdg/dot-dagger
printf 'dotfiles: /fixture\n' > /tmp/xdg/dot-dagger/config.yaml

printf 'n\n' | dotd teardown \
  --files /fixture \
  --env-file /fixture/env.yaml

test -f /tmp/xdg/dot-dagger/config.yaml \
  || { printf 'FAIL: config.yaml should be preserved on cancel\n'; exit 1; }

printf 'PASS: teardown-cancel test\n'
