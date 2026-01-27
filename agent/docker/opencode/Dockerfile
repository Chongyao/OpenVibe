FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    curl git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy local opencode binary
COPY opencode /usr/local/bin/opencode
RUN chmod +x /usr/local/bin/opencode

WORKDIR /project
EXPOSE 4096

# Listen on all interfaces when using --network host
ENTRYPOINT ["/bin/sh", "-c", "/usr/local/bin/opencode serve --hostname 0.0.0.0 --port 4096"]
CMD []
