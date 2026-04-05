#!/bin/sh
set -e

# Inject runtime config into index.html at container start.
if [ -n "${API_BASE_URL}" ]; then
  sed -i "s|__API_BASE_URL_PLACEHOLDER__|${API_BASE_URL}|g" /usr/share/nginx/html/index.html
fi

exec nginx -g 'daemon off;'
