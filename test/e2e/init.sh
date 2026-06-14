#!/bin/sh
set -e

export XDG_CONFIG_HOME=/tmp/xdg
mkdir -p /tmp/xdg/dot-dagger
printf 'dotfiles: /tmp/testdotfiles\n' > /tmp/xdg/dot-dagger/config.yaml
mkdir -p /tmp/testdotfiles

# 3 × (y = create dir, \n = accept default name), trailing EOF skips source-line prompt
printf 'y\n\ny\n\ny\n\n' | dotd init --dotd-config /tmp/xdg/dot-dagger/config.yaml

test -f /tmp/testdotfiles/shellrc/.dagger \
  || { printf 'FAIL: shellrc/.dagger not scaffolded\n'; exit 1; }
test -f /tmp/testdotfiles/config/.dagger \
  || { printf 'FAIL: config/.dagger not scaffolded\n'; exit 1; }
test -f /tmp/testdotfiles/bin/.dagger \
  || { printf 'FAIL: bin/.dagger not scaffolded\n'; exit 1; }

printf 'PASS: init test\n'
