#!/bin/sh
set -e

export XDG_CONFIG_HOME=/tmp/xdg
mkdir -p /tmp/xdg/dot-dagger
printf 'dotfiles: /fixture\n' > /tmp/xdg/dot-dagger/config.yaml
cp /fixture/env.yaml /tmp/xdg/dot-dagger/env.yaml

# No --env-file: teardown removes the resolved (XDG default) paths.
printf 'y\n' | dotd teardown \
  --files /fixture

test ! -f /tmp/xdg/dot-dagger/config.yaml \
  || { printf 'FAIL: config.yaml should be removed after teardown\n'; exit 1; }
test ! -f /tmp/xdg/dot-dagger/env.yaml \
  || { printf 'FAIL: env.yaml should be removed after teardown\n'; exit 1; }

printf 'PASS: teardown-confirm test\n'
