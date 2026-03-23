# Runtime image - copy locally built binaries into the container
FROM debian:stable-slim

RUN apt-get update && apt-get install -y --no-install-recommends \
    ca-certificates \
    sqlite3 \
    && rm -rf /var/lib/apt/lists/* \
    && useradd -m -s /bin/bash app

WORKDIR /app

# Build binaries locally first, then copy them into the image
COPY bin/forum-server /app/forum-server
COPY frontend/dist /app/frontend/dist
COPY config.toml /app/config.toml

RUN mkdir -p /data && chown -R app:app /app /data

USER app

EXPOSE 8080

ENTRYPOINT ["/app/forum-server"]
