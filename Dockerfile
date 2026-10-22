FROM debian:bookworm-slim AS brotli-build

# Debian's current package predates the CVE-2026-111497 fix. Build the
# upstream security release on the same libc and retain the existing ABI.
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates curl build-essential cmake \
    && rm -rf /var/lib/apt/lists/*
RUN curl --fail --show-error --location --proto '=https' --tlsv1.2 \
      https://codeload.github.com/google/brotli/tar.gz/refs/tags/v1.0.10 -o /tmp/brotli.tar.gz \
    && echo 'b517ebdb5266b963d28d6e639b973479874ec72086deb6ee4a2d639e1b13606b  /tmp/brotli.tar.gz' | sha256sum -c - \
    && mkdir /src && tar -xzf /tmp/brotli.tar.gz -C /src --strip-components=1 \
    && cmake -S /src -B /build -DCMAKE_BUILD_TYPE=Release -DCMAKE_INSTALL_PREFIX=/usr \
      -DCMAKE_INSTALL_LIBDIR=lib/$(dpkg-architecture -qDEB_HOST_MULTIARCH) \
    && cmake --build /build --parallel 2 \
    && ctest --test-dir /build --output-on-failure \
    && DESTDIR=/install cmake --install /build \
    && mkdir -p /package/DEBIAN /package/usr/share/doc/libbrotli1 \
    && libdir=lib/$(dpkg-architecture -qDEB_HOST_MULTIARCH) \
    && mkdir -p /package/usr/$libdir \
    && cp -a /install/usr/$libdir/libbrotli*.so.* /package/usr/$libdir/ \
    && cp /src/LICENSE /package/usr/share/doc/libbrotli1/copyright \
    && printf 'Package: libbrotli1\nVersion: 1.0.10-0komari1\nArchitecture: %s\nMaintainer: Komari Retro\nDepends: libc6 (>= 2.34)\nDescription: Brotli 1.0.10 security release (CVE-2026-111497)\n' "$(dpkg --print-architecture)" > /package/DEBIAN/control \
    && dpkg-deb --build --root-owner-group /package /brotli-security.deb

FROM debian:bookworm-slim

WORKDIR /app

RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates tzdata curl && rm -rf /var/lib/apt/lists/*

COPY --from=brotli-build /brotli-security.deb /tmp/brotli-security.deb
RUN dpkg -i /tmp/brotli-security.deb && ldconfig \
    && curl --version | grep -F 'brotli/1.0.10' \
    && rm /tmp/brotli-security.deb

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
