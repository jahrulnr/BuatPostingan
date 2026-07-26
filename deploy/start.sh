#!/bin/sh
set -eu

: "${BP_HTTP_ADDR:=127.0.0.1:1313}"
: "${BP_WEB_ROOT:=/app/web}"
: "${BP_STORAGE_ROOT:=/app/storage/webchat}"
: "${BP_DOCS_ROOT:=/app/resources/webchat/docs}"
: "${BP_PROMPTS_ROOT:=/app/resources/webchat/prompts}"
: "${BP_TOOLS_ROOT:=/app/resources/webchat/tools}"
: "${BP_SKILLS_ROOT:=/app/resources/webchat/skills}"
: "${BP_WORKSPACE_ROOT:=/app/storage/pages}"

export BP_HTTP_ADDR BP_WEB_ROOT BP_STORAGE_ROOT BP_DOCS_ROOT BP_PROMPTS_ROOT
export BP_TOOLS_ROOT BP_SKILLS_ROOT BP_WORKSPACE_ROOT

if [ ! -e /app/storage/pages/home ]; then
    cp -R /app/homepage /app/storage/pages/home
    mkdir -p /app/storage/pages/.published
    ln -s ../home /app/storage/pages/.published/home
fi

/usr/local/bin/buatpostingan &
app_pid=$!

trap 'kill -TERM "$app_pid" 2>/dev/null || true; wait "$app_pid" 2>/dev/null || true' INT TERM
nginx -g 'daemon off;' &
nginx_pid=$!

wait "$nginx_pid"
kill -TERM "$app_pid" 2>/dev/null || true
wait "$app_pid" 2>/dev/null || true
