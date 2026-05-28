#!/bin/sh
set -e

TAG="${DOTD_VERSION:?DOTD_VERSION must be set}"

sh /repo/install.sh --version "${TAG}"

test -x "${HOME}/.local/bin/dotd"   || { printf 'FAIL: dotd not installed at ~/.local/bin/dotd\n'; exit 1; }
"${HOME}/.local/bin/dotd" --version || { printf 'FAIL: dotd --version exited non-zero\n'; exit 1; }

export PATH="$HOME/.local/bin:$PATH"
