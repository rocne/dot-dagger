#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

if [ -z "${DOTD_VERSION}" ]; then
  DOTD_VERSION="$(curl -fsSL "https://api.github.com/repos/rocne/dot-dagger/releases/latest" \
    | grep '"tag_name"' \
    | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')"
fi

printf 'e2e: testing release %s\n' "${DOTD_VERSION}"

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

printf '\nAll e2e tests passed.\n'
