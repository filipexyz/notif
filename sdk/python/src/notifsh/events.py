from __future__ import annotations

import asyncio
import json
from datetime import datetime
from typing import Any, Callable

import websockets
from websockets.exceptions import ConnectionClosed

from .errors import APIError
from .errors import ConnectionError as NotifConnectionError
from .types import Event

MAX_RECONNECT_ATTEMPTS = 0  # 0 = infinite
INITIAL_RECONNECT_DELAY = 1.0
MAX_RECONNECT_DELAY = 30.0
PING_INTERVAL = 30.0  # 30 seconds
PONG_TIMEOUT = 10.0  # 10 seconds to receive pong


class EventStream:
    """WebSocket event stream that implements async iterator protocol."""

    def __init__(
        self,
        api_key: str,
        server: str,
        topics: list[str],
        auto_ack: bool = True,
        from_: str = "latest",
        group: str | None = None,
    ) -> None:
        self._api_key = api_key
        self._ws_url = server.replace("http://", "ws://").replace("https://", "wss://")
        self._topics = topics
        self._auto_ack = auto_ack
        self._from = from_
        self._group = group

        self._ws: websockets.WebSocketClientProtocol | None = None
        self._event_queue: asyncio.Queue[Event] = asyncio.Queue()
        self._closed = False
        self._connected = False
        self._reconnect_attempts = 0
        self._reader_task: asyncio.Task[None] | None = None
        self._ping_task: asyncio.Task[None] | None = None
        self._on_close: Callable[[], None] | None = None

    async def _connect(self) -> None:
        if self._closed:
            return

        try:
            self._ws = await websockets.connect(
                f"{self._ws_url}/ws",
                additional_headers={"Authorization": f"Bearer {self._api_key}"},
            )
        except Exception as e:
            raise NotifConnectionError(str(e), cause=e) from e

        # Send subscribe message
        await self._ws.send(
            json.dumps(
                {
                    "action": "subscribe",
                    "topics": self._topics,
                    "options": {
                        "auto_ack": self._auto_ack,
                        "from": self._from,
                        "group": self._group,
                    },
                }
            )
        )

        # Wait for subscribed confirmation
        response = json.loads(await self._ws.recv())
        if response.get("type") == "error":
            raise APIError(0, response.get("message", "subscription failed"))

        self._connected = True
        self._reconnect_attempts = 0

        # Start background tasks
        self._reader_task = asyncio.create_task(self._read_messages())
        self._ping_task = asyncio.create_task(self._ping_loop())

    async def _read_messages(self) -> None:
        try:
            async for message in self._ws:  # type: ignore[union-attr]
                data = json.loads(message)

                if data.get("type") == "event":
                    event = self._create_event(data)
                    await self._event_queue.put(event)

                elif data.get("type") == "error":
                    # Log error but don't stop iteration
                    pass

        except ConnectionClosed:
            self._connected = False
            if not self._closed:
                await self._attempt_reconnect()
        except Exception:
            self._connected = False

    def _create_event(self, data: dict[str, Any]) -> Event:
        event_id = data["id"]
        auto_ack = self._auto_ack

        async def ack() -> None:
            if auto_ack:
                return  # Server already acked
            await self._send_ack(event_id)

        async def nack(retry_in: str | None = None) -> None:
            if auto_ack:
                return  # Server already acked, can't nack
            await self._send_nack(event_id, retry_in)

        return Event(
            id=event_id,
            topic=data["topic"],
            data=data.get("data", {}),
            timestamp=datetime.fromisoformat(data["timestamp"].replace("Z", "+00:00")),
            attempt=data.get("attempt", 1),
            max_attempts=data.get("max_attempts", 5),
            _ack_fn=ack,
            _nack_fn=nack,
        )

    async def _send_ack(self, event_id: str) -> None:
        if self._ws and self._connected:
            await self._ws.send(json.dumps({"action": "ack", "id": event_id}))

    async def _send_nack(self, event_id: str, retry_in: str | None = None) -> None:
        if self._ws and self._connected:
            await self._ws.send(
                json.dumps(
                    {
                        "action": "nack",
                        "id": event_id,
                        "retry_in": retry_in or "5m",
                    }
                )
            )

    async def _ping_loop(self) -> None:
        """Send periodic pings to detect dead connections."""
        try:
            while self._connected and self._ws and not self._closed:
                await asyncio.sleep(PING_INTERVAL)
                if self._ws and self._connected:
                    try:
                        pong = await self._ws.ping()
                        await asyncio.wait_for(pong, timeout=PONG_TIMEOUT)
                    except asyncio.TimeoutError:
                        # Pong timeout, connection is dead
                        if self._ws:
                            await self._ws.close()
                        break
                    except Exception:
                        break
        except asyncio.CancelledError:
            pass

    async def _stop_tasks(self) -> None:
        """Stop background tasks."""
        if self._ping_task:
            self._ping_task.cancel()
            try:
                await self._ping_task
            except asyncio.CancelledError:
                pass
            self._ping_task = None

    async def _attempt_reconnect(self) -> None:
        await self._stop_tasks()

        if MAX_RECONNECT_ATTEMPTS > 0 and self._reconnect_attempts >= MAX_RECONNECT_ATTEMPTS:
            self._closed = True
            return

        delay = min(
            INITIAL_RECONNECT_DELAY * (2**self._reconnect_attempts),
            MAX_RECONNECT_DELAY,
        )
        self._reconnect_attempts += 1

        await asyncio.sleep(delay)

        if not self._closed:
            try:
                await self._connect()
            except Exception:
                await self._attempt_reconnect()

    def __aiter__(self) -> EventStream:
        return self

    async def __anext__(self) -> Event:
        # Connect on first iteration
        if not self._connected and not self._closed:
            await self._connect()

        while True:
            if self._closed and self._event_queue.empty():
                raise StopAsyncIteration

            try:
                event = await asyncio.wait_for(self._event_queue.get(), timeout=1.0)
                return event

            except asyncio.TimeoutError:
                if self._closed:
                    raise StopAsyncIteration
                continue

    async def close(self) -> None:
        """Close the event stream."""
        if self._closed:
            return

        self._closed = True

        await self._stop_tasks()

        if self._reader_task:
            self._reader_task.cancel()
            try:
                await self._reader_task
            except asyncio.CancelledError:
                pass

        if self._ws:
            await self._ws.close()

        self._connected = False

        if self._on_close:
            self._on_close()

    @property
    def is_connected(self) -> bool:
        """Check if the stream is connected."""
        return self._connected
