#!/bin/sh
set -e

export XDG_CONFIG_HOME=/tmp/xdg
mkdir -p /tmp/xdg/dot-dagger
printf 'dotfiles: /fixture\n' > /tmp/xdg/dot-dagger/config.yaml
cp /fixture/env.yaml /tmp/xdg/dot-dagger/env.yaml

printf 'n\n' | dotd teardown \
  --files /fixture

test -f /tmp/xdg/dot-dagger/config.yaml \
  || { printf 'FAIL: config.yaml should be preserved on cancel\n'; exit 1; }
test -f /tmp/xdg/dot-dagger/env.yaml \
  || { printf 'FAIL: env.yaml should be preserved on cancel\n'; exit 1; }

printf 'PASS: teardown-cancel test\n'
