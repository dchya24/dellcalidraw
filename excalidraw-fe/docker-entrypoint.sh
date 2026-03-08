#!/bin/sh
set -e

# Set default PORT if not set
PORT=${PORT:-80}

# Substitute environment variables in nginx config
export PORT
envsubst '$PORT' < /etc/nginx/conf.d/default.conf > /etc/nginx/conf.d/default.conf.tmp
mv /etc/nginx/conf.d/default.conf.tmp /etc/nginx/conf.d/default.conf

# Start nginx
exec nginx -g 'daemon off;'
