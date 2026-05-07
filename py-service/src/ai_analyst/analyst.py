import json
import logging
from typing import Any, Protocol

from pydantic import ValidationError

from .schemas import Setup

logger = logging.getLogger(__name__)


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


SUBMIT_SETUP_TOOL: dict[str, Any] = {
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


class _AnthropicLike(Protocol):
    messages: Any


class Analyst:
    def __init__(self, client: _AnthropicLike, model: str, max_tokens: int = 512) -> None:
        self.client = client
        self.model = model
        self.max_tokens = max_tokens

    async def analyze(self, snapshot: dict[str, Any]) -> Setup:
        user_payload = self._build_user_payload(snapshot)

        response = await self.client.messages.create(
            model=self.model,
            max_tokens=self.max_tokens,
            system=[
                {
                    "type": "text",
                    "text": SYSTEM_PROMPT,
                    "cache_control": {"type": "ephemeral"},
                }
            ],
            tools=[
                {**SUBMIT_SETUP_TOOL, "cache_control": {"type": "ephemeral"}},
            ],
            tool_choice={"type": "tool", "name": "submit_setup"},
            messages=[{"role": "user", "content": user_payload}],
        )

        tool_input = _extract_tool_input(response.content, "submit_setup")
        if tool_input is None:
            raise AnalystError("Claude did not call submit_setup")

        try:
            return Setup.model_validate(tool_input)
        except ValidationError as e:
            raise AnalystError(f"submit_setup payload failed validation: {e}") from e

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


def _extract_tool_input(content: list[Any], tool_name: str) -> dict[str, Any] | None:
    for block in content:
        if getattr(block, "name", None) == tool_name and hasattr(block, "input"):
            return block.input  # type: ignore[no-any-return]
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
