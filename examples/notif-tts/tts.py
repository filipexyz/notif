#!/usr/bin/env python3
"""
notif TTS - Subscribe to a topic and speak text using ElevenLabs.

Usage:
    python tts.py [options]

Requires:
    pip install notifsh httpx python-dotenv

Environment:
    ELEVENLABS_API_KEY  - ElevenLabs API key (required)
    NOTIF_API_KEY       - notif.sh API key (required)
"""
from __future__ import annotations

import argparse
import asyncio
import os
import platform
import signal
import sys
import tempfile

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "..", "sdk", "python", "src"))

from dotenv import load_dotenv
load_dotenv(os.path.join(os.path.dirname(__file__), ".env"))

import httpx
from notifsh import Notif

ELEVENLABS_URL = "https://api.elevenlabs.io/v1/text-to-speech/{voice_id}"
DEFAULT_VOICE = "JBFqnCBsd6RMkjVDRZzb"  # George
DEFAULT_MODEL = "eleven_multilingual_v2"
DEFAULT_TOPIC = "tts"


async def synthesize(client: httpx.AsyncClient, api_key: str, text: str, voice_id: str, model: str) -> bytes:
    """Call ElevenLabs TTS and return raw audio bytes (mp3)."""
    resp = await client.post(
        ELEVENLABS_URL.format(voice_id=voice_id),
        headers={"xi-api-key": api_key},
        json={
            "text": text,
            "model_id": model,
            "voice_settings": {"stability": 0.5, "similarity_boost": 0.75},
        },
        timeout=30.0,
    )
    resp.raise_for_status()
    return resp.content


async def play_audio(audio: bytes) -> None:
    """Write audio to a temp file and play it."""
    with tempfile.NamedTemporaryFile(suffix=".mp3", delete=False) as f:
        f.write(audio)
        path = f.name

    try:
        system = platform.system()
        if system == "Darwin":
            cmd = ["afplay", path]
        elif system == "Linux":
            cmd = ["mpv", "--no-video", "--really-quiet", path]
        else:
            cmd = ["ffplay", "-nodisp", "-autoexit", "-loglevel", "quiet", path]

        proc = await asyncio.create_subprocess_exec(
            *cmd,
            stdout=asyncio.subprocess.DEVNULL,
            stderr=asyncio.subprocess.DEVNULL,
        )
        await proc.wait()
    finally:
        os.unlink(path)


async def run(args: argparse.Namespace) -> None:
    elevenlabs_key = args.elevenlabs_key or os.environ.get("ELEVENLABS_API_KEY")
    if not elevenlabs_key:
        print("Error: set ELEVENLABS_API_KEY or pass --elevenlabs-key")
        sys.exit(1)

    notif = Notif(api_key=args.api_key, server=args.server)
    shutdown = asyncio.Event()

    print(f"notif tts started")
    print(f"  Topic: {args.topic}")
    print(f"  Voice: {args.voice}")
    print(f"  Model: {args.model}")
    print()

    http = httpx.AsyncClient()

    try:
        async for event in notif.subscribe(args.topic):
            if shutdown.is_set():
                break

            text = event.data.get("text", "")
            if not text:
                print(f"  [skip] empty text from event {event.id}")
                continue

            print(f"  [speak] {text[:80]}{'...' if len(text) > 80 else ''}")

            try:
                audio = await synthesize(http, elevenlabs_key, text, args.voice, args.model)
                await play_audio(audio)
            except httpx.HTTPStatusError as e:
                print(f"  [error] ElevenLabs API: {e.response.status_code} {e.response.text[:200]}")
            except Exception as e:
                print(f"  [error] {e}")
    except asyncio.CancelledError:
        pass
    finally:
        await http.aclose()
        await notif.close()
        print("\nStopped")


def main() -> None:
    parser = argparse.ArgumentParser(description="notif TTS with ElevenLabs")
    parser.add_argument("--topic", "-t", default=DEFAULT_TOPIC, help=f"Topic to subscribe (default: {DEFAULT_TOPIC})")
    parser.add_argument("--voice", "-v", default=DEFAULT_VOICE, help="ElevenLabs voice ID")
    parser.add_argument("--model", "-m", default=DEFAULT_MODEL, help="ElevenLabs model ID")
    parser.add_argument("--server", "-s", default="https://api.notif.sh", help="Notif server URL")
    parser.add_argument("--api-key", "-k", help="Notif API key (or NOTIF_API_KEY)")
    parser.add_argument("--elevenlabs-key", help="ElevenLabs API key (or ELEVENLABS_API_KEY)")
    args = parser.parse_args()

    loop = asyncio.new_event_loop()
    task = loop.create_task(run(args))

    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, task.cancel)

    try:
        loop.run_until_complete(task)
    except asyncio.CancelledError:
        pass


if __name__ == "__main__":
    main()
