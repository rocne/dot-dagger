#!/bin/sh
set -e

printf 'dotfiles: /fixture\n' > /tmp/dotd.yaml

dotd config show \
  --dotd-config /tmp/dotd.yaml \
  | grep -q "dotfiles" || { printf 'FAIL: config show missing dotfiles\n'; exit 1; }

VAL=$(dotd config get dotfiles \
  --dotd-config /tmp/dotd.yaml)
[ "$VAL" = "/fixture" ] \
  || { printf 'FAIL: config get dotfiles = %s, want /fixture\n' "$VAL"; exit 1; }

dotd config set dotfiles /fixture2 \
  --dotd-config /tmp/dotd.yaml

VAL=$(dotd config get dotfiles \
  --dotd-config /tmp/dotd.yaml)
[ "$VAL" = "/fixture2" ] \
  || { printf 'FAIL: config get after set = %s, want /fixture2\n' "$VAL"; exit 1; }

printf 'PASS: config-cmds test\n'
