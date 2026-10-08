# Icecast2 Docker Image

A multi-platform Docker image for [Icecast](https://icecast.org/), built directly from the Icecast source code.

## Platforms

| Platform, Architecture |
| ---------------------- |
| linux, amd64           |
| linux, arm64           |

## Environment

| Variable                  | Default           | Description                          |
| ------------------------- | ----------------- | ------------------------------------ |
| `ICECAST_HOSTNAME`        | `localhost`       | Icecast hostname                     |
| `ICECAST_LOCATION`        | `Earth`           | Server location                      |
| `ICECAST_ADMIN_EMAIL`     | `admin@localhost` | Administrator email                  |
| `ICECAST_PORT`            | `8000`            | Icecast listening port               |
| `ICECAST_SOURCE_PASSWORD` | `change-me`       | Source client password               |
| `ICECAST_RELAY_PASSWORD`  | `change-me`       | Relay password                       |
| `ICECAST_ADMIN_USER`      | `admin`           | Icecast administrator username       |
| `ICECAST_ADMIN_PASSWORD`  | `change-me`       | Icecast administrator password       |
| `ICECAST_MAX_CLIENTS`     | `100`             | Maximum number of clients            |
| `ICECAST_MAX_SOURCES`     | `2`               | Maximum number of source connections |

## Docker

```bash
docker pull ghcr.io/acayseth/icecast2:latest

docker run -d -p 8000:8000 ghcr.io/acayseth/icecast2:latest
```

## Version

docker pull ghcr.io/acayseth/icecast2:2.5.0

The version tag identifies the Icecast upstream version.

The version tag may be updated when the Docker image, configuration, or wrapper is changed without changing the Icecast version.

## Git commit

Every build is also published with a Git commit tag:

docker pull ghcr.io/acayseth/icecast2:sha-abcdef1

The commit tag identifies the exact Git commit used to build the image.
