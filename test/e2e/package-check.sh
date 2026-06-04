#!/bin/sh
set -e

OUT=$(dotd package check \
  --files /fixture \
  --env-file /fixture/env.yaml \
  --env os=linux \
  --env context=personal)

printf '%s' "$OUT" | grep -q "fake-installed" \
  || { printf 'FAIL: fake-installed not in package check output\n'; exit 1; }
printf '%s' "$OUT" | grep -q "installed" \
  || { printf 'FAIL: installed status not in output\n'; exit 1; }

cp -r /fixture /tmp/pkgdotfiles
printf '#!/bin/bash\n# @require(not-installable)\necho hi\n' \
  > /tmp/pkgdotfiles/shellrc/hard-fail.sh

if dotd package generate \
  --files /tmp/pkgdotfiles \
  --env-file /tmp/pkgdotfiles/env.yaml \
  --env os=linux 2>/dev/null; then
  printf 'FAIL: package generate should fail with uninstallable @require\n'
  exit 1
fi

printf 'PASS: package-check test\n'
