# metrics

## RED Dashboard

### Rate

sum by (command) (rate(command_requests_total{job="$app"}[5m]))
sum by (source) (rate(api_requests_total{job="$app"}[5m]))

### Duration

histogram_quantile(0.50, sum by (le) (rate({__name__=~"command_duration_ms_total_bucket|request_duration_ms_total_bucket", job="$app"}[5m])))
histogram_quantile(0.90, sum by (le) (rate({__name__=~"command_duration_ms_total_bucket|request_duration_ms_total_bucket", job="$app"}[5m])))
histogram_quantile(0.99, sum by (le) (rate({__name__=~"command_duration_ms_total_bucket|request_duration_ms_total_bucket", job="$app"}[5m])))

### Memory

process_resident_memory_bytes{job="$app"}
process_virtual_memory_bytes{job="$app"}
go_memstats_heap_inuse_bytes{job="$app"}

### Errors

absent(up{job="$app"})

### Goroutines

go_goroutines{job="$app"}

---

## Business Dashboard

### Links on track

links_on_track_total{job="scrapper"}

### Rate bot

sum by (command) (rate(command_requests_total{job="$app"}[1m]))

### Rate scrapper

sum by (source) (rate(api_requests_total{job="$app"}[1m]))

### Scrapper duration

histogram_quantile(0.50, sum by (le, scope) (rate(request_duration_ms_total_bucket{job="$app"}[5m])))
histogram_quantile(0.95, sum by (le, scope) (rate(request_duration_ms_total_bucket{job="$app"}[5m])))
histogram_quantile(0.99, sum by (le, scope) (rate(request_duration_ms_total_bucket{job="$app"}[5m])))

### Bot duration

histogram_quantile(0.50, sum by (le, scope) (rate(command_duration_ms_total_bucket{job="$app"}[5m])))
histogram_quantile(0.90, sum by (le, scope) (rate(command_duration_ms_total_bucket{job="$app"}[5m])))
histogram_quantile(0.99, sum by (le, scope) (rate(command_duration_ms_total_bucket{job="$app"}[5m])))

### Agent duration

histogram_quantile(0.50, sum by (le, scope_type) (rate(request_duration_ms_total_bucket{job="agent"}[5m])))
histogram_quantile(0.95, sum by (le, scope_type) (rate(request_duration_ms_total_bucket{job="agent"}[5m])))
histogram_quantile(0.99, sum by (le, scope_type) (rate(request_duration_ms_total_bucket{job="agent"}[5m])))

### Notification count

sent_notification_total{job="$app"}
rate(sent_notification_total{job="$app"}[5m]) * 60

## Alert

expr: process_resident_memory_bytes{job=~"bot|scrapper|agent"} > 200*1024*1024
for: 1m
severity: P3
