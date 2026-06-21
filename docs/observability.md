# Observability

## Метрики

Все сервисы используют Push-модель: метрики собираются в `prometheus.Registry` и каждые 10s пушатся в **Pushgateway** через `GatewayPusher`. Данные визуализируются в Grafana.

### Scrapper Service

| Метрика | Тип | Labels | Описание |
| --------- | ----- | -------- | ---------- |
| `links_on_track_total` | Gauge | - | Количество отслеживаемых ссылок |
| `request_duration_ms_total` | Histogram | `scope`, `scope_type` | Длительность операций (crawl, db queries, cache) |
| `api_requests_total` | Counter | `source` | Количество запросов к внешним API (github, stackoverflow) |

### Bot Service

| Метрика | Тип | Labels | Описание |
| --------- | ----- | -------- | ---------- |
| `command_requests_total` | Counter | `command` | Количество обработанных команд (/track, /untrack, /list и т.д.) |
| `command_duration_ms_total` | Histogram | `scope`, `scope_type` | Длительность обработки команд |
| `sent_notification_total` | Counter | - | Количество доставленных уведомлений |

### Agent Service

| Метрика | Тип | Labels | Описание |
| --------- | ----- | -------- | ---------- |
| `request_duration_ms_total` | Histogram | `scope`, `scope_type` | Длительность операций (filtering, summarization, grouping) |

### Дашборды Grafana

#### RED Dashboard

- **Rate:** `rate(command_requests_total[5m])` по командам, `rate(api_requests_total[5m])` по источникам
- **Duration:** p50/p90/p99 latency через `histogram_quantile` для `command_duration_ms_total` и `request_duration_ms_total`
- **Memory:** `process_resident_memory_bytes`, `process_virtual_memory_bytes`, `go_memstats_heap_inuse_bytes`
- **Errors:** `absent(up{job=~"bot|scrapper|agent"})`
- **Goroutines:** `go_goroutines{job=~"bot|scrapper|agent"}`

#### Business Dashboard

- **Links on track:** `links_on_track_total{job="scrapper"}` - сколько ссылок под отслеживанием
- **Command rate (bot):** `rate(command_requests_total[1m])` by command
- **API rate (scrapper):** `rate(api_requests_total[1m])` by source
- **Duration (scrapper):** p50/p95/p99 `request_duration_ms_total` by scope
- **Duration (bot):** p50/p90/p99 `command_duration_ms_total` by scope
- **Duration (agent):** p50/p95/p99 `request_duration_ms_total` by scope_type
- **Notification count:** `sent_notification_total` + `rate(sent_notification_total[5m]) * 60`

#### Examples

![example-1](img/metrics_scrapper.png)

![example-2](img/metrics_bot.png)

![example-3](img/metrics.png)

### Алерты

- **High Memory Usage:** `process_resident_memory_bytes > 200MB` в течение 1 минуты, настроен через Grafana Alerting

---

## Логи

Сервисы используют пакет `log/slog` для структурированного логирования:

```go
slog.New(slog.NewJSONHandler(os.Stdout, nil))
```

- **Структурированные:**, также каждый лог выводится в json(todo: add Loki or ELK)
- **Аттрибуты логов** вместо форматирования строк используются типизированные ключи (`slog.String`, `slog.Int64`, `slog.Any`, `slog.Duration`, ...)
- **Контекст модуля:** каждый компонент имеет свой контекст `logger.With(slog.String("module", "..."))`

### Примеры логов

```json
{"level":"INFO","msg":"crawl resource start","link":"github.com/owner/repo","module":"crawler-service","time":"______"}
{"level":"INFO","msg":"resource crawled","link":"github.com/owner/repo","module":"crawler-service","time":"______"}

{"level":"INFO","msg":"subscribe start","link":"github.com/owner/repo","client_id":________}
{"level":"INFO","msg":"subscribe end","link":"github.com/owner/repo","client_id":________}

{"level":"INFO","msg":"request","method":"POST","path":"/links","duration":"12ms"}

{"level":"ERROR","msg":"sending request","err":"","module":"summarizer"}

{"level":"INFO","msg":"metrics pushed","module":"push-publisher"}
{"level":"ERROR","msg":"push metrics to gateway","err":"...","module":"push-publisher"}
```
