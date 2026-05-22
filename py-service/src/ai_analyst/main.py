import logging
import os
import sys
from collections.abc import AsyncIterator
from contextlib import asynccontextmanager
from datetime import UTC, datetime
from typing import cast

from anthropic import AsyncAnthropic
from fastapi import FastAPI, HTTPException, Request, Response
from slowapi import Limiter, _rate_limit_exceeded_handler
from slowapi.errors import RateLimitExceeded
from slowapi.util import get_remote_address

from .analyst import Analyst, AnalystError
from .client import MarketDataClient, MarketDataError
from .schemas import AnalyzeRequest, AnalyzeResponse


def _configure_logging(level: str) -> None:
    numeric = getattr(logging, level.upper(), logging.INFO)
    logging.basicConfig(
        level=numeric,
        format='{"ts":"%(asctime)s","lvl":"%(levelname)s","name":"%(name)s","msg":"%(message)s"}',
    )


def build_app() -> FastAPI:
    api_key = os.environ.get("ANTHROPIC_API_KEY")
    if not api_key:
        print("ANTHROPIC_API_KEY is required", file=sys.stderr)
        sys.exit(2)

    model = os.environ.get("ANTHROPIC_MODEL", "claude-sonnet-4-5")
    market_data_url = os.environ.get("MARKET_DATA_URL", "http://localhost:8080")
    rate_limit = os.environ.get("AI_ANALYZE_RATE_LIMIT", "10/minute")

    _configure_logging(os.environ.get("AI_LOG_LEVEL", "info"))
    log = logging.getLogger("ai_analyst")

    market = MarketDataClient(market_data_url)
    anthropic = AsyncAnthropic(api_key=api_key)
    analyst = Analyst(anthropic, model)

    limiter = Limiter(key_func=get_remote_address, default_limits=[])

    @asynccontextmanager
    async def lifespan(_: FastAPI) -> AsyncIterator[None]:
        log.info("starting: model=%s upstream=%s rate_limit=%s", model, market_data_url, rate_limit)
        try:
            yield
        finally:
            await market.close()
            log.info("shutdown complete")

    app = FastAPI(title="ai-analyst", version="0.1.0", lifespan=lifespan)
    app.state.limiter = limiter

    def rate_limit_handler(request: Request, exc: Exception) -> Response:
        # slowapi only ever passes RateLimitExceeded here; the Exception
        # signature is what Starlette's handler registry expects.
        return _rate_limit_exceeded_handler(request, cast(RateLimitExceeded, exc))

    app.add_exception_handler(RateLimitExceeded, rate_limit_handler)

    @app.get("/health")
    async def health() -> dict[str, str]:
        return {"status": "ok"}

    @app.post("/analyze", response_model=AnalyzeResponse)
    @limiter.limit(rate_limit)
    async def analyze(request: Request, req: AnalyzeRequest) -> AnalyzeResponse:
        symbol = req.symbol.upper()
        log.info("analyze start: symbol=%s interval=%s", symbol, req.interval)

        try:
            snapshot = await market.snapshot(symbol, req.interval, req.limit)
        except MarketDataError as e:
            log.warning("market-data failed: %s", e)
            raise HTTPException(status_code=502, detail=f"market-data: {e}") from e

        try:
            setup = await analyst.analyze(snapshot)
        except AnalystError as e:
            log.warning("analyst failed: %s", e)
            raise HTTPException(status_code=500, detail=f"analyst: {e}") from e

        log.info(
            "analyze done: symbol=%s direction=%s confidence=%.2f",
            symbol, setup.direction.value, setup.confidence,
        )
        return AnalyzeResponse(
            symbol=symbol,
            interval=req.interval,
            setup=setup,
            fetched_at=datetime.now(UTC),
        )

    return app


def run() -> None:
    import uvicorn

    addr = os.environ.get("AI_HTTP_ADDR", ":8081")
    host, _, port_str = addr.rpartition(":")
    if not host:
        host = "0.0.0.0"
    uvicorn.run(build_app(), host=host, port=int(port_str))


if __name__ == "__main__":
    run()
