#!/usr/bin/env bash
set -euo pipefail
command -v go >/dev/null || { echo "Install Go first: https://go.dev/dl/"; exit 1; }
command -v ollama >/dev/null || { echo "Install Ollama first: https://ollama.com/download"; exit 1; }
ollama pull gemma3:1b
go mod download
mkdir -p data
echo "Dependencies installed. Copy .env.example, choose a strong admin token, then run scripts/start.sh."
