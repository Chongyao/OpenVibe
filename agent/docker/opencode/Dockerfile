FROM ubuntu:24.04

RUN apt-get update && apt-get install -y \
    curl git ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Copy opencode from build context
COPY opencode /usr/local/bin/opencode
RUN chmod +x /usr/local/bin/opencode

WORKDIR /project
EXPOSE 4096

ENTRYPOINT ["opencode", "serve"]
CMD ["--port", "4096"]
