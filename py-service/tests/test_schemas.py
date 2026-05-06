import pytest
from pydantic import ValidationError

from ai_analyst.schemas import AnalyzeRequest, Direction, Setup


def test_setup_valid_minimum():
    s = Setup(direction=Direction.HOLD, confidence=0.5, rationale="mixed signals")
    assert s.direction is Direction.HOLD
    assert s.targets == []


def test_setup_valid_full():
    s = Setup(
        direction=Direction.BUY,
        confidence=0.72,
        entry=100.0,
        stop=98.5,
        targets=[101.0, 102.5],
        rationale="ema cross with rsi room",
    )
    assert s.entry == 100.0
    assert len(s.targets) == 2


def test_setup_rejects_out_of_range_confidence():
    with pytest.raises(ValidationError):
        Setup(direction=Direction.BUY, confidence=1.5, rationale="x")
    with pytest.raises(ValidationError):
        Setup(direction=Direction.BUY, confidence=-0.1, rationale="x")


def test_setup_rejects_long_rationale():
    with pytest.raises(ValidationError):
        Setup(direction=Direction.HOLD, confidence=0.5, rationale="x" * 281)


def test_setup_rejects_empty_rationale():
    with pytest.raises(ValidationError):
        Setup(direction=Direction.HOLD, confidence=0.5, rationale="")


def test_setup_rejects_unknown_direction():
    with pytest.raises(ValidationError):
        Setup(direction="moon", confidence=0.5, rationale="x")  # type: ignore[arg-type]


def test_setup_rejects_extra_fields():
    with pytest.raises(ValidationError):
        Setup(  # type: ignore[call-arg]
            direction=Direction.BUY,
            confidence=0.5,
            rationale="x",
            extra_field="boom",
        )


def test_request_defaults():
    r = AnalyzeRequest(symbol="BTCUSDT")
    assert r.interval == "1h"
    assert r.limit == 100


def test_request_rejects_empty_symbol():
    with pytest.raises(ValidationError):
        AnalyzeRequest(symbol="")


def test_request_rejects_bad_limit():
    with pytest.raises(ValidationError):
        AnalyzeRequest(symbol="BTCUSDT", limit=10)
    with pytest.raises(ValidationError):
        AnalyzeRequest(symbol="BTCUSDT", limit=10000)
