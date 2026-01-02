from dataclasses import dataclass, field
from datetime import datetime
from typing import Any, Callable, Coroutine


@dataclass
class EmitResponse:
    """Response from emitting an event."""

    id: str
    topic: str
    created_at: datetime


@dataclass
class Event:
    """Event received from a subscription."""

    id: str
    topic: str
    data: dict[str, Any]
    timestamp: datetime
    attempt: int
    max_attempts: int
    _ack_fn: Callable[[], Coroutine[Any, Any, None]] = field(repr=False)
    _nack_fn: Callable[[str | None], Coroutine[Any, Any, None]] = field(repr=False)

    async def ack(self) -> None:
        """Acknowledge this event (only works when auto_ack=False)."""
        await self._ack_fn()

    async def nack(self, retry_in: str | None = None) -> None:
        """Negative acknowledge this event, requesting redelivery (only works when auto_ack=False)."""
        await self._nack_fn(retry_in)
