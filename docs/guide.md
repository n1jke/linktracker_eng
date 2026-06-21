# Запуск и разработка

## 1. Получение токенов

- **Telegram:** Создать бота и получить его тоекен через [@BotFather](https://telegram.me/BotFather)
- **GitHub:** Получить токен в [github.com/settings/tokens](https://github.com/settings/tokens) -> Personal access tokens -> Token(Classic)
- **HuggingFace:** [huggingface.co/settings/tokens](https://huggingface.co/settings/tokens) -> создать токен с `read` правами

## 2. Настройка окружения

Скопировать примеры конфигов и установить свои токены в .env а также `<username>` и `<password>` для Valkey:

```bash
cd deploy
cp .env.example .env                    
cp valkey.example.conf valkey.conf
cp sentinel.example.conf sentinel.conf
cp users.example.acl users.acl
```

## 3. Сборка и запуск

```bash
cd deploy
docker compose up -d --build
```

`--build` нужен при первой сборке или после изменений в коде.

## 4. Тестирование и линтинг

```bash
make fmt          # go fmt
make lint         # golangci-lint
make test         # unit-тесты
make test-slow    # интеграционные тесты(testcontainers)
make test-all     # все тесты
make html_test    # html отчет тестов
make clean        # удалить артефакты тестов
```
