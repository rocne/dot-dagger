FROM ubuntu:24.04
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY apply.sh /tests/apply.sh
COPY context.sh /tests/context.sh
COPY dag-order.sh /tests/dag-order.sh
COPY dry-run.sh /tests/dry-run.sh
COPY idempotent.sh /tests/idempotent.sh
COPY check.sh /tests/check.sh
COPY list.sh /tests/list.sh
COPY bin.sh /tests/bin.sh
COPY symlinks-nested.sh /tests/symlinks-nested.sh
COPY disable.sh /tests/disable.sh
COPY packages.sh /tests/packages.sh
COPY conflict.sh /tests/conflict.sh
COPY unapply.sh /tests/unapply.sh
COPY unapply-cancel.sh /tests/unapply-cancel.sh
COPY compose.sh /tests/compose.sh
COPY macos-apply.sh /tests/macos-apply.sh
COPY setup.sh /tests/setup.sh
COPY teardown-confirm.sh /tests/teardown-confirm.sh
COPY teardown-cancel.sh /tests/teardown-cancel.sh
COPY init.sh /tests/init.sh
COPY config-cmds.sh /tests/config-cmds.sh
COPY env-cmds.sh /tests/env-cmds.sh
COPY adopt.sh /tests/adopt.sh
COPY bundle.sh /tests/bundle.sh
COPY dag-check.sh /tests/dag-check.sh
COPY package-check.sh /tests/package-check.sh
COPY annotate.sh /tests/annotate.sh
COPY concepts.sh /tests/concepts.sh
COPY procure/ /procure/
