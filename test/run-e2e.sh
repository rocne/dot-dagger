#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# resolve tag
if [ -z "${DOTD_VERSION}" ]; then
  DOTD_VERSION="$(curl -fsSL "https://api.github.com/repos/rocne/dot-dagger/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
fi

printf 'e2e: testing release %s\n' "${DOTD_VERSION}"

# build image
docker build -t dotd-e2e "${SCRIPT_DIR}/e2e"

# binary test
printf '\n=== binary test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/binary.sh

# installer test
printf '\n=== installer test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${REPO_ROOT}/install.sh:/repo/install.sh:ro" \
  dotd-e2e sh /tests/install.sh

# combined test
printf '\n=== combined test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${REPO_ROOT}/install.sh:/repo/install.sh:ro" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/combined.sh

printf '\nAll e2e tests passed.\n'
