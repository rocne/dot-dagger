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

# context test
printf '\n=== context test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/context.sh

# dag-order test
printf '\n=== dag-order test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/dag-order.sh

# dry-run test
printf '\n=== dry-run test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/dry-run.sh

# idempotent test
printf '\n=== idempotent test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/idempotent.sh

# check test
printf '\n=== check test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/check.sh

# list test
printf '\n=== list test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/list.sh

# bin test
printf '\n=== bin test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/bin.sh

# symlinks-nested test
printf '\n=== symlinks-nested test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/symlinks-nested.sh

# disable test
printf '\n=== disable test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/disable.sh

# packages test
printf '\n=== packages test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/packages.sh

# conflict test
printf '\n=== conflict test ===\n'
docker run --rm \
  -e DOTD_VERSION="${DOTD_VERSION}" \
  -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
  dotd-e2e sh /tests/conflict.sh

printf '\nAll e2e tests passed.\n'
