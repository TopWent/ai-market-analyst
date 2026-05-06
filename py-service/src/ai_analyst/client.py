from typing import Any

import httpx


class MarketDataError(Exception):
    pass


class MarketDataClient:
    """Thin wrapper around the market-data REST API.

    Holds a single httpx.AsyncClient for the lifetime of the instance so HTTP
    connections are pooled across requests. Call close() on shutdown.
    """

    def __init__(self, base_url: str, timeout: float = 10.0) -> None:
        self.base_url = base_url.rstrip("/")
        self._client = httpx.AsyncClient(
            base_url=self.base_url,
            timeout=timeout,
            limits=httpx.Limits(max_connections=20, max_keepalive_connections=10),
        )

    async def close(self) -> None:
        await self._client.aclose()

    async def snapshot(self, symbol: str, interval: str, limit: int = 100) -> dict[str, Any]:
        try:
            resp = await self._client.get(
                "/snapshot",
                params={"symbol": symbol, "interval": interval, "limit": limit},
            )
        except httpx.HTTPError as e:
            raise MarketDataError(f"request failed: {e}") from e

        if resp.status_code != 200:
            raise MarketDataError(f"status {resp.status_code}: {resp.text[:256]}")

        try:
            return resp.json()
        except ValueError as e:
            raise MarketDataError(f"invalid json: {e}") from e
