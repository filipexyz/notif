"""Python SDK for notif.sh event hub."""

from .client import Notif
from .errors import APIError, AuthError, ConnectionError, NotifError
from .events import EventStream
from .types import EmitResponse, Event

__all__ = [
    "Notif",
    "Event",
    "EventStream",
    "EmitResponse",
    "NotifError",
    "APIError",
    "AuthError",
    "ConnectionError",
]

__version__ = "0.1.0"
