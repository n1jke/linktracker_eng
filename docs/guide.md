# Setup & Development

## 1. Obtaining Tokens

- **Telegram:** Create a bot and get its token via [@BotFather](https://telegram.me/BotFather)
- **GitHub:** Generate a token at [github.com/settings/tokens](https://github.com/settings/tokens) -> Personal access tokens -> Token (Classic)
- **HuggingFace:** [huggingface.co/settings/tokens](https://huggingface.co/settings/tokens) -> create a token with `read` permissions

## 2. Environment Setup

Copy the example configs and set your tokens in `.env`, as well as `<username>` and `<password>` for Valkey:

```bash
cd deploy
cp .env.example .env
cp valkey.example.conf valkey.conf
cp sentinel.example.conf sentinel.conf
cp users.example.acl users.acl
```

## 3. Build & Run

```bash
cd deploy
docker compose up -d --build
```

`--build` is required on first run or after code changes.

## 4. Testing & Linting

```bash
make fmt          # go fmt
make lint         # golangci-lint
make test         # unit tests
make test-slow    # integration tests (testcontainers)
make test-all     # all tests
make html_test    # HTML test report
make clean        # remove test artifacts
```
