# syntax=docker/dockerfile:1

ARG LIQUIDSOAP_VERSION=2.4.5

# ================================================================================
#                           build go-lang/s3-stream-gateway
# ================================================================================

FROM golang:1.27.1-alpine AS s3sg-builder

WORKDIR /src

# COPY s3sg/go.mod s3sg/go.sum ./
# RUN go mod download

COPY s3sg/ ./

RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/s3sg .

# ================================================================================
#                           build acayseth/liquidsoap
# ================================================================================

FROM savonet/liquidsoap:v${LIQUIDSOAP_VERSION}

ARG LIQUIDSOAP_VERSION

LABEL \
    org.opencontainers.image.title="Liquidsoap" \
    org.opencontainers.image.description="" \
    org.opencontainers.image.version="${LIQUIDSOAP_VERSION}" \
    org.opencontainers.image.source="https://github.com/acayseth/on-air"

USER 0:0

WORKDIR /app

RUN mkdir -p \
        /app/storage/tracks \
        /app/storage/jingles \
    && chown 10001:10001 \
        /app/storage \
        /app/storage/tracks \
        /app/storage/jingles

COPY --chown=10001:10001 main.liq /app/main.liq

COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh

COPY --from=s3sg-builder \
    --chown=10001:10001 \
    /out/s3sg \
    /usr/local/bin/s3sg

RUN chmod 0555 \
    /usr/local/bin/docker-entrypoint.sh \
    /usr/local/bin/s3sg

ENV HOME=/tmp \
    XDG_CACHE_HOME=/tmp

USER 10001:10001

ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]

CMD ["liquidsoap", "/app/main.liq"]
