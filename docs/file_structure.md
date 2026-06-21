# File Structure

```md
├── api
│   ├── grpc
│   │   ├── generate.go
│   │   └── link_tracker.proto
│   └── bot|scrapper              / openAPI specs
├── cmd
│   ├── agent
│   │   ├── app.go  / di-container
│   │   └── main.go / entry point
│   ├── bot
│   └── scrapper
├── config/
├── deploy
│   ├── docker-compose.yml
│   ├── Dockerfile.*
│   ├── grafana
│   ├── schema              / avro-schemas
│   └── scripts
├── docs/
├── internal
│   ├── agent                 / other service have likely layout(for me this is more clean)
│   │   ├── application
│   │   │   ├── dto.go
│   │   │   ├── fx_module.go
│   │   │   ├── mocks/
│   │   │   ├── process_case.go
│   │   │   └── process_case_test.go
│   │   ├── domain
│   │   │   ├── decision.go
│   │   │   ├── event.go
│   │   │   ├── event_test.go
│   │   │   ├── filter.go
│   │   │   ├── filter_test.go
│   │   │   └── fx_module.go
│   │   └── infrastructure
│   │       ├── ai
│   │       │   ├── fx_module.go
│   │       │   └── summarizer.go
│   │       ├── kafka
│   │       │   ├── consumer
│   │       │   │   ├── consumer.go
│   │       │   │   ├── fx_module.go
│   │       │   │   ├── mocks/
│   │       │   │   └── tools.go
│   │       │   └── producer
│   │       │       ├── fx_module.go
│   │       │       └── producer.go
│   │       ├── repository
│   │       │   ├── fx_module.go
│   │       │   ├── inbox_repo.go
│   │       │   └── outbox_repo.go
│   │       ├── scheduler
│   │       │   ├── fx_module.go
│   │       │   ├── jobs.go
│   │       │   └── scheduler.go
│   │       └── telemetry
│   │           ├── fx_module.go
│   │           └── metrics.go
│   ├── bot
│   │   ├── application
│   │   └── infrastructure
│   │       ├── grpc|http
│   │       │   ├── client
│   │       │   └── server
│   │       ├── kafka
│   │       ├── repository
│   │       ├── scheduler
│   │       ├── telegram
│   │       └── telemetry
│   ├── infrastructure    / sharded infra(reuse code in monorepo)
│   │   ├── repository    / transactor(abstraction to work with psql transaction(put pgx.Transaction inside ctx))
│   │   ├── server        / http|gprc servers abstraction with graceful shutdown
│   │   └── transport
│   │       ├── grpc|http
│   │       │   ├── interceptor|middleware
│   │       │   ├── codegen                 / shared codegen via oapi-codegen|protoc
│   │       └── metrics.go
│   ├── scrapper
│   │   ├── application
│   │   ├── domain
│   │   └── infrastructure
│   │       ├── crawlers
│   │       ├── grpc|http
│   │       │   ├── client
│   │       │   └── server
│   │       ├── kafka
│   │       ├── repository
│   │       │   ├── query_builder
│   │       │   ├── sql
│   │       │   └── valkey          / for get_links requests
│   │       ├── scheduler
│   │       └── telemetry
│   └── tests
├── load_testing
├── migrations
├── pkg
│   ├── ...
│   └── retry
└── README.md
```
