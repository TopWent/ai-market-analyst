from typing import Any
from unittest.mock import AsyncMock

import pytest

from ai_analyst.analyst import Analyst, AnalystError, SUBMIT_SETUP_TOOL
from ai_analyst.schemas import Direction


class StubBlock:
    def __init__(self, name: str | None = None, input: dict[str, Any] | None = None) -> None:
        self.name = name
        self.input = input


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
async def test_analyze_passes_force_tool_choice_and_caching():
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

    tools = kwargs["tools"]
    assert len(tools) == 1
    assert tools[0]["name"] == "submit_setup"
    assert tools[0]["cache_control"] == {"type": "ephemeral"}

    system = kwargs["system"]
    assert isinstance(system, list)
    assert system[0]["cache_control"] == {"type": "ephemeral"}
    assert "submit_setup" in system[0]["text"]


@pytest.mark.asyncio
async def test_analyze_user_payload_is_trimmed_json():
    captured: dict[str, Any] = {}

    async def fake_create(**kwargs: Any) -> StubResponse:
        captured.update(kwargs)
        return StubResponse(
            [StubBlock(name="submit_setup", input={"direction": "hold", "confidence": 0.5, "rationale": "x"})]
        )

    client = type("Client", (), {})()
    client.messages = type("Messages", (), {"create": staticmethod(fake_create)})()

    snap = sample_snapshot()
    await Analyst(client, "claude-test").analyze(snap)

    user_msg = captured["messages"][0]["content"]
    import json
    payload = json.loads(user_msg)
    assert payload["symbol"] == "BTCUSDT"
    # Tail trims to last 30 candles even though we sent 40.
    assert len(payload["candles_tail"]) == 30
    # Indicator tails trim to last 15.
    assert len(payload["indicators_tail"]["ema_12"]) == 15
    assert len(payload["indicators_tail"]["macd"]["macd"]) == 15


@pytest.mark.asyncio
async def test_analyze_raises_when_tool_not_called():
    blocks = [StubBlock(name="other_tool", input={})]
    client, _ = make_client(blocks)

    with pytest.raises(AnalystError) as exc:
        await Analyst(client, "claude-test").analyze(sample_snapshot())
    assert "submit_setup" in str(exc.value)


@pytest.mark.asyncio
async def test_analyze_raises_on_invalid_payload():
    blocks = [
        StubBlock(
            name="submit_setup",
            input={"direction": "moon", "confidence": 0.5, "rationale": "x"},
        )
    ]
    client, _ = make_client(blocks)

    with pytest.raises(AnalystError) as exc:
        await Analyst(client, "claude-test").analyze(sample_snapshot())
    assert "validation" in str(exc.value).lower()


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


def test_tool_definition_shape():
    schema = SUBMIT_SETUP_TOOL["input_schema"]
    assert schema["properties"]["direction"]["enum"] == ["buy", "sell", "hold"]
    assert "rationale" in schema["required"]
    assert "confidence" in schema["required"]
