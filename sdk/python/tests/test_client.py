import os
from unittest.mock import AsyncMock, MagicMock, patch

import pytest

from notifsh import APIError, AuthError, Notif
from notifsh.errors import ConnectionError as NotifConnectionError


class TestNotifConstructor:
    """Test Notif client constructor."""

    def test_raises_auth_error_when_no_api_key(self):
        with pytest.raises(AuthError, match="API key not provided"):
            Notif()

    def test_raises_auth_error_when_invalid_prefix(self):
        with pytest.raises(AuthError, match="must start with 'nsh_'"):
            Notif(api_key="invalid_key")

    def test_accepts_valid_api_key(self):
        client = Notif(api_key="nsh_testkey12345678901234567890")
        assert client is not None

    def test_reads_api_key_from_env_var(self):
        os.environ["NOTIF_API_KEY"] = "nsh_envkey12345678901234567890"
        client = Notif()
        assert client is not None

    def test_prefers_argument_over_env_var(self):
        os.environ["NOTIF_API_KEY"] = "nsh_envkey12345678901234567890"
        client = Notif(api_key="nsh_argkey12345678901234567890")
        assert client.server_url == "https://api.notif.sh"

    def test_uses_default_server(self):
        client = Notif(api_key="nsh_testkey12345678901234567890")
        assert client.server_url == "https://api.notif.sh"

    def test_uses_custom_server(self):
        client = Notif(
            api_key="nsh_testkey12345678901234567890",
            server="http://localhost:8080",
        )
        assert client.server_url == "http://localhost:8080"


class TestNotifEmit:
    """Test Notif.emit() method."""

    @pytest.fixture
    def client(self):
        return Notif(api_key="nsh_testkey12345678901234567890")

    @pytest.mark.asyncio
    async def test_emit_success(self, client):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.content = b'{"id": "evt_123", "topic": "test.topic", "created_at": "2025-01-01T00:00:00Z"}'
        mock_response.json.return_value = {
            "id": "evt_123",
            "topic": "test.topic",
            "created_at": "2025-01-01T00:00:00Z",
        }

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.post = AsyncMock(return_value=mock_response)
            mock_get_client.return_value = mock_http

            result = await client.emit("test.topic", {"foo": "bar"})

            assert result.id == "evt_123"
            assert result.topic == "test.topic"
            mock_http.post.assert_called_once()

    @pytest.mark.asyncio
    async def test_emit_auth_error(self, client):
        mock_response = MagicMock()
        mock_response.status_code = 401

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.post = AsyncMock(return_value=mock_response)
            mock_get_client.return_value = mock_http

            with pytest.raises(AuthError):
                await client.emit("test", {})

    @pytest.mark.asyncio
    async def test_emit_api_error(self, client):
        mock_response = MagicMock()
        mock_response.status_code = 400
        mock_response.content = b'{"error": "topic is required"}'
        mock_response.json.return_value = {"error": "topic is required"}

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.post = AsyncMock(return_value=mock_response)
            mock_get_client.return_value = mock_http

            with pytest.raises(APIError) as exc_info:
                await client.emit("", {})

            assert exc_info.value.status_code == 400
            assert "topic is required" in str(exc_info.value)

    @pytest.mark.asyncio
    async def test_emit_connection_error(self, client):
        import httpx

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.post = AsyncMock(side_effect=httpx.RequestError("Network error"))
            mock_get_client.return_value = mock_http

            with pytest.raises(NotifConnectionError):
                await client.emit("test", {})


class TestNotifSubscribe:
    """Test Notif.subscribe() method."""

    def test_raises_error_when_no_topics(self):
        client = Notif(api_key="nsh_testkey12345678901234567890")
        with pytest.raises(ValueError, match="At least one topic is required"):
            client.subscribe()

    def test_accepts_single_topic(self):
        client = Notif(api_key="nsh_testkey12345678901234567890")
        stream = client.subscribe("test.*")
        assert stream is not None

    def test_accepts_multiple_topics(self):
        client = Notif(api_key="nsh_testkey12345678901234567890")
        stream = client.subscribe("orders.*", "leads.*")
        assert stream is not None

    def test_accepts_options(self):
        client = Notif(api_key="nsh_testkey12345678901234567890")
        stream = client.subscribe("test.*", auto_ack=False, from_="beginning")
        assert stream is not None


class TestNotifContextManager:
    """Test Notif as async context manager."""

    @pytest.mark.asyncio
    async def test_context_manager(self):
        async with Notif(api_key="nsh_testkey12345678901234567890") as client:
            assert client is not None
            assert client.server_url == "https://api.notif.sh"


