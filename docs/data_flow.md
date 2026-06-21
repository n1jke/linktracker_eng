# Data Flow — Event Lifecycle

```mermaid
sequenceDiagram
    participant User as Telegram User
    participant Bot as Bot Service
    participant Scrapper as Scrapper Service
    participant DS as Data Stores
    participant API as External API
    participant K_RAW as Kafka (link-raw-updates)
    participant Agent as Agent Service
    participant HF as HuggingFace AI
    participant K_PREP as Kafka (link-processed-updates)

    Note over User,Bot: Subscription
    User->>Bot: /track https://github.com/owner/repo
    Bot->>Scrapper: gRPC: TrackLink(chatID, url)
    Scrapper->>DS: INSERT links, subscriptions (tx)
    Scrapper->>DS: DELETE cache
    Scrapper-->>Bot: ok
    Bot-->>User: Link tracked!

    Note over Scrapper,API: Crawl
    Scrapper->>DS: SELECT links (batch)
    Scrapper->>API: poll for changes
    API-->>Scrapper: new events

    Note over Scrapper,K_RAW: Outbox Relay
    Scrapper->>DS: INSERT outbox (tx)
    Scrapper->>DS: SELECT pending outbox
    Scrapper->>K_RAW: produce Avro message
    Scrapper->>DS: UPDATE outbox SET processed

    Note over K_RAW,Agent: Agent
    K_RAW-->>Agent: consume Avro message
    Agent->>DS: INSERT agent_inbox (tx)

    Note over Agent,HF: Filter & Summarize
    Agent->>DS: SELECT pending inbox (window)
    Agent->>Agent: filter stop-words, excluded authors
    Agent->>Agent: classify priority
    alt description > threshold
        Agent->>HF: POST summarize(description)
        HF-->>Agent: summarized text
    end
    Agent->>Agent: group by chatID in timerange
    Agent->>DS: INSERT agent_outbox (tx)
    Agent->>DS: UPDATE agent_inbox SET processed (tx)

    Note over Agent,K_PREP: Agent Outbox Relay
    Agent->>DS: SELECT pending outbox
    Agent->>K_PREP: produce Avro message
    Agent->>DS: UPDATE agent_outbox SET sent

    Note over K_PREP,Bot: Bot Consumption & Delivery (tx)
    K_PREP-->>Bot: consume Avro message
    Bot->>DS: INSERT bot_inbox (tx)
    Bot->>DS: SELECT pending inbox (tx)
    Bot->>User: send message via Telegram API
    Bot->>DS: UPDATE bot_inbox SET processed
    User-->>User: receives link updates
```

## Fault tolerance

- **Transactional Outbox/Inbox** Все события сохраняются/обновляются в рамках транзакции БД, и только потом обрабатываются  фоновой job для отправки/обработки события
- **Idempotency:** Каждое событие включает `idempotency-key` header, consumers обеспечивает идемпотентную обработку через unique constraints
- **DLQ:** Необработанные события после допустимого лимита попадают в dead-letter topics для ручного разбора
- **Circuit Breaker + Retry:** gRPC/HTTP вызовы используют exponential backoff и circuit breaker
- **Rate Limiter** gRPC/HTTP запросы к scrapper сервису использую rate-limitting interceptors/middleware
