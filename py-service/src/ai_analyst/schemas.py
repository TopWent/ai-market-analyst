from datetime import datetime
from enum import StrEnum

from pydantic import BaseModel, ConfigDict, Field


class Direction(StrEnum):
    BUY = "buy"
    SELL = "sell"
    HOLD = "hold"


class Setup(BaseModel):
    model_config = ConfigDict(extra="forbid")

    direction: Direction
    confidence: float = Field(ge=0, le=1)
    entry: float | None = None
    stop: float | None = None
    targets: list[float] = Field(default_factory=list)
    rationale: str = Field(max_length=280, min_length=1)


class AnalyzeRequest(BaseModel):
    symbol: str = Field(min_length=1, max_length=20)
    interval: str = Field(default="1h", min_length=1, max_length=5)
    limit: int = Field(default=100, ge=20, le=1000)


class AnalyzeResponse(BaseModel):
    symbol: str
    interval: str
    setup: Setup
    fetched_at: datetime
