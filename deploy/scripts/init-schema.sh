#!/usr/bin/env bash
set -euo pipefail

SUBJECT="${KAFKA_RAW_UPDATES_TOPIC}-value"
URL="${SCHEMA_REGISTRY_URL}/subjects/${SUBJECT}/versions"

echo "Publishing schema to ${URL}..."

RAW_SCHEMA=$(cat /schema/0002_raw_update.json | tr -d '\n' | sed 's/"/\\"/g')
RAW_PAYLOAD="{\"schema\": \"${RAW_SCHEMA}\"}"

RAW_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d "${RAW_PAYLOAD}" \
  "${URL}")


if echo "$RAW_RESPONSE" | grep -q "id"; then
  echo "Success!"
  echo "Response: $RAW_RESPONSE"
else
  echo "Failed to publish raw-updates schema!"
  echo "Response: $RAW_RESPONSE"
  exit 1
fi

SUBJECT="${KAFKA_PREP_UPDATES_TOPIC}-value"
URL="${SCHEMA_REGISTRY_URL}/subjects/${SUBJECT}/versions"

echo "Publishing schema to ${URL}..."

PREP_SCHEMA=$(cat /schema/0003_prepared_update.json | tr -d '\n' | sed 's/"/\\"/g')
PREP_PAYLOAD="{\"schema\": \"${PREP_SCHEMA}\"}"

PREP_RESPONSE=$(curl -s -X POST \
  -H "Content-Type: application/vnd.schemaregistry.v1+json" \
  -d "${PREP_PAYLOAD}" \
  "${URL}")

if echo "$PREP_RESPONSE" | grep -q "id"; then
  echo "Success!"
  echo "Response: $PREP_RESPONSE"
else
  echo "Failed to publish prepared-updates schema!"
  echo "Response: $PREP_RESPONSE"
  exit 1
fi