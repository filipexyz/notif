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


# Schedule types


@dataclass
class CreateScheduleResponse:
    """Response from creating a scheduled event."""

    id: str
    topic: str
    scheduled_for: datetime
    created_at: datetime


@dataclass
class Schedule:
    """A scheduled event."""

    id: str
    topic: str
    data: dict[str, Any]
    scheduled_for: datetime
    status: str  # pending, completed, cancelled, failed
    created_at: datetime
    error: str | None = None
    executed_at: datetime | None = None


@dataclass
class ListSchedulesResponse:
    """Response from listing scheduled events."""

    schedules: list[Schedule]
    total: int


@dataclass
class RunScheduleResponse:
    """Response from running a scheduled event immediately."""

    schedule_id: str
    event_id: str
