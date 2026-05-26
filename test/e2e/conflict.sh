#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"
ASSET="dotd_${TAG}_linux_amd64.tar.gz"
BASE_URL="https://github.com/rocne/dot-dagger/releases/download/${TAG}"

curl -fsSL -o "/tmp/${ASSET}" "${BASE_URL}/${ASSET}"
tar -xzf "/tmp/${ASSET}" -C /tmp
chmod +x /tmp/dotd

mkdir -p /home/e2e/bin /tmp/generated

# Place a plain file where dotd would create the .zshrc symlink
printf 'existing content\n' > /home/e2e/.zshrc

APPLY_ARGS="--files /fixture --env-file /fixture/env.yaml --link-root /home/e2e --bin-dir /home/e2e/bin --init-file /tmp/init.sh --generated-dir /tmp/generated --env os=linux --env context=personal"

# Without --force: must fail when a plain file exists at the symlink destination
/tmp/dotd apply $APPLY_ARGS 2>/dev/null \
  && { printf 'FAIL: apply without --force should have failed on plain-file conflict\n'; exit 1; } \
  || true
# Plain file must still be there (not replaced)
test -L /home/e2e/.zshrc \
  && { printf 'FAIL: apply without --force must not replace plain file with symlink\n'; exit 1; } \
  || true

# Remove any partial state and restore the plain file for the --force test
rm -f /home/e2e/.zshrc
printf 'existing content\n' > /home/e2e/.zshrc

# With --force: must succeed and .zshrc must become a symlink
/tmp/dotd apply $APPLY_ARGS --force \
  || { printf 'FAIL: apply --force failed\n'; exit 1; }

test -L /home/e2e/.zshrc \
  || { printf 'FAIL: .zshrc is not a symlink after apply --force\n'; exit 1; }
TARGET=$(readlink /home/e2e/.zshrc)
[ "$TARGET" = "/fixture/conf/dot-zshrc" ] \
  || { printf 'FAIL: .zshrc symlink target wrong: %s\n' "$TARGET"; exit 1; }

printf 'PASS: conflict test\n'
