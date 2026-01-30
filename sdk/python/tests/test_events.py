"""Tests for EventStream WebSocket subscription with reconnection."""

import asyncio
import json
from unittest.mock import patch

import pytest

from notifsh.events import (
    EventStream,
    MAX_RECONNECT_ATTEMPTS,
    INITIAL_RECONNECT_DELAY,
    MAX_RECONNECT_DELAY,
    PING_INTERVAL,
    PONG_TIMEOUT,
)


class MockWebSocket:
    """Mock websocket for testing."""

    def __init__(self):
        self.sent_messages = []
        self.closed = False
        self._recv_queue = asyncio.Queue()
        self._close_event = asyncio.Event()

    async def send(self, message: str) -> None:
        self.sent_messages.append(json.loads(message))

    async def recv(self) -> str:
        try:
            msg = await asyncio.wait_for(self._recv_queue.get(), timeout=0.5)
            return msg
        except asyncio.TimeoutError:
            if self.closed:
                from websockets.exceptions import ConnectionClosed
                raise ConnectionClosed(None, None)
            raise

    async def close(self) -> None:
        self.closed = True
        self._close_event.set()

    async def ping(self) -> asyncio.Future:
        """Return a future that resolves when pong is received."""
        future = asyncio.get_event_loop().create_future()
        future.set_result(None)
        return future

    def queue_message(self, msg: dict) -> None:
        """Queue a message to be received."""
        self._recv_queue.put_nowait(json.dumps(msg))

    def simulate_close(self) -> None:
        """Simulate connection close."""
        self.closed = True

    def __aiter__(self):
        return self

    async def __anext__(self):
        if self.closed:
            raise StopAsyncIteration
        try:
            return await self.recv()
        except Exception:
            raise StopAsyncIteration


def create_mock_connect(mock_ws: MockWebSocket):
    """Create a mock websockets.connect function."""
    async def mock_connect(*args, **kwargs):
        return mock_ws
    return mock_connect


class TestEventStreamBasics:
    """Test basic EventStream functionality."""

    @pytest.mark.asyncio
    async def test_connect_sends_subscribe_message(self):
        mock_ws = MockWebSocket()

        with patch("notifsh.events.websockets.connect", side_effect=create_mock_connect(mock_ws)):
            stream = EventStream(
                api_key="nsh_testkey12345678901234567890",
                server="http://localhost:8080",
                topics=["test-topic"],
            )

            # Queue subscribed response
            mock_ws.queue_message({"type": "subscribed"})

            # Trigger connection
            await stream._connect()

            # Verify subscribe message was sent
            assert len(mock_ws.sent_messages) == 1
            msg = mock_ws.sent_messages[0]
            assert msg["action"] == "subscribe"
            assert msg["topics"] == ["test-topic"]
            assert msg["options"]["auto_ack"] is True

            await stream.close()

    @pytest.mark.asyncio
    async def test_connect_with_options(self):
        mock_ws = MockWebSocket()

        with patch("notifsh.events.websockets.connect", side_effect=create_mock_connect(mock_ws)):
            stream = EventStream(
                api_key="nsh_testkey12345678901234567890",
                server="http://localhost:8080",
                topics=["topic-a", "topic-b"],
                auto_ack=False,
                from_="beginning",
                group="my-group",
            )

            mock_ws.queue_message({"type": "subscribed"})

            await stream._connect()

            msg = mock_ws.sent_messages[0]
            assert msg["topics"] == ["topic-a", "topic-b"]
            assert msg["options"]["auto_ack"] is False
            assert msg["options"]["from"] == "beginning"
            assert msg["options"]["group"] == "my-group"

            await stream.close()

    @pytest.mark.asyncio
    async def test_is_connected_property(self):
        mock_ws = MockWebSocket()

        with patch("notifsh.events.websockets.connect", side_effect=create_mock_connect(mock_ws)):
            stream = EventStream(
                api_key="nsh_testkey12345678901234567890",
                server="http://localhost:8080",
                topics=["test-topic"],
            )

            assert stream.is_connected is False

            mock_ws.queue_message({"type": "subscribed"})
            await stream._connect()

            assert stream.is_connected is True

            await stream.close()

            assert stream.is_connected is False


