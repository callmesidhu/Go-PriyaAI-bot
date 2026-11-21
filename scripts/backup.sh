#!/usr/bin/env bash
set -euo pipefail
mkdir -p backups
stamp=$(date +%Y%m%d-%H%M%S)
sqlite3 data/priya.db ".backup 'backups/priya-${stamp}.db'"
echo "Created backups/priya-${stamp}.db"
