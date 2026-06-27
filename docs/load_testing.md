# Load Testing

## 1. Configuration

- **DB:** 1,000 users each with 100 subscriptions => 100,000 subscriptions
- **Virtual Users (VU):** 32 (2 * cpu_count)
- **Protocol:** gRPC / HTTP with and without Valkey
- **Test stages:**
  - Ramp-up: 1 minute
  - Stage: 5 minutes
  - Ramp-down: 30 seconds
- **Read/Write ratio:** 99/1

## 2. Results

| Metric | http+cache | http | grpc | grpc+cache |
| --- | --- | --- | --- | --- |
| **rps** | 273.78 | 277.7 | 265.14 | 271.05 |
| **avg** | 2ms | 4ms | 5ms | 2ms |
| **p90** | 3ms | 7ms | 8ms | 3ms |
| **p99** | 9ms | 19ms | 22ms | 9ms |
| **fail-rate** | 0.6% | 0.6% | 0.1% | 0.1% |
| **data_received** | 950mb | 885mb | 659 mb | 685mb |

---

## 3. Conclusions

- The error rate is very low across all configurations, though relying on it under such synthetic load may not be representative.
- Response percentiles for Valkey-backed variants are 1.8–2.5 times lower on average.
- HTTP was slightly faster than gRPC, which can be explained by the test script being written in JavaScript (JSON is its native format).