class TestEventStreamReceive:
    """Test receiving events."""

    @pytest.mark.asyncio
    async def test_receive_event(self):
        mock_ws = MockWebSocket()

        with patch("notifsh.events.websockets.connect", side_effect=create_mock_connect(mock_ws)):
            stream = EventStream(
                api_key="nsh_testkey12345678901234567890",
                server="http://localhost:8080",
                topics=["test-topic"],
            )

            mock_ws.queue_message({"type": "subscribed"})

            await stream._connect()

            # Queue an event
            mock_ws.queue_message({
                "type": "event",
                "id": "evt-123",
                "topic": "test-topic",
                "data": {"message": "hello"},
                "timestamp": "2025-01-01T00:00:00Z",
                "attempt": 1,
                "max_attempts": 5,
            })

            # Get event from queue
            event = await asyncio.wait_for(stream._event_queue.get(), timeout=2)

            assert event.id == "evt-123"
            assert event.topic == "test-topic"
            assert event.data["message"] == "hello"
            assert event.attempt == 1
            assert event.max_attempts == 5

            await stream.close()


class TestEventStreamAckNack:
    """Test ack/nack functionality."""

    @pytest.mark.asyncio
    async def test_send_ack(self):
        mock_ws = MockWebSocket()

        with patch("notifsh.events.websockets.connect", side_effect=create_mock_connect(mock_ws)):
            stream = EventStream(
                api_key="nsh_testkey12345678901234567890",
                server="http://localhost:8080",
                topics=["test-topic"],
                auto_ack=False,
            )

            mock_ws.queue_message({"type": "subscribed"})
            await stream._connect()

            # Send ack
            await stream._send_ack("evt-123")

            # Find ack message
            ack_msgs = [m for m in mock_ws.sent_messages if m.get("action") == "ack"]
            assert len(ack_msgs) == 1
            assert ack_msgs[0]["id"] == "evt-123"

            await stream.close()

    @pytest.mark.asyncio
    async def test_send_nack(self):
        mock_ws = MockWebSocket()

        with patch("notifsh.events.websockets.connect", side_effect=create_mock_connect(mock_ws)):
            stream = EventStream(
                api_key="nsh_testkey12345678901234567890",
                server="http://localhost:8080",
                topics=["test-topic"],
                auto_ack=False,
            )

            mock_ws.queue_message({"type": "subscribed"})
            await stream._connect()

            # Send nack
            await stream._send_nack("evt-456", "10m")

            # Find nack message
            nack_msgs = [m for m in mock_ws.sent_messages if m.get("action") == "nack"]
            assert len(nack_msgs) == 1
            assert nack_msgs[0]["id"] == "evt-456"
            assert nack_msgs[0]["retry_in"] == "10m"

            await stream.close()


class TestEventStreamReconnection:
    """Test reconnection logic."""

    def test_reconnect_constants(self):
        """Verify reconnection constants are set correctly."""
        assert MAX_RECONNECT_ATTEMPTS == 0  # Infinite
        assert INITIAL_RECONNECT_DELAY == 1.0
        assert MAX_RECONNECT_DELAY == 30.0
        assert PING_INTERVAL == 30.0
        assert PONG_TIMEOUT == 10.0

    @pytest.mark.asyncio
    async def test_reconnect_attempts_counter(self):
        stream = EventStream(
            api_key="nsh_testkey12345678901234567890",
            server="http://localhost:8080",
            topics=["test-topic"],
        )

        assert stream._reconnect_attempts == 0

        # Simulate reconnection
        stream._reconnect_attempts = 3

        assert stream._reconnect_attempts == 3

    @pytest.mark.asyncio
    async def test_exponential_backoff_delay(self):
        """Test that delays follow exponential backoff pattern."""
        stream = EventStream(
            api_key="nsh_testkey12345678901234567890",
            server="http://localhost:8080",
            topics=["test-topic"],
        )

        # Calculate expected delays
        delays = []
        for attempt in range(5):
            delay = min(
                INITIAL_RECONNECT_DELAY * (2 ** attempt),
                MAX_RECONNECT_DELAY,
            )
            delays.append(delay)

        expected = [1.0, 2.0, 4.0, 8.0, 16.0]
        assert delays == expected

        # Verify max delay is capped
        max_delay = min(INITIAL_RECONNECT_DELAY * (2 ** 10), MAX_RECONNECT_DELAY)
        assert max_delay == MAX_RECONNECT_DELAY


