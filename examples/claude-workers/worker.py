#!/usr/bin/env python3
"""
Claude Code worker - subscribes to triggers and runs Claude CLI.

Usage: python worker.py <agent> [--budget <usd>] [--cwd /path]
"""
from __future__ import annotations

import argparse
import asyncio
import json
import os
import signal
import sys
import time
from dataclasses import dataclass
from typing import Any
from uuid import uuid4

# Add SDK to path if running from scripts/
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "sdk", "python", "src"))

from notifsh import Notif

# Protocol constants
TOPIC_DISCOVER = "claude.agents.discover"
TOPIC_AVAILABLE = "claude.agents.available"
TOPIC_TRIGGER = "claude.trigger.{agent}"
TOPIC_RESPONSE = "claude.response.{agent}"


@dataclass
class WorkerConfig:
    agent: str
    budget_usd: float | None = None
    cwd: str | None = None
    server: str = "https://api.notif.sh"
    api_key: str | None = None


class ClaudeWorker:
    def __init__(self, config: WorkerConfig) -> None:
        self.config = config
        self.client: Notif | None = None
        self._shutdown = asyncio.Event()

    @property
    def trigger_topic(self) -> str:
        return TOPIC_TRIGGER.format(agent=self.config.agent)

    @property
    def response_topic(self) -> str:
        return TOPIC_RESPONSE.format(agent=self.config.agent)

    async def start(self) -> None:
        self.client = Notif(api_key=self.config.api_key, server=self.config.server)

        print(f"Claude worker started")
        print(f"  Agent: {self.config.agent}")
        print(f"  Budget: {'unlimited' if self.config.budget_usd is None else f'${self.config.budget_usd:.2f}'}")
        print(f"  CWD: {self.config.cwd or os.getcwd()}")
        print(f"  Topic: {self.trigger_topic}")
        print()

        # Run both handlers concurrently
        try:
            await asyncio.gather(
                self._handle_discovery(),
                self._handle_triggers(),
            )
        except Exception as e:
            print(f"Error: {e}")
            raise

    async def _handle_discovery(self) -> None:
        """Respond to discovery requests."""
        print("  [discovery] Subscribing to discovery topic...")
        try:
            async for event in self.client.subscribe(TOPIC_DISCOVER, group=f"worker-{self.config.agent}"):
                if self._shutdown.is_set():
                    break
                await self._announce()
        except Exception as e:
            print(f"  [discovery] Error: {e}")

    async def _announce(self) -> None:
        """Publish availability."""
        await self.client.emit(TOPIC_AVAILABLE, {
            "agent": self.config.agent,
            "budget_usd": self.config.budget_usd,
            "cwd": self.config.cwd or os.getcwd(),
        })

    async def _handle_triggers(self) -> None:
        """Process trigger requests."""
        print(f"  [triggers] Subscribing to {self.trigger_topic}...")
        try:
            async for event in self.client.subscribe(self.trigger_topic, group=f"worker-{self.config.agent}"):
                if self._shutdown.is_set():
                    break
                asyncio.create_task(self._process(event.data))
        except Exception as e:
            print(f"  [triggers] Error: {e}")

    async def _process(self, data: dict[str, Any]) -> None:
        """Process a single trigger request."""
        prompt = data.get("prompt", "")
        session = data.get("session")
        request_id = data.get("request_id", f"req_{uuid4().hex[:12]}")

        if not prompt:
            await self._respond(request_id, "No prompt provided", "", True, 0)
            return

        print(f">>> [{request_id[:12]}] {prompt[:60]}...")

        start = time.monotonic()
        result = await self._run_claude(prompt, session)
        duration = time.monotonic() - start

        await self._respond(
            request_id,
            result["result"],
            result["session"],
            result["is_error"],
            result.get("cost_usd", 0),
        )

        status = "error" if result["is_error"] else "done"
        print(f"<<< [{request_id[:12]}] {status} ({duration:.1f}s, ${result.get('cost_usd', 0):.4f})")

    async def _run_claude(self, prompt: str, session: str | None) -> dict[str, Any]:
        """Execute Claude CLI."""
        cmd = ["claude", "-p", "--output-format", "json"]

        if self.config.budget_usd is not None:
            cmd.extend(["--max-budget-usd", str(self.config.budget_usd)])

        if session:
            cmd.extend(["--resume", session])

        proc = await asyncio.create_subprocess_exec(
            *cmd,
            stdin=asyncio.subprocess.PIPE,
            stdout=asyncio.subprocess.PIPE,
            stderr=asyncio.subprocess.PIPE,
            cwd=self.config.cwd,
        )

        stdout, stderr = await proc.communicate(prompt.encode())

        if proc.returncode != 0:
            return {
                "result": stderr.decode() or stdout.decode() or "Claude CLI failed",
                "session": "",
                "is_error": True,
            }

        try:
            output = json.loads(stdout.decode())
            # Claude outputs array of messages, last one has result
            last = output[-1] if isinstance(output, list) else output
            return {
                "result": last.get("result", ""),
                "session": output[0].get("session_id", "") if isinstance(output, list) else "",
                "is_error": last.get("is_error", False),
                "cost_usd": last.get("total_cost_usd", 0),
            }
        except (json.JSONDecodeError, KeyError, IndexError) as e:
            return {
                "result": stdout.decode() or str(e),
                "session": "",
                "is_error": False,
            }

    async def _respond(self, request_id: str, result: str, session: str, is_error: bool, cost: float) -> None:
        """Send response."""
        await self.client.emit(self.response_topic, {
            "request_id": request_id,
            "result": result,
            "session": session,
            "is_error": is_error,
            "cost_usd": cost,
        })

    def shutdown(self) -> None:
        self._shutdown.set()

    async def close(self) -> None:
        if self.client:
            await self.client.close()


def main() -> None:
    parser = argparse.ArgumentParser(description="Claude Code worker")
    parser.add_argument("agent", help="Agent name (e.g., 'coder')")
    parser.add_argument("--budget", "-b", type=float, help="Max budget in USD (unlimited if not set)")
    parser.add_argument("--cwd", "-C", help="Working directory")
    parser.add_argument("--server", "-s", default="https://api.notif.sh", help="Notif server URL")
    parser.add_argument("--api-key", "-k", help="Notif API key (or set NOTIF_API_KEY)")
    args = parser.parse_args()

    config = WorkerConfig(
        agent=args.agent,
        budget_usd=args.budget,
        cwd=args.cwd,
        server=args.server,
        api_key=args.api_key,
    )

    worker = ClaudeWorker(config)

    loop = asyncio.new_event_loop()
    asyncio.set_event_loop(loop)

    # Signal handlers
    for sig in (signal.SIGINT, signal.SIGTERM):
        loop.add_signal_handler(sig, worker.shutdown)

    try:
        loop.run_until_complete(worker.start())
    except KeyboardInterrupt:
        pass
    finally:
        loop.run_until_complete(worker.close())
        print("\nWorker stopped")


if __name__ == "__main__":
    main()
