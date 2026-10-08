#!/bin/sh

set -eu

# ============================================================
# Default environment variables
# ============================================================

: "${ICECAST_HOST:=icecast}"
: "${ICECAST_PORT:=8000}"
: "${ICECAST_PASSWORD:=change-me}"

: "${S3_BUCKET:=}"
: "${S3_REGION:=}"
: "${S3_ENDPOINT:=}"
: "${S3_ACCESS_KEY_ID:=}"
: "${S3_SECRET_ACCESS_KEY:=}"
: "${S3_PREFIX:=}"

# ============================================================
# Export environment variables
# ============================================================

export ICECAST_HOST
export ICECAST_PORT
export ICECAST_PASSWORD

export S3_BUCKET
export S3_REGION
export S3_ENDPOINT
export S3_ACCESS_KEY_ID
export S3_SECRET_ACCESS_KEY
export S3_PREFIX

# ============================================================
# Information
# ============================================================

echo "Liquidsoap ${LIQUIDSOAP_VERSION:-unknown}"

echo "Configuration:"
echo "  ICECAST_HOST: ${ICECAST_HOST}"
echo "  ICECAST_PORT: ${ICECAST_PORT}"

if [ -n "${S3_BUCKET}" ]; then
    echo "  S3 streaming: enabled"
    echo "  S3_BUCKET: ${S3_BUCKET}"
    echo "  S3_REGION: ${S3_REGION:-default}"
    echo "  S3_ENDPOINT: ${S3_ENDPOINT:-default}"
    echo "  S3_PREFIX: ${S3_PREFIX:-/}"
else
    echo "  S3 streaming: disabled"
fi

# ============================================================
# Verify required directories
# ============================================================

for directory in \
    /app/storage \
    /app/storage/tracks \
    /app/storage/jingles
do
    if [ ! -d "$directory" ]; then
        echo "ERROR: Required directory is missing: $directory" >&2
        exit 1
    fi

    if [ ! -r "$directory" ] || [ ! -x "$directory" ]; then
        echo "ERROR: Directory is not readable: $directory" >&2
        exit 1
    fi
done

# ============================================================
# S3 streaming gateway
# ============================================================

S3_GATEWAY="/usr/local/bin/s3sg"
gateway_pid=""

if [ -n "${S3_BUCKET}" ]; then

    if [ ! -x "$S3_GATEWAY" ]; then
        echo "ERROR: S3 streaming gateway is not executable: $S3_GATEWAY" >&2
        exit 1
    fi

    echo "Starting S3 streaming gateway..."

    "$S3_GATEWAY" &
    gateway_pid=$!

    # --------------------------------------------------------
    # Wait for gateway
    # --------------------------------------------------------

    attempt=0
    max_attempts=50

    while [ "$attempt" -lt "$max_attempts" ]; do

        if "$S3_GATEWAY" --health-check >/dev/null 2>&1; then
            echo "S3 streaming gateway is ready."
            break
        fi

        if ! kill -0 "$gateway_pid" 2>/dev/null; then
            wait "$gateway_pid" 2>/dev/null || true

            echo "ERROR: S3 streaming gateway failed to start." >&2
            exit 1
        fi

        attempt=$((attempt + 1))
        sleep 0.1
    done

    if ! "$S3_GATEWAY" --health-check >/dev/null 2>&1; then
        echo "ERROR: S3 streaming gateway did not become ready." >&2

        kill -TERM "$gateway_pid" 2>/dev/null || true
        wait "$gateway_pid" 2>/dev/null || true

        exit 1
    fi
fi

# ============================================================
# Start Liquidsoap
# ============================================================

echo "Starting Liquidsoap..."

"$@" &
liquidsoap_pid=$!

# ============================================================
# Shutdown handling
# ============================================================

stop_children() {
    echo "Stopping services..."

    kill -TERM "$liquidsoap_pid" 2>/dev/null || true

    if [ -n "${gateway_pid}" ]; then
        kill -TERM "$gateway_pid" 2>/dev/null || true
    fi

    wait "$liquidsoap_pid" 2>/dev/null || true

    if [ -n "${gateway_pid}" ]; then
        wait "$gateway_pid" 2>/dev/null || true
    fi
}

trap stop_children INT TERM

# ============================================================
# Wait for Liquidsoap
# ============================================================

set +e
wait "$liquidsoap_pid"
status=$?
set -e

stop_children

exit "$status"
