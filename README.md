# AI Market Analyst

[![Go Version](https://img.shields.io/badge/go-1.24+-blue.svg)](https://go.dev/dl/)
[![Python Version](https://img.shields.io/badge/python-3.11+-blue.svg)](https://www.python.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

Two services. A Go service ingests crypto OHLCV and computes indicators; a Python service feeds that to Claude via tool-calling and gets back a structured trading setup.

## What it does

1. The Go `market-data` service polls Binance for OHLCV candles and computes EMA, RSI, MACD, and Bollinger Bands. Serves them over a small HTTP API.
2. The Python `ai-analyst` service pulls a snapshot from `market-data`, sends it to Claude with a `submit_setup` tool definition, and returns a typed JSON signal: direction, confidence, entry, stop, targets, and a short rationale.
3. A demo CLI runs the whole thing against a symbol/interval pair and prints the signal.

## Architecture

The Python analyst calls the Go service over HTTP to get a market snapshot, then calls the Anthropic API to turn that snapshot into a signal. The two services are independent: the Go service runs standalone for indicator computation, and the analyst will point at any HTTP endpoint that returns the same payload shape.

## Quick start

Requires Docker and an Anthropic API key.

```bash
git clone https://github.com/TopWent/ai-market-analyst.git
cd ai-market-analyst
cp .env.example .env
# edit .env: set ANTHROPIC_API_KEY=sk-ant-...

docker compose up --build
```

In another terminal:

```bash
./demo/run.sh BTCUSDT 1h
```

You should see a JSON signal with direction, confidence, entry/stop/targets, and a short rationale.

## Components

| Path | Stack | Responsibility |
|---|---|---|
| `go-service/` | Go 1.24, net/http | OHLCV fetch, indicator math, REST API |
| `py-service/` | Python 3.11, anthropic, pydantic, fastapi | Claude tool-calling, structured signal |
| `demo/` | bash | End-to-end smoke pipeline |

## Configuration

| Env var | Default | Used by | Description |
|---|---|---|---|
| `EXCHANGE_BASE_URL` | `https://api.binance.com` | go-service | REST base for OHLCV |
| `MD_HTTP_ADDR` | `:8080` | go-service | listen address |
| `MD_CACHE_TTL` | `30s` | go-service | OHLCV cache TTL (0 disables) |
| `MD_LOG_LEVEL` | `info` | go-service | structured log level |
| `MARKET_DATA_URL` | `http://market-data:8080` | py-service | upstream for snapshots |
| `ANTHROPIC_API_KEY` | required | py-service | Claude API key |
| `ANTHROPIC_MODEL` | `claude-sonnet-4-5` | py-service | model id |
| `AI_HTTP_ADDR` | `:8081` | py-service | listen address |
| `AI_ANALYZE_RATE_LIMIT` | `10/minute` | py-service | per-IP rate limit on `/analyze` |
| `AI_LOG_LEVEL` | `info` | py-service | log level |

## Indicators

Implemented in `go-service/internal/indicators` with table-driven tests:

- EMA (configurable period)
- RSI (Wilder smoothing, 14-period default)
- MACD (12, 26, 9)
- Bollinger Bands (20, 2 stddev)

All return aligned slices indexed by candle timestamp, with NaN-padding for the warmup window.

## Claude tool definition

The analyst forces one tool call: `submit_setup`, with a strict schema (direction enum, confidence 0-1, optional levels, max 280-char rationale). The tool input is validated with pydantic. If validation fails, the analyst retries once and feeds the error back to Claude so it can correct the payload. Transient API errors (rate limit, timeout) get one short-backoff retry.

## Development

Root Makefile fans out to both services.

```bash
make test         # go test + pytest
make lint         # golangci-lint (go) + ruff and mypy (python)
make build        # build the go binary, install the python package
make docker       # build both images
```

The Go service needs golangci-lint on PATH for `make lint`; the Python service expects the dev extras installed (`make -C py-service install`).

## License

MIT. See [LICENSE](LICENSE).

## Author

[@TopWent](https://github.com/TopWent). Backend AI Engineer.
