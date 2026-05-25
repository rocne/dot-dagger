#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"

# run the project installer for this specific release
sh /repo/install.sh --version "${TAG}"

# assertions
test -x "${HOME}/.local/bin/dotd"   || { printf 'FAIL: dotd not installed at ~/.local/bin/dotd\n'; exit 1; }
"${HOME}/.local/bin/dotd" --version || { printf 'FAIL: dotd --version exited non-zero\n'; exit 1; }

printf 'PASS: installer test\n'
