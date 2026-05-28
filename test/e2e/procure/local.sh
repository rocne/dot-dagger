#!/bin/sh
set -e

mkdir -p "${HOME}/.local/bin"
cp /staged/dotd "${HOME}/.local/bin/dotd"
chmod +x "${HOME}/.local/bin/dotd"

export PATH="$HOME/.local/bin:$PATH"
