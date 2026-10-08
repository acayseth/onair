# syntax=docker/dockerfile:1

# ============================================================
# Build stage
# ============================================================
FROM debian:trixie-slim AS builder

ARG ICECAST_VERSION

ENV DEBIAN_FRONTEND=noninteractive

RUN echo "deb http://deb.debian.org/debian trixie-backports main" \
        > /etc/apt/sources.list.d/trixie-backports.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        automake \
        build-essential \
        ca-certificates \
        curl \
        libcurl4-openssl-dev \
        libogg-dev \
        librhash-dev \
        libspeex-dev \
        libssl-dev \
        libtheora-dev \
        libtool \
        libvorbis-dev \
        libxml2-dev \
        libxslt1-dev \
        pkg-config \
    && apt-get install -y --no-install-recommends \
        -t trixie-backports \
        libigloo-dev \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /tmp

RUN curl -fsSL \
        "https://downloads.xiph.org/releases/icecast/icecast-${ICECAST_VERSION}.tar.gz" \
        -o icecast.tar.gz \
    && tar -xzf icecast.tar.gz \
    && rm icecast.tar.gz

WORKDIR /tmp/icecast-${ICECAST_VERSION}

RUN ./configure \
        --prefix=/usr \
        --sysconfdir=/etc/icecast \
        --localstatedir=/var \
        --disable-static \
    && make -j"$(nproc)" \
    && make DESTDIR=/tmp/rootfs install


# ============================================================
# Runtime stage
# ============================================================
FROM debian:trixie-slim

ARG ICECAST_VERSION

ENV ICECAST_VERSION="${ICECAST_VERSION}" \
    TZ=UTC \
    DEBIAN_FRONTEND=noninteractive

LABEL \
    org.opencontainers.image.title="Icecast" \
    org.opencontainers.image.description="Icecast streaming media server built from source" \
    org.opencontainers.image.version="${ICECAST_VERSION}" \
    org.opencontainers.image.source="https://github.com/acayseth/on-air"

# ============================================================
# Runtime dependencies
# ============================================================

RUN echo "deb http://deb.debian.org/debian trixie-backports main" \
        > /etc/apt/sources.list.d/trixie-backports.list \
    && apt-get update \
    && apt-get install -y --no-install-recommends \
        ca-certificates \
        gettext-base \
        libcurl4t64 \
        libogg0 \
        librhash1 \
        libspeex1 \
        libssl3t64 \
        libtheora0 \
        libvorbis0a \
        libxml2 \
        libxslt1.1 \
    && apt-get install -y --no-install-recommends \
        -t trixie-backports \
        libigloo0t64 \
    && rm -rf /var/lib/apt/lists/*

# ============================================================
# Icecast installation
# ============================================================

COPY --from=builder /tmp/rootfs/ /

# Remove Icecast's default configuration.
RUN rm -f /etc/icecast/icecast.xml

# ============================================================
# User
# ============================================================

RUN groupadd \
        --system \
        --gid 10001 \
        icecast \
    && useradd \
        --system \
        --uid 10001 \
        --gid 10001 \
        --home-dir /var/lib/icecast \
        --no-create-home \
        --shell /usr/sbin/nologin \
        icecast

# ============================================================
# Runtime directories
# ============================================================

RUN mkdir -p \
        /etc/icecast \
        /var/log/icecast \
        /var/lib/icecast \
        /var/lib/icecast/web \
    && chown -R icecast:icecast \
        /etc/icecast \
        /var/log/icecast \
        /var/lib/icecast

# ============================================================
# Our configuration
# ============================================================

COPY icecast.xml /etc/icecast/icecast.xml.source

# ============================================================
# Entrypoint
# ============================================================

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

RUN chmod 0755 /usr/local/bin/docker-entrypoint.sh \
    && chown -R icecast:icecast \
        /etc/icecast \
        /usr/local/bin/docker-entrypoint.sh

WORKDIR /var/lib/icecast

EXPOSE 8000

USER icecast

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

CMD ["icecast", "-c", "/etc/icecast/icecast.xml"]
