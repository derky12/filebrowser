#!/bin/sh
# Create a user/group matching PUID/PGID so files written to bind mounts get
# the expected owner, then drop privileges and hand over to the stock init.
set -e

PUID="${PUID:-1000}"
PGID="${PGID:-1000}"

if [ "$PUID" = "0" ]; then
  exec /init.sh "$@"
fi

getent group "$PGID" >/dev/null 2>&1 || addgroup -g "$PGID" filebrowser
getent passwd "$PUID" >/dev/null 2>&1 || adduser -D -H -u "$PUID" -G "$(getent group "$PGID" | cut -d: -f1)" filebrowser

chown "$PUID:$PGID" /config /database

exec su-exec "$PUID:$PGID" /init.sh "$@"
