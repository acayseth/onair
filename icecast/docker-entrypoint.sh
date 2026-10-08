#!/bin/sh

set -eu

# ============================================================
# Default environment variables
# ============================================================

: "${ICECAST_HOSTNAME:=localhost}"
: "${ICECAST_LOCATION:=Earth}"
: "${ICECAST_ADMIN_EMAIL:=admin@localhost}"

: "${ICECAST_PORT:=8000}"

: "${ICECAST_SOURCE_PASSWORD:=change-me}"
: "${ICECAST_RELAY_PASSWORD:=change-me}"

: "${ICECAST_ADMIN_USER:=admin}"
: "${ICECAST_ADMIN_PASSWORD:=change-me}"

: "${ICECAST_MAX_CLIENTS:=100}"
: "${ICECAST_MAX_SOURCES:=2}"

export TZ
export ICECAST_HOSTNAME
export ICECAST_LOCATION
export ICECAST_ADMIN_EMAIL
export ICECAST_PORT
export ICECAST_SOURCE_PASSWORD
export ICECAST_RELAY_PASSWORD
export ICECAST_ADMIN_USER
export ICECAST_ADMIN_PASSWORD
export ICECAST_MAX_CLIENTS
export ICECAST_MAX_SOURCES

echo "Icecast ${ICECAST_VERSION}"
echo "Generating configuration: /etc/icecast/icecast.xml"

# ============================================================
# Generate configuration
# ============================================================

envsubst \
    '${ICECAST_HOSTNAME}
     ${ICECAST_LOCATION}
     ${ICECAST_ADMIN_EMAIL}
     ${ICECAST_PORT}
     ${ICECAST_SOURCE_PASSWORD}
     ${ICECAST_RELAY_PASSWORD}
     ${ICECAST_ADMIN_USER}
     ${ICECAST_ADMIN_PASSWORD}
     ${ICECAST_MAX_CLIENTS}
     ${ICECAST_MAX_SOURCES}' \
    < "/etc/icecast/icecast.xml.source" \
    > "/etc/icecast/icecast.xml"

# ============================================================
# Validate configuration
# ============================================================

if command -v xmllint >/dev/null 2>&1; then
    echo "Validating Icecast configuration..."

    if ! xmllint --noout "/etc/icecast/icecast.xml"; then
        echo "ERROR: Invalid Icecast XML configuration."
        exit 1
    fi
fi

# ============================================================
# Security warning
# ============================================================

if [ "${ICECAST_SOURCE_PASSWORD}" = "change-me" ]; then
    echo "WARNING: Using the default Icecast source password."
fi

if [ "${ICECAST_ADMIN_PASSWORD}" = "change-me" ]; then
    echo "WARNING: Using the default Icecast admin password."
fi

echo "Starting Icecast..."

exec "$@"
