#!/bin/sh
set -e

cp -r /fixture /tmp/dotfiles

printf '#!/bin/sh\necho hi\n' > /tmp/newscript.sh

dotd adopt /tmp/newscript.sh \
  --files /tmp/dotfiles \
  --env-file /tmp/dotfiles/env.yaml \
  --to shellrc/

test -f /tmp/dotfiles/shellrc/newscript.sh \
  || { printf 'FAIL: newscript.sh not adopted into shellrc/\n'; exit 1; }
test ! -f /tmp/newscript.sh \
  || { printf 'FAIL: source file should be moved (not copied)\n'; exit 1; }

printf 'PASS: adopt test\n'
