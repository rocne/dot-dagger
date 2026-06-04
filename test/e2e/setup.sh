#!/bin/sh
set -e

export XDG_CONFIG_HOME=/tmp/xdg
export DOTFILES=/fixture

printf '\n\n\n\n' | dotd setup

test -f /tmp/xdg/dot-dagger/config.yaml \
  || { printf 'FAIL: config.yaml not written\n'; exit 1; }
grep -q "dotfiles: /fixture" /tmp/xdg/dot-dagger/config.yaml \
  || { printf 'FAIL: dotfiles not set to /fixture in config.yaml\n'; exit 1; }

printf 'PASS: setup test\n'
