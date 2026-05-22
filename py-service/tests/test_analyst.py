import json
from typing import Any
from unittest.mock import AsyncMock

import httpx
import pytest
from anthropic import APITimeoutError

from ai_analyst.analyst import SUBMIT_SETUP_TOOL, Analyst, AnalystError
from ai_analyst.schemas import Direction


class StubBlock:
    def __init__(
        self,
        name: str | None = None,
        input: dict[str, Any] | None = None,
        id: str = "tool_1",
    ) -> None:
        self.name = name
        self.input = input
        self.id = id


class StubResponse:
    def __init__(self, blocks: list[Any]) -> None:
        self.content = blocks


def make_client(blocks: list[Any]) -> tuple[Any, AsyncMock]:
    create = AsyncMock(return_value=StubResponse(blocks))
    client = type("Client", (), {})()
    client.messages = type("Messages", (), {"create": create})()
    return client, create


def sample_snapshot() -> dict[str, Any]:
    return {
        "symbol": "BTCUSDT",
        "interval": "1h",
        "candles": [{"close": float(i)} for i in range(40)],
        "indicators": {
            "ema_12": list(range(40)),
            "macd": {
                "macd": list(range(40)),
                "signal": list(range(40)),
                "histogram": list(range(40)),
            },
        },
    }


@pytest.mark.asyncio
async def test_analyze_returns_setup_from_tool_use():
    blocks = [
        StubBlock(
            name="submit_setup",
            input={"direction": "buy", "confidence": 0.7, "rationale": "ema cross"},
        )
    ]
    client, create = make_client(blocks)

    setup = await Analyst(client, "claude-test").analyze(sample_snapshot())

    assert setup.direction is Direction.BUY
    assert setup.confidence == 0.7
    create.assert_awaited_once()


@pytest.mark.asyncio
async def test_analyze_forces_tool_choice():
    blocks = [
        StubBlock(
            name="submit_setup",
            input={"direction": "hold", "confidence": 0.5, "rationale": "mixed"},
        )
    ]
    client, create = make_client(blocks)

    await Analyst(client, "claude-test").analyze(sample_snapshot())

    kwargs = create.call_args.kwargs
    assert kwargs["model"] == "claude-test"
    assert kwargs["tool_choice"] == {"type": "tool", "name": "submit_setup"}
    assert isinstance(kwargs["system"], str)

    tools = kwargs["tools"]
    assert len(tools) == 1
    assert tools[0]["name"] == "submit_setup"


@pytest.mark.asyncio
async def test_analyze_user_payload_is_trimmed_json():
    captured: dict[str, Any] = {}

    async def fake_create(**kwargs: Any) -> StubResponse:
        captured.update(kwargs)
        return StubResponse(
            [
                StubBlock(
                    name="submit_setup",
                    input={"direction": "hold", "confidence": 0.5, "rationale": "x"},
                )
            ]
        )

    client = type("Client", (), {})()
    client.messages = type("Messages", (), {"create": staticmethod(fake_create)})()

    await Analyst(client, "claude-test").analyze(sample_snapshot())

    user_msg = captured["messages"][0]["content"]
    payload = json.loads(user_msg)
    assert payload["symbol"] == "BTCUSDT"
    assert len(payload["candles_tail"]) == 30  # last 30 of 40
    assert len(payload["indicators_tail"]["ema_12"]) == 15  # last 15
    assert len(payload["indicators_tail"]["macd"]["macd"]) == 15


@pytest.mark.asyncio
async def test_analyze_raises_when_tool_not_called():
    blocks = [StubBlock(name="other_tool", input={})]
    client, _ = make_client(blocks)

    with pytest.raises(AnalystError) as exc:
        await Analyst(client, "claude-test").analyze(sample_snapshot())
    assert "submit_setup" in str(exc.value)


def block(direction: str, confidence: float = 0.5, rationale: str = "x") -> StubBlock:
    return StubBlock(
        name="submit_setup",
        input={"direction": direction, "confidence": confidence, "rationale": rationale},
    )


@pytest.mark.asyncio
async def test_analyze_retries_invalid_payload_then_succeeds():
    bad = StubResponse([block("moon")])
    good = StubResponse([block("buy", 0.6, "ok")])
    create = AsyncMock(side_effect=[bad, good])
    client = type("Client", (), {})()
    client.messages = type("Messages", (), {"create": create})()

    setup = await Analyst(client, "claude-test").analyze(sample_snapshot())

    assert setup.direction is Direction.BUY
    assert create.await_count == 2
    # The retry must feed the validation error back as a tool_result.
    retry_messages = create.await_args_list[1].kwargs["messages"]
    assert retry_messages[-1]["content"][0]["type"] == "tool_result"
    assert retry_messages[-1]["content"][0]["is_error"] is True


@pytest.mark.asyncio
async def test_analyze_raises_after_two_invalid_payloads():
    create = AsyncMock(side_effect=[StubResponse([block("moon")]), StubResponse([block("moon")])])
    client = type("Client", (), {})()
    client.messages = type("Messages", (), {"create": create})()

    with pytest.raises(AnalystError) as exc:
        await Analyst(client, "claude-test").analyze(sample_snapshot())
    assert "validation" in str(exc.value).lower()
    assert create.await_count == 2


@pytest.mark.asyncio
async def test_analyze_raises_on_out_of_range_confidence():
    blocks = [
        StubBlock(
            name="submit_setup",
            input={"direction": "buy", "confidence": 1.7, "rationale": "x"},
        )
    ]
    client, _ = make_client(blocks)

    with pytest.raises(AnalystError):
        await Analyst(client, "claude-test").analyze(sample_snapshot())


@pytest.mark.asyncio
async def test_analyze_retries_transient_error(monkeypatch: pytest.MonkeyPatch) -> None:
    # Don't actually sleep during the backoff.
    async def no_sleep(_: float) -> None:
        return None

    monkeypatch.setattr("ai_analyst.analyst.asyncio.sleep", no_sleep)

    timeout = APITimeoutError(request=httpx.Request("POST", "https://api.anthropic.com"))
    good = StubResponse([block("hold", 0.5, "ok")])
    create = AsyncMock(side_effect=[timeout, good])
    client = type("Client", (), {})()
    client.messages = type("Messages", (), {"create": create})()

    setup = await Analyst(client, "claude-test").analyze(sample_snapshot())

    assert setup.direction is Direction.HOLD
    assert create.await_count == 2


@pytest.mark.asyncio
async def test_analyze_raises_on_persistent_transient_error(
    monkeypatch: pytest.MonkeyPatch,
) -> None:
    async def no_sleep(_: float) -> None:
        return None

    monkeypatch.setattr("ai_analyst.analyst.asyncio.sleep", no_sleep)

    timeout = APITimeoutError(request=httpx.Request("POST", "https://api.anthropic.com"))
    create = AsyncMock(side_effect=[timeout, timeout])
    client = type("Client", (), {})()
    client.messages = type("Messages", (), {"create": create})()

    with pytest.raises(AnalystError):
        await Analyst(client, "claude-test").analyze(sample_snapshot())
    assert create.await_count == 2


def test_tool_definition_shape():
    schema = SUBMIT_SETUP_TOOL["input_schema"]
    assert schema["properties"]["direction"]["enum"] == ["buy", "sell", "hold"]
    assert "rationale" in schema["required"]
    assert "confidence" in schema["required"]
