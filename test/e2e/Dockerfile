FROM ubuntu:24.04
RUN apt-get update \
    && apt-get install -y --no-install-recommends curl ca-certificates \
    && rm -rf /var/lib/apt/lists/*
COPY binary.sh /tests/binary.sh
COPY install.sh /tests/install.sh
COPY combined.sh /tests/combined.sh
