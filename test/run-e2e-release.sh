#!/bin/sh
# run-e2e-release.sh — post-release smoke test using a published GitHub release.
#
# This is the release-install variant. The canonical pre-merge gate is
# run-e2e.sh, which builds from source. Use this script to verify a
# release tag after publishing.
#
# Usage:
#   DOTD_VERSION=v0.3.0 sh test/run-e2e-release.sh
#   sh test/run-e2e-release.sh   # fetches latest published release
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [ -z "${DOTD_VERSION}" ]; then
  DOTD_VERSION="$(curl -fsSL "https://api.github.com/repos/rocne/dot-dagger/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
fi

: "${DOTD_VERSION:?Could not determine release version}"

printf 'e2e-release: testing release %s\n' "${DOTD_VERSION}"

docker build -t dotd-e2e "${SCRIPT_DIR}/e2e"

run_test() {
  EXERCISER="$1"
  printf '\n=== %s ===\n' "${EXERCISER}"
  docker run --rm \
    -e DOTD_VERSION="${DOTD_VERSION}" \
    -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
    -v "${REPO_ROOT}/install.sh:/repo/install.sh:ro" \
    dotd-e2e \
    sh -c ". /procure/release.sh && sh /tests/${EXERCISER}"
}

run_test apply.sh
run_test context.sh
run_test dag-order.sh
run_test dry-run.sh
run_test idempotent.sh
run_test check.sh
run_test list.sh
run_test bin.sh
run_test symlinks-nested.sh
run_test disable.sh
run_test packages.sh
run_test conflict.sh
run_test unapply.sh
run_test unapply-cancel.sh
run_test compose.sh
run_test macos-apply.sh
run_test setup.sh
run_test teardown-confirm.sh
run_test teardown-cancel.sh
run_test init.sh

printf '\nAll e2e-release tests passed.\n'
