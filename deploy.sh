#!/usr/bin/env bash
set -euo pipefail

: "${DEPLOY_SERVER:?Укажите SSH-адрес сервера в DEPLOY_SERVER}"
: "${DEPLOY_PATH:?Укажите путь к клону репозитория в DEPLOY_PATH}"
printf -v remote_path '%q' "$DEPLOY_PATH"
ssh "$DEPLOY_SERVER" "cd $remote_path && git pull --ff-only && docker compose up -d --build"