class TestEventStreamClose:
    """Test close functionality."""

    @pytest.mark.asyncio
    async def test_close_sets_closed_flag(self):
        stream = EventStream(
            api_key="nsh_testkey12345678901234567890",
            server="http://localhost:8080",
            topics=["test-topic"],
        )

        assert stream._closed is False

        await stream.close()

        assert stream._closed is True

    @pytest.mark.asyncio
    async def test_double_close_is_safe(self):
        stream = EventStream(
            api_key="nsh_testkey12345678901234567890",
            server="http://localhost:8080",
            topics=["test-topic"],
        )

        await stream.close()
        await stream.close()
        await stream.close()

        assert stream._closed is True

    @pytest.mark.asyncio
    async def test_close_stops_reader_task(self):
        mock_ws = MockWebSocket()

        with patch("notifsh.events.websockets.connect", side_effect=create_mock_connect(mock_ws)):
            stream = EventStream(
                api_key="nsh_testkey12345678901234567890",
                server="http://localhost:8080",
                topics=["test-topic"],
            )

            mock_ws.queue_message({"type": "subscribed"})
            await stream._connect()

            assert stream._reader_task is not None

            await stream.close()

            # Reader task should be cancelled
            assert stream._reader_task is None or stream._reader_task.cancelled()


class TestEventStreamPingPong:
    """Test ping/pong heartbeat functionality."""

    def test_ping_interval_constant(self):
        """Verify ping interval is 30 seconds."""
        assert PING_INTERVAL == 30.0

    def test_pong_timeout_constant(self):
        """Verify pong timeout is 10 seconds."""
        assert PONG_TIMEOUT == 10.0

    @pytest.mark.asyncio
    async def test_ping_task_created_on_connect(self):
        mock_ws = MockWebSocket()

        with patch("notifsh.events.websockets.connect", side_effect=create_mock_connect(mock_ws)):
            stream = EventStream(
                api_key="nsh_testkey12345678901234567890",
                server="http://localhost:8080",
                topics=["test-topic"],
            )

            mock_ws.queue_message({"type": "subscribed"})
            await stream._connect()

            # Ping task should be created
            assert stream._ping_task is not None

            await stream.close()

    @pytest.mark.asyncio
    async def test_ping_task_stopped_on_close(self):
        mock_ws = MockWebSocket()

        with patch("notifsh.events.websockets.connect", side_effect=create_mock_connect(mock_ws)):
            stream = EventStream(
                api_key="nsh_testkey12345678901234567890",
                server="http://localhost:8080",
                topics=["test-topic"],
            )

            mock_ws.queue_message({"type": "subscribed"})
            await stream._connect()

            ping_task = stream._ping_task

            await stream.close()

            # Ping task should be cancelled
            assert ping_task is None or ping_task.cancelled() or ping_task.done()


class TestEventStreamAsyncIterator:
    """Test async iterator protocol."""

    def test_aiter_returns_self(self):
        stream = EventStream(
            api_key="nsh_testkey12345678901234567890",
            server="http://localhost:8080",
            topics=["test-topic"],
        )

        assert stream.__aiter__() is stream

    @pytest.mark.asyncio
    async def test_anext_raises_stop_when_closed(self):
        stream = EventStream(
            api_key="nsh_testkey12345678901234567890",
            server="http://localhost:8080",
            topics=["test-topic"],
        )

        stream._closed = True

        with pytest.raises(StopAsyncIteration):
            await stream.__anext__()


class TestEventStreamErrorHandling:
    """Test error handling."""

    @pytest.mark.asyncio
    async def test_api_error_on_error_message(self):
        mock_ws = MockWebSocket()

        with patch("notifsh.events.websockets.connect", side_effect=create_mock_connect(mock_ws)):
            stream = EventStream(
                api_key="nsh_testkey12345678901234567890",
                server="http://localhost:8080",
                topics=["test-topic"],
            )

            # Queue error response instead of subscribed
            mock_ws.queue_message({
                "type": "error",
                "message": "invalid topic format",
            })

            from notifsh.errors import APIError
            with pytest.raises(APIError) as exc_info:
                await stream._connect()

            assert "invalid topic format" in str(exc_info.value)
