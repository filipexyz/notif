class NotifError(Exception):
    """Base exception for notif SDK."""

    pass


class APIError(NotifError):
    """Error from API (HTTP errors)."""

    def __init__(self, status_code: int, message: str) -> None:
        self.status_code = status_code
        super().__init__(f"API error ({status_code}): {message}")


class AuthError(NotifError):
    """Authentication error (401)."""

    def __init__(self, message: str = "invalid or missing API key") -> None:
        super().__init__(f"authentication error: {message}")


class ConnectionError(NotifError):
    """Network/connection failure."""

    def __init__(self, message: str, cause: Exception | None = None) -> None:
        self.cause = cause
        super().__init__(f"connection error: {message}")
