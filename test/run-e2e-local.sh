#!/bin/sh
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
REPO_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

trap 'rm -f "${SCRIPT_DIR}/e2e/dotd"' EXIT

printf 'e2e-local: building dotd for linux/amd64\n'
GOOS=linux GOARCH=amd64 go build -o "${SCRIPT_DIR}/e2e/dotd" "${REPO_ROOT}/cmd/dotd"

printf 'e2e-local: building docker image\n'
docker build -t dotd-e2e-local -f "${SCRIPT_DIR}/e2e/Dockerfile.local" "${SCRIPT_DIR}/e2e"

run_test() {
  EXERCISER="$1"
  printf '\n=== %s ===\n' "${EXERCISER}"
  docker run --rm \
    -v "${SCRIPT_DIR}/e2e/fixture:/fixture:ro" \
    dotd-e2e-local \
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

printf '\nAll e2e-local tests passed.\n'
