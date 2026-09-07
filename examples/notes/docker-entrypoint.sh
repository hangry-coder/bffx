#!/bin/sh
set -e
# Named volumes for SQLite mount as root-owned; ensure bffx can write before dropping privs.
mkdir -p /app/.bffx/data
if [ "$(id -u)" = "0" ]; then
	chown -R bffx:bffx /app/.bffx/data
	exec su-exec bffx "$@"
fi
exec "$@"
