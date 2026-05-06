import httpx
import pytest
import respx

from ai_analyst.client import MarketDataClient, MarketDataError


@pytest.fixture
async def client():
    c = MarketDataClient("http://market-data:8080")
    try:
        yield c
    finally:
        await c.close()


@pytest.mark.asyncio
async def test_snapshot_passes_params(client):
    payload = {"symbol": "BTCUSDT", "interval": "1h", "candles": [], "indicators": {}}

    with respx.mock(base_url="http://market-data:8080") as mock:
        route = mock.get("/snapshot").mock(return_value=httpx.Response(200, json=payload))

        got = await client.snapshot("BTCUSDT", "1h", limit=200)

        assert got == payload
        assert route.called
        req = route.calls.last.request
        assert req.url.params["symbol"] == "BTCUSDT"
        assert req.url.params["interval"] == "1h"
        assert req.url.params["limit"] == "200"


@pytest.mark.asyncio
async def test_snapshot_non_200_raises(client):
    with respx.mock(base_url="http://market-data:8080") as mock:
        mock.get("/snapshot").mock(return_value=httpx.Response(502, text="upstream gone"))

        with pytest.raises(MarketDataError) as exc:
            await client.snapshot("BTCUSDT", "1h")
        assert "502" in str(exc.value)


@pytest.mark.asyncio
async def test_snapshot_invalid_json_raises(client):
    with respx.mock(base_url="http://market-data:8080") as mock:
        mock.get("/snapshot").mock(return_value=httpx.Response(200, text="not json"))

        with pytest.raises(MarketDataError):
            await client.snapshot("BTCUSDT", "1h")


@pytest.mark.asyncio
async def test_snapshot_network_error_raises(client):
    with respx.mock(base_url="http://market-data:8080") as mock:
        mock.get("/snapshot").mock(side_effect=httpx.ConnectError("refused"))

        with pytest.raises(MarketDataError):
            await client.snapshot("BTCUSDT", "1h")


@pytest.mark.asyncio
async def test_close_is_idempotent():
    c = MarketDataClient("http://market-data:8080")
    await c.close()
    await c.close()


def test_base_url_trailing_slash_stripped():
    c = MarketDataClient("http://x/")
    assert c.base_url == "http://x"
