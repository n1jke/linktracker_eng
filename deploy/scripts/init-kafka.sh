#!/usr/bin/env bash
set -euo pipefail

BOOTSTRAP_SERVERS="${BOOTSTRAP_SERVERS:-kafka-1:9092,kafka-2:9092,kafka-3:9092}"
KAFKA_TOPICS_BIN="${KAFKA_TOPICS_BIN:-/opt/kafka/bin/kafka-topics.sh}"

if [ ! -x "${KAFKA_TOPICS_BIN}" ]; then
  echo "[init-kafka] kafka topics bin not found: ${KAFKA_TOPICS_BIN}"
  exit 1
fi

echo "[init-kafka] Starting up Kafka cluster: ${BOOTSTRAP_SERVERS}"
for i in $(seq 1 10); do
  if "${KAFKA_TOPICS_BIN}" --bootstrap-server "${BOOTSTRAP_SERVERS}" --list >/dev/null 2>&1; then
    echo "[init-kafka] Kafka ready"
    break
  fi

  if [ "${i}" -eq 10 ]; then
    echo "[init-kafka] Kafka did not ready"
    exit 1
  fi

  sleep 3
done

# raw link updates from scrapper -> agent
"${KAFKA_TOPICS_BIN}" --bootstrap-server "${BOOTSTRAP_SERVERS}" --create --if-not-exists \
  --topic "${KAFKA_RAW_UPDATES_TOPIC}" --partitions 3 --replication-factor 3 --config min.insync.replicas=2

# rawDLQ for raw link updates
"${KAFKA_TOPICS_BIN}" --bootstrap-server "${BOOTSTRAP_SERVERS}" --create --if-not-exists \
  --topic "${KAFKA_RAW_DLQ_TOPIC}" --partitions 3 --replication-factor 3 --config min.insync.replicas=2

# prepared link updates from agen -> bot
"${KAFKA_TOPICS_BIN}" --bootstrap-server "${BOOTSTRAP_SERVERS}" --create --if-not-exists \
  --topic "${KAFKA_PREP_UPDATES_TOPIC}" --partitions 3 --replication-factor 3 --config min.insync.replicas=2

# prepDLQ for prepared link updates
"${KAFKA_TOPICS_BIN}" --bootstrap-server "${BOOTSTRAP_SERVERS}" --create --if-not-exists \
  --topic "${KAFKA_PREP_DLQ_TOPIC}" --partitions 3 --replication-factor 3 --config min.insync.replicas=2

echo "[init-kafka] link-raw-updates topic"
"${KAFKA_TOPICS_BIN}" --bootstrap-server "${BOOTSTRAP_SERVERS}" --describe --topic "${KAFKA_RAW_UPDATES_TOPIC}"

echo "[init-kafka] link-raw-updates-dlq topic"
"${KAFKA_TOPICS_BIN}" --bootstrap-server "${BOOTSTRAP_SERVERS}" --describe --topic "${KAFKA_RAW_DLQ_TOPIC}"

echo "[init-kafka] link-processed-updates topic"
"${KAFKA_TOPICS_BIN}" --bootstrap-server "${BOOTSTRAP_SERVERS}" --describe --topic "${KAFKA_PREP_UPDATES_TOPIC}"

echo "[init-kafka] link-processed-updates-dlq topic"
"${KAFKA_TOPICS_BIN}" --bootstrap-server "${BOOTSTRAP_SERVERS}" --describe --topic "${KAFKA_PREP_DLQ_TOPIC}"

echo "[init-kafka] Kafka cluster up"