class TestNotifSchedule:
    """Test Notif.schedule() method."""

    @pytest.fixture
    def client(self):
        return Notif(api_key="nsh_testkey12345678901234567890")

    @pytest.mark.asyncio
    async def test_schedule_with_in(self, client):
        mock_response = MagicMock()
        mock_response.status_code = 201
        mock_response.content = b'{"id": "sch_123", "topic": "test.topic", "scheduled_for": "2025-01-01T00:30:00Z", "created_at": "2025-01-01T00:00:00Z"}'
        mock_response.json.return_value = {
            "id": "sch_123",
            "topic": "test.topic",
            "scheduled_for": "2025-01-01T00:30:00Z",
            "created_at": "2025-01-01T00:00:00Z",
        }

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.post = AsyncMock(return_value=mock_response)
            mock_get_client.return_value = mock_http

            result = await client.schedule("test.topic", {"foo": "bar"}, in_="30m")

            assert result.id == "sch_123"
            assert result.topic == "test.topic"
            mock_http.post.assert_called_once()

    @pytest.mark.asyncio
    async def test_schedule_auth_error(self, client):
        mock_response = MagicMock()
        mock_response.status_code = 401

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.post = AsyncMock(return_value=mock_response)
            mock_get_client.return_value = mock_http

            with pytest.raises(AuthError):
                await client.schedule("test", {}, in_="5m")


class TestNotifListSchedules:
    """Test Notif.list_schedules() method."""

    @pytest.fixture
    def client(self):
        return Notif(api_key="nsh_testkey12345678901234567890")

    @pytest.mark.asyncio
    async def test_list_schedules_success(self, client):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.content = b'{"schedules": [], "total": 0}'
        mock_response.json.return_value = {
            "schedules": [
                {
                    "id": "sch_123",
                    "topic": "test.topic",
                    "data": {"foo": "bar"},
                    "scheduled_for": "2025-01-15T10:00:00Z",
                    "status": "pending",
                    "created_at": "2025-01-01T00:00:00Z",
                }
            ],
            "total": 1,
        }

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.get = AsyncMock(return_value=mock_response)
            mock_get_client.return_value = mock_http

            result = await client.list_schedules(status="pending", limit=10, offset=0)

            assert result.total == 1
            assert len(result.schedules) == 1
            assert result.schedules[0].id == "sch_123"
            mock_http.get.assert_called_once()


class TestNotifGetSchedule:
    """Test Notif.get_schedule() method."""

    @pytest.fixture
    def client(self):
        return Notif(api_key="nsh_testkey12345678901234567890")

    @pytest.mark.asyncio
    async def test_get_schedule_success(self, client):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.content = b'{}'
        mock_response.json.return_value = {
            "id": "sch_123",
            "topic": "test.topic",
            "data": {"foo": "bar"},
            "scheduled_for": "2025-01-15T10:00:00Z",
            "status": "pending",
            "created_at": "2025-01-01T00:00:00Z",
        }

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.get = AsyncMock(return_value=mock_response)
            mock_get_client.return_value = mock_http

            result = await client.get_schedule("sch_123")

            assert result.id == "sch_123"
            assert result.status == "pending"

    @pytest.mark.asyncio
    async def test_get_schedule_not_found(self, client):
        mock_response = MagicMock()
        mock_response.status_code = 404
        mock_response.content = b'{"error": "schedule not found"}'
        mock_response.json.return_value = {"error": "schedule not found"}

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.get = AsyncMock(return_value=mock_response)
            mock_get_client.return_value = mock_http

            with pytest.raises(APIError) as exc_info:
                await client.get_schedule("sch_notfound")

            assert exc_info.value.status_code == 404


class TestNotifCancelSchedule:
    """Test Notif.cancel_schedule() method."""

    @pytest.fixture
    def client(self):
        return Notif(api_key="nsh_testkey12345678901234567890")

    @pytest.mark.asyncio
    async def test_cancel_schedule_success(self, client):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.content = b''

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.delete = AsyncMock(return_value=mock_response)
            mock_get_client.return_value = mock_http

            await client.cancel_schedule("sch_123")
            mock_http.delete.assert_called_once()


class TestNotifRunSchedule:
    """Test Notif.run_schedule() method."""

    @pytest.fixture
    def client(self):
        return Notif(api_key="nsh_testkey12345678901234567890")

    @pytest.mark.asyncio
    async def test_run_schedule_success(self, client):
        mock_response = MagicMock()
        mock_response.status_code = 200
        mock_response.content = b'{}'
        mock_response.json.return_value = {
            "schedule_id": "sch_123",
            "event_id": "evt_456",
        }

        with patch.object(client, "_get_client") as mock_get_client:
            mock_http = AsyncMock()
            mock_http.post = AsyncMock(return_value=mock_response)
            mock_get_client.return_value = mock_http

            result = await client.run_schedule("sch_123")

            assert result.schedule_id == "sch_123"
            assert result.event_id == "evt_456"
