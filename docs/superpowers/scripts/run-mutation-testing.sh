#!/usr/bin/env bash
# Mutation testing runner for dot-dagger (Track E of the 2026-06 audit campaign).
#
# Run this on a HIGH-MEMORY host (the maintainer's Mac), NOT on the low-memory
# remote dev box (1.6 GB RAM, no swap). gremlins forks one `go test` compile per
# worker (~200-400 MB each); enough parallel compiles OOM-kill the machine. See
# docs/superpowers/plans/2026-06-15-mutation-testing-deferred.md for the full why.
#
# Usage (from the repo root):
#   bash docs/superpowers/scripts/run-mutation-testing.sh                  # all priority pkgs
#   WORKERS=8 bash docs/superpowers/scripts/run-mutation-testing.sh        # tune parallelism
#   bash docs/superpowers/scripts/run-mutation-testing.sh internal/predicate   # one pkg
#
# Output: ./phaseE-mutation-results-<date>.md (a LOCAL artifact — do not commit).
# Raw per-package logs land in a temp dir, path printed at the end.
set -euo pipefail

# --- tunables -----------------------------------------------------------------
# WORKERS: number of mutants tested in parallel. gremlins defaults to NumCPU;
# each worker holds a go compile. Safe rule of thumb: WORKERS <= floor(free_GB / 0.5).
# 16 GB Mac -> 4-8 comfortable. More = faster + more memory. Mutation testing is
# slow but not urgent, so favour headroom over speed.
WORKERS="${WORKERS:-4}"
# TIMEOUT_COEFFICIENT: per-mutant timeout = baseline_test_time * coefficient.
# The remote run timed out 122/125 mutants because the derived timeout was too
# tight for the fast suite. 10 gives headroom so real KILLED/LIVED verdicts replace
# TIMED OUT. If a package still shows many TIMED OUT, raise this and rerun it.
TIMEOUT_COEFFICIENT="${TIMEOUT_COEFFICIENT:-10}"

DATE="$(date +%Y-%m-%d)"
RESULTS="./phaseE-mutation-results-${DATE}.md"
LOGDIR="$(mktemp -d -t dotd-mutation-XXXX)"

# Priority packages, highest-stakes logic first. Pass args to override.
PKGS=(
  internal/pipeline      # DAG engine: walk, order, act, actions, filter
  internal/predicate     # condition eval, parser, lexer
  internal/annotation
  internal/node
  internal/ecosystem
  internal/packages
  internal/env
  internal/adopter
)
[ "$#" -gt 0 ] && PKGS=("$@")

# --- preflight ----------------------------------------------------------------
command -v go >/dev/null || { echo "go not on PATH (try: brew install go)"; exit 1; }
if ! command -v gremlins >/dev/null; then
  echo "installing gremlins..."
  go install github.com/go-gremlins/gremlins/cmd/gremlins@latest
  export PATH="$(go env GOPATH)/bin:$PATH"
fi
echo "go:       $(go version)"
echo "gremlins: $(gremlins --version 2>&1 | head -1)"
echo "workers:  ${WORKERS}   timeout-coefficient: ${TIMEOUT_COEFFICIENT}"

echo "=== baseline: go test ./... (must be GREEN) ==="
if ! go test ./... >"${LOGDIR}/baseline.log" 2>&1; then
  echo "BASELINE RED — mutation results would be meaningless. Aborting."
  tail -30 "${LOGDIR}/baseline.log"
  exit 1
fi
echo "baseline GREEN"

# --- run ----------------------------------------------------------------------
{
  echo "# Phase E — mutation testing results (${DATE})"
  echo
  echo "Host: $(uname -srm) · gremlins $(gremlins --version 2>&1 | head -1)"
  echo "Settings: --workers ${WORKERS} --timeout-coefficient ${TIMEOUT_COEFFICIENT}"
  echo "Baseline: GREEN"
  echo
  echo "LIVED = a mutation the suite failed to catch (untested behavior)."
  echo "NOT COVERED = code no test exercises at all."
  echo "TIMED OUT should be near 0; if high, raise TIMEOUT_COEFFICIENT and rerun that pkg."
  echo
} >"${RESULTS}"

for pkg in "${PKGS[@]}"; do
  echo "=== ${pkg} ==="
  log="${LOGDIR}/$(echo "$pkg" | tr / _).log"
  gremlins unleash \
    --workers "${WORKERS}" \
    --timeout-coefficient "${TIMEOUT_COEFFICIENT}" \
    "./${pkg}/" >"${log}" 2>&1 || true
  {
    echo "## ${pkg}"
    echo '```'
    grep -E "LIVED|NOT COVERED" "${log}" || echo "(no LIVED / NOT COVERED mutants)"
    echo "--- summary ---"
    tail -6 "${log}"
    echo '```'
    echo
  } >>"${RESULTS}"
  echo "  -> appended to ${RESULTS}"
done

echo
echo "DONE. Triage LIVED + NOT COVERED in ${RESULTS}: each is a behavior the suite"
echo "does not pin down -> write a killing test. Raw logs: ${LOGDIR}"
