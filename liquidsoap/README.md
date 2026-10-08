# Liquidsoap Docker Image

A multi-platform Docker image for [Liquidsoap](https://www.liquidsoap.info/) designed for running automated radio streams with Icecast.

## Platforms

| Platform, Architecture |
| ---------------------- |
| linux, amd64           |
| linux, arm64           |

## Environment

| Variable                  | Default       | Description                              |
| ------------------------- | ------------- | ---------------------------------------- |
| `TZ`                      | `UTC`         | Container timezone                       |
| `ICECAST_HOST`            | `icecast`     | Icecast hostname                         |
| `ICECAST_PORT`            | `8000`        | Icecast port                             |
| `ICECAST_USER`            | `source`      | Icecast source username                  |
| `ICECAST_PASSWORD`        | empty         | Icecast source password                  |
| `ICECAST_MOUNT`           | `/radio.mp3`  | Icecast mount point                      |
| `ICECAST_NAME`            | `Radio Dream` | Stream name                              |
| `ICECAST_GENRE`           | `Rock`        | Stream genre                             |
| `ICECAST_DESCRIPTION`     | `Radio Dream` | Stream description                       |
| `ICECAST_URL`             | empty         | Public stream URL                        |
| `JINGLE_EVERY`            | `4`           | Number of tracks between jingles         |
| `STREAM_BITRATE`          | `128`         | MP3 stream bitrate in kbps               |
| `DISCOGS_CONSUMER_KEY`    | empty         | Discogs API consumer key                 |
| `DISCOGS_CONSUMER_SECRET` | empty         | Discogs API consumer secret              |
| `S3_BUCKET`               | empty         | S3 bucket name; enables S3 mode when set |
| `S3_TRACKS_PREFIX`        | `tracks`      | S3 prefix containing tracks              |
| `S3_JINGLES_PREFIX`       | `jingles`     | S3 prefix containing jingles             |
| `S3_ACCESS_KEY_ID`        | empty         | S3 access key ID                         |
| `S3_SECRET_ACCESS_KEY`    | empty         | S3 secret access key                     |

## Docker

```bash
docker pull ghcr.io/acayseth/liqudisoap:latest

docker run -d -p 8000:8000 ghcr.io/acayseth/liqudisoap:latest
```
