from __future__ import annotations

import os
from datetime import datetime
from typing import TYPE_CHECKING, Any

import httpx

from .constants import API_KEY_PREFIX, DEFAULT_SERVER, DEFAULT_TIMEOUT, ENV_VAR_NAME
from .errors import APIError, AuthError
from .errors import ConnectionError as NotifConnectionError
from .events import EventStream
from .types import EmitResponse

if TYPE_CHECKING:
    from collections.abc import AsyncIterator

    from .types import Event


class Notif:
    """Async client for notif.sh API."""

    def __init__(
        self,
        api_key: str | None = None,
        *,
        server: str = DEFAULT_SERVER,
        timeout: float = DEFAULT_TIMEOUT,
    ) -> None:
        """
        Initialize the Notif client.

        Args:
            api_key: API key for authentication. If not provided, reads from
                     NOTIF_API_KEY environment variable.
            server: Server URL. Defaults to https://api.notif.sh.
            timeout: Request timeout in seconds. Defaults to 30.

        Raises:
            AuthError: If no API key is provided or key has invalid format.
        """
        resolved_key = api_key or os.environ.get(ENV_VAR_NAME)

        if not resolved_key:
            raise AuthError(
                f"API key not provided. Set {ENV_VAR_NAME} environment variable "
                "or pass api_key argument."
            )

        if not resolved_key.startswith(API_KEY_PREFIX):
            raise AuthError(f"API key must start with '{API_KEY_PREFIX}'")

        self._api_key = resolved_key
        self._server = server
        self._timeout = timeout
        self._http_client: httpx.AsyncClient | None = None
        self._active_streams: set[EventStream] = set()

    async def _get_client(self) -> httpx.AsyncClient:
        if self._http_client is None:
            self._http_client = httpx.AsyncClient(
                timeout=self._timeout,
                headers={
                    "Authorization": f"Bearer {self._api_key}",
                    "Content-Type": "application/json",
                },
            )
        return self._http_client

    async def emit(self, topic: str, data: dict[str, Any]) -> EmitResponse:
        """
        Publish an event to a topic.

        Args:
            topic: The topic to publish to.
            data: The event payload (must be JSON-serializable).

        Returns:
            EmitResponse with the event id, topic, and timestamp.

        Raises:
            AuthError: If authentication fails.
            APIError: If the API returns an error.
            ConnectionError: If there's a network error.
        """
        client = await self._get_client()

        try:
            response = await client.post(
                f"{self._server}/api/v1/emit",
                json={"topic": topic, "data": data},
            )
        except httpx.RequestError as e:
            raise NotifConnectionError(str(e), cause=e) from e

        if response.status_code == 401:
            raise AuthError()

        if response.status_code not in (200, 201):
            body = response.json() if response.content else {}
            raise APIError(response.status_code, body.get("error", "emit failed"))

        result = response.json()
        return EmitResponse(
            id=result["id"],
            topic=result["topic"],
            created_at=datetime.fromisoformat(result["created_at"].replace("Z", "+00:00")),
        )

    def subscribe(
        self,
        *topics: str,
        auto_ack: bool = True,
        from_: str | None = None,
        group: str | None = None,
    ) -> AsyncIterator[Event]:
        """
        Subscribe to events on the given topics.

        Args:
            *topics: One or more topic patterns to subscribe to (e.g., 'orders.*').
            auto_ack: If True (default), events are automatically acknowledged
                      after successful processing. If False, you must call
                      event.ack() manually.
            from_: Where to start receiving events. Can be 'latest' (default),
                   'beginning', or an ISO8601 timestamp.
            group: Consumer group name for load-balanced consumption.

        Returns:
            An async iterator that yields Event objects.

        Example:
            async for event in client.subscribe('orders.*'):
                print(event.data)

            # With manual ack:
            async for event in client.subscribe('orders.*', auto_ack=False):
                await event.ack()
        """
        if not topics:
            raise ValueError("At least one topic is required")

        stream = EventStream(
            api_key=self._api_key,
            server=self._server,
            topics=list(topics),
            auto_ack=auto_ack,
            from_=from_ or "latest",
            group=group,
        )

        self._active_streams.add(stream)
        stream._on_close = lambda: self._active_streams.discard(stream)

        return stream

    @property
    def server_url(self) -> str:
        """Get the configured server URL."""
        return self._server

    async def close(self) -> None:
        """Close the client and clean up resources."""
        for stream in list(self._active_streams):
            await stream.close()
        self._active_streams.clear()

        if self._http_client:
            await self._http_client.aclose()
            self._http_client = None

    async def __aenter__(self) -> Notif:
        return self

    async def __aexit__(self, *args: object) -> None:
        await self.close()
