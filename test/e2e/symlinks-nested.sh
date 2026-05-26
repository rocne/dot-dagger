#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated
/tmp/dotd apply \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --link-root /home/e2e \
  --bin-dir /home/e2e/bin \
  --init-file /tmp/init.sh \
  --generated-dir /tmp/generated \
  --env os=linux \
  --env context=personal

# conf/dot-config/nvim/init.lua → ~/.config/nvim/init.lua (explicit action in .dagger)
test -L /home/e2e/.config/nvim/init.lua \
  || { printf 'FAIL: ~/.config/nvim/init.lua symlink missing\n'; exit 1; }

# conf/dot-gitconfig → ~/.gitconfig (context=personal predicate matches)
test -L /home/e2e/.gitconfig \
  || { printf 'FAIL: ~/.gitconfig symlink missing (context=personal)\n'; exit 1; }

# conf/dot-zshrc → ~/.zshrc (basic, no predicate)
test -L /home/e2e/.zshrc \
  || { printf 'FAIL: ~/.zshrc symlink missing\n'; exit 1; }

printf 'PASS: symlinks-nested test\n'
