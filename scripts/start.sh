#!/usr/bin/env bash
set -euo pipefail
test -f .env && set -a && source .env && set +a
exec go run ./cmd/priya
