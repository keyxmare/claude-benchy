#!/bin/sh
# Stage the read-only mounted credentials into a writable config dir so token
# refresh works inside the throwaway container, then hand off to claude.
set -e

mkdir -p "$CLAUDE_CONFIG_DIR"
if [ -f /benchy/creds/.credentials.json ]; then
    cp /benchy/creds/.credentials.json "$CLAUDE_CONFIG_DIR/.credentials.json"
    chmod 600 "$CLAUDE_CONFIG_DIR/.credentials.json"
fi

exec claude "$@"
