# Go-PriyaAI-bot

PRIYA AI is a lightweight, self-hosted Go service for a local Ollama-powered Instagram companion.
It stores conversations locally in SQLite and is designed to receive DMs from a separate, transparent
browser worker. It does not use Meta APIs, extract browser cookies, or request Instagram credentials.

## Quick start

1. Install Go and [Ollama](https://ollama.com), then run `ollama pull gemma3:1b`.
2. Copy `.env.example` to `.env` and export its values in your shell.
3. Run `go run ./cmd/priya`.
4. Verify with `curl http://127.0.0.1:8080/health`.

Administrative endpoints require `Authorization: Bearer $PRIYA_ADMIN_TOKEN`. The service deliberately
starts with browser automation paused: a browser worker must be configured and a human must complete any login.

## Safety boundary

This project never stores passwords, cookies, or authentication tokens. It does not bypass challenges,
CAPTCHAs, rate limits, or platform controls. When a browser reports a login or security challenge, processing
is paused for manual intervention.
