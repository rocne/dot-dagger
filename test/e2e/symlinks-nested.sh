#!/bin/sh
set -e

export HOME=/home/e2e
export XDG_BIN_HOME=/home/e2e/bin
export XDG_DATA_HOME=/tmp/xdgdata
mkdir -p /home/e2e/bin /tmp/xdgdata

dotd apply \
  --files /fixture \
  --dotd-env /fixture/env.yaml \
  --env os=linux \
  --env context=personal

test -L /home/e2e/.config/nvim/init.lua \
  || { printf 'FAIL: ~/.config/nvim/init.lua symlink missing\n'; exit 1; }
TARGET=$(readlink /home/e2e/.config/nvim/init.lua)
[ "$TARGET" = "/fixture/conf/dot-config/nvim/init.lua" ] \
  || { printf 'FAIL: ~/.config/nvim/init.lua target wrong: %s\n' "$TARGET"; exit 1; }

test -L /home/e2e/.gitconfig \
  || { printf 'FAIL: ~/.gitconfig symlink missing (context=personal)\n'; exit 1; }
TARGET=$(readlink /home/e2e/.gitconfig)
[ "$TARGET" = "/fixture/conf/dot-gitconfig" ] \
  || { printf 'FAIL: ~/.gitconfig target wrong: %s\n' "$TARGET"; exit 1; }

test -L /home/e2e/.zshrc \
  || { printf 'FAIL: ~/.zshrc symlink missing\n'; exit 1; }
TARGET=$(readlink /home/e2e/.zshrc)
[ "$TARGET" = "/fixture/conf/dot-zshrc" ] \
  || { printf 'FAIL: ~/.zshrc target wrong: %s\n' "$TARGET"; exit 1; }

printf 'PASS: symlinks-nested test\n'
