#!/bin/sh
set -e

# Inject runtime config into index.html at container start.
# Uses a temp file approach since sed -i may fail on read-only or permission-restricted filesystems.
if [ -n "${API_BASE_URL}" ]; then
  tmpfile=$(mktemp)
  sed "s|__API_BASE_URL_PLACEHOLDER__|${API_BASE_URL}|g" /usr/share/nginx/html/index.html > "$tmpfile"
  cat "$tmpfile" > /usr/share/nginx/html/index.html
  rm -f "$tmpfile"
fi

exec nginx -g 'daemon off;'
