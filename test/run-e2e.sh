#!/bin/sh
# run-e2e.sh — canonical pre-merge e2e gate. Builds dotd from source HEAD.
#
# This script compiles dotd for linux/amd64 from the current working tree
# and runs the full exerciser suite inside Docker. It is the default e2e
# gate and should be run before merging to main.
#
# For the release-install smoke test (post-publish), use run-e2e-release.sh.
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

trap 'rm -f "${SCRIPT_DIR}/e2e/dotd"' EXIT

printf 'e2e: building dotd from source (linux/amd64)\n'
GOOS=linux GOARCH=amd64 go build -o "${SCRIPT_DIR}/e2e/dotd" "${REPO_ROOT}/cmd/dotd"

printf 'e2e: building docker image\n'
docker build -t dotd-e2e -f "${SCRIPT_DIR}/e2e/Dockerfile.local" "${SCRIPT_DIR}/e2e"

run_test() {
  EXERCISER="$1"
  printf '\n=== %s ===\n' "${EXERCISER}"
  docker run --rm \
    -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
    dotd-e2e \
    sh -c ". /procure/local.sh && sh /tests/${EXERCISER}"
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
run_test config-cmds.sh
run_test env-cmds.sh
run_test adopt.sh
run_test bundle.sh
run_test dag-check.sh
run_test package-check.sh
run_test annotate.sh

printf '\nAll e2e tests passed.\n'
