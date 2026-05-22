import asyncio
import json
import logging
from typing import Any

from anthropic import APITimeoutError, AsyncAnthropic, RateLimitError
from anthropic.types import MessageParam, ToolParam
from pydantic import ValidationError

from .schemas import Setup

logger = logging.getLogger(__name__)

# How long to wait before retrying a transient API error, in seconds.
_TRANSIENT_BACKOFF = 1.0


SYSTEM_PROMPT = (
    "You are a disciplined cryptocurrency trading analyst. Given recent OHLCV "
    "candles and computed indicators (EMA-12, EMA-26, RSI-14, MACD, Bollinger "
    "Bands), you produce one structured setup by calling the submit_setup tool.\n\n"
    "Rules:\n"
    "- Always call submit_setup. Never reply with prose.\n"
    "- Use 'hold' when signals are mixed, weak, or contradict each other.\n"
    "- Confidence reflects how clean the setup is, not the size of the move.\n"
    "  Clean trend with confirming indicators: 0.7-0.9. Mixed: 0.4-0.6. Weak: <0.4.\n"
    "- Rationale must be terse (under 280 characters) and reference specific\n"
    "  indicators with their numeric state. No hedging, no disclaimers.\n"
    "- If you propose entry, you must propose stop. Targets are optional but\n"
    "  must be ordered in the direction of the trade.\n"
    "- Do not invent prices. Use values from the snapshot or simple offsets.\n"
)


SUBMIT_SETUP_TOOL: ToolParam = {
    "name": "submit_setup",
    "description": "Submit a structured trading setup analysis for the supplied snapshot.",
    "input_schema": {
        "type": "object",
        "properties": {
            "direction": {
                "type": "string",
                "enum": ["buy", "sell", "hold"],
                "description": "Trade direction. Use 'hold' when signals are unclear.",
            },
            "confidence": {
                "type": "number",
                "minimum": 0,
                "maximum": 1,
                "description": "How clean the setup is, from 0 to 1.",
            },
            "entry": {
                "type": ["number", "null"],
                "description": "Suggested entry price. Required if direction is buy or sell.",
            },
            "stop": {
                "type": ["number", "null"],
                "description": "Stop-loss price. Required if entry is set.",
            },
            "targets": {
                "type": "array",
                "items": {"type": "number"},
                "description": "Take-profit levels, ordered toward the trade direction.",
            },
            "rationale": {
                "type": "string",
                "maxLength": 280,
                "description": "Short reasoning referencing actual indicator values.",
            },
        },
        "required": ["direction", "confidence", "rationale"],
    },
}


class AnalystError(Exception):
    pass


class Analyst:
    def __init__(self, client: AsyncAnthropic, model: str, max_tokens: int = 512) -> None:
        self.client = client
        self.model = model
        self.max_tokens = max_tokens

    async def analyze(self, snapshot: dict[str, Any]) -> Setup:
        messages: list[MessageParam] = [
            {"role": "user", "content": self._build_user_payload(snapshot)},
        ]

        # One retry budget for bad payloads. We feed the validation error back
        # as a tool_result so the model can correct itself, then give up.
        for attempt in range(2):
            response = await self._create(messages)

            tool_use = _find_tool_use(response.content, "submit_setup")
            if tool_use is None:
                raise AnalystError("Claude did not call submit_setup")

            try:
                return Setup.model_validate(tool_use.input)
            except ValidationError as e:
                if attempt == 1:
                    raise AnalystError(f"submit_setup payload failed validation: {e}") from e
                logger.warning("invalid setup payload, retrying with feedback: %s", e)
                messages.append({"role": "assistant", "content": response.content})
                messages.append(
                    {
                        "role": "user",
                        "content": [
                            {
                                "type": "tool_result",
                                "tool_use_id": tool_use.id,
                                "is_error": True,
                                "content": (
                                    f"Validation failed: {e}. "
                                    "Call submit_setup again with corrected fields."
                                ),
                            }
                        ],
                    }
                )

        raise AnalystError("retry budget exhausted")  # pragma: no cover

    async def _create(self, messages: list[MessageParam]) -> Any:
        # Retry once on transient API failures with a short backoff.
        for attempt in range(2):
            try:
                return await self.client.messages.create(
                    model=self.model,
                    max_tokens=self.max_tokens,
                    system=SYSTEM_PROMPT,
                    tools=[SUBMIT_SETUP_TOOL],
                    tool_choice={"type": "tool", "name": "submit_setup"},
                    messages=messages,
                )
            except (RateLimitError, APITimeoutError) as e:
                if attempt == 1:
                    raise AnalystError(f"anthropic call failed: {e}") from e
                logger.warning("transient anthropic error, retrying: %s", e)
                await asyncio.sleep(_TRANSIENT_BACKOFF)
        raise AnalystError("transient retry budget exhausted")  # pragma: no cover

    @staticmethod
    def _build_user_payload(snapshot: dict[str, Any]) -> str:
        candles = snapshot.get("candles") or []
        indicators = snapshot.get("indicators") or {}

        # Trim to the most recent slices so token usage stays bounded.
        tail_candles = candles[-30:]
        tail_indicators = _tail_indicators(indicators, n=15)

        return json.dumps(
            {
                "symbol": snapshot.get("symbol"),
                "interval": snapshot.get("interval"),
                "fetched_at": snapshot.get("fetched_at"),
                "candles_tail": tail_candles,
                "indicators_tail": tail_indicators,
            },
            default=str,
        )


def _find_tool_use(content: list[Any], tool_name: str) -> Any:
    for block in content:
        if getattr(block, "name", None) == tool_name and hasattr(block, "input"):
            return block
    return None


def _tail_indicators(indicators: dict[str, Any], n: int) -> dict[str, Any]:
    out: dict[str, Any] = {}
    for k, v in indicators.items():
        if isinstance(v, list):
            out[k] = v[-n:]
        elif isinstance(v, dict):
            out[k] = {sk: (sv[-n:] if isinstance(sv, list) else sv) for sk, sv in v.items()}
        else:
            out[k] = v
    return out
