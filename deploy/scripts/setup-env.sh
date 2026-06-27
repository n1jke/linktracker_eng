#!/bin/bash
set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
ENV_FILE="${SCRIPT_DIR}/../.env"

# Create .env from example if it doesn't exist
if [ ! -f "$ENV_FILE" ]; then
  cp "${SCRIPT_DIR}/../.env.example" "$ENV_FILE"
  echo "Created .env from .env.example"
fi

# Override tokens from environment variables (Codespaces secrets)
for key in TELEGRAM_TOKEN GITHUB_TOKEN AGENT_HG_TOKEN; do
  val="${!key}"
  if [ -n "$val" ]; then
    sed -i "s@^${key}=todo@${key}=${val}@" "$ENV_FILE"
    echo "Set ${key}"
  fi
done

echo "✅ Environment configured"
