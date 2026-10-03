FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata curl && rm -rf /var/lib/apt/lists/*

ARG TARGETARCH=amd64
ARG BINARY_NAME=komari-linux-${TARGETARCH}

COPY ${BINARY_NAME} /app/komari
RUN chmod +x /app/komari

ENV GIN_MODE=release
ENV KOMARI_DB_TYPE=sqlite
ENV KOMARI_DB_FILE=/app/data/komari.db
ENV KOMARI_LISTEN=0.0.0.0:25774

VOLUME ["/app/data"]
EXPOSE 25774

CMD ["/app/komari", "server"]
