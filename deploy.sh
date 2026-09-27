#!/usr/bin/env bash
set -euo pipefail

server="${DEPLOY_SERVER:-srv-31-169-124-108}"
ssh "$server" 'cd /opt/cascade-hub-bot && git pull --ff-only && docker compose up -d --build'
