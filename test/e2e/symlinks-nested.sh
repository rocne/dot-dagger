#!/bin/sh
set -e

mkdir -p /home/e2e/bin /tmp/generated
dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
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
