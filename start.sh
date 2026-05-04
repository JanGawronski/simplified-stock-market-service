#!/usr/bin/env bash
set -euo pipefail

if [[ $# -ne 1 ]]; then
  echo "Usage: ./start.sh <port>"
  exit 1
fi

export HOST_PORT="$1"
docker compose up --build -d --remove-orphans
echo "Stock market service is available at http://localhost:${HOST_PORT}"

