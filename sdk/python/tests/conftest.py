import os

import pytest


@pytest.fixture(autouse=True)
def clear_env():
    """Clear NOTIF_API_KEY env var before each test."""
    env_key = "NOTIF_API_KEY"
    original = os.environ.get(env_key)
    os.environ.pop(env_key, None)
    yield
    if original:
        os.environ[env_key] = original
    else:
        os.environ.pop(env_key, None)
