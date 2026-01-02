#!/usr/bin/env python3
"""
Claude Code client - discover workers and chat interactively.

Usage: python claude_client.py [--server http://localhost:8080]
"""
from __future__ import annotations

import argparse
import asyncio
import os
import sys
from dataclasses import dataclass
from uuid import uuid4

# Add SDK to path if running from scripts/
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", "sdk", "python", "src"))

from notifsh import Notif

try:
    from rich.console import Console
    from rich.table import Table
    from rich.panel import Panel
    from rich.markdown import Markdown
    from rich.live import Live
    from rich.spinner import Spinner
    from rich.text import Text
except ImportError:
    print("Please install rich: pip install rich")
    sys.exit(1)

# Protocol constants
TOPIC_DISCOVER = "claude.agents.discover"
TOPIC_AVAILABLE = "claude.agents.available"
TOPIC_TRIGGER = "claude.trigger.{agent}"
TOPIC_RESPONSE = "claude.response.{agent}"

console = Console()


@dataclass
class Agent:
    name: str
    budget_usd: float | None
    cwd: str | None


class ClaudeClient:
    def __init__(self, server: str, api_key: str | None = None) -> None:
        self.server = server
        self.api_key = api_key
        self.client: Notif | None = None

    async def __aenter__(self) -> ClaudeClient:
        self.client = Notif(api_key=self.api_key, server=self.server)
        return self

    async def __aexit__(self, *args) -> None:
        if self.client:
            await self.client.close()

    async def discover(self, timeout: float = 2.0) -> list[Agent]:
        """Discover available agents."""
        agents: list[Agent] = []
        seen: set[str] = set()

        # Subscribe first
        subscription = self.client.subscribe(TOPIC_AVAILABLE)

        async def collect():
            try:
                async with asyncio.timeout(timeout + 2):
                    async for event in subscription:
                        name = event.data.get("agent", "")
                        if name and name not in seen:
                            seen.add(name)
                            agents.append(Agent(
                                name=name,
                                budget_usd=event.data.get("budget_usd", 0),
                                cwd=event.data.get("cwd", ""),
                            ))
            except asyncio.TimeoutError:
                pass

        # Start collector in background
        collector = asyncio.create_task(collect())

        # Yield to let collector start
        await asyncio.sleep(0)

        # Wait for WebSocket to connect
        for _ in range(30):  # Max 3 seconds
            if subscription.is_connected:
                break
            await asyncio.sleep(0.1)

        # Broadcast discovery
        await self.client.emit(TOPIC_DISCOVER, {})

        # Wait for responses
        await asyncio.sleep(timeout)

        # Cleanup
        collector.cancel()
        try:
            await collector
        except asyncio.CancelledError:
            pass

        return agents

    async def ask(self, agent: str, prompt: str, session: str | None = None, timeout: float = 300) -> dict:
        """Send prompt to agent and wait for response."""
        request_id = f"req_{uuid4().hex[:12]}"
        trigger_topic = TOPIC_TRIGGER.format(agent=agent)
        response_topic = TOPIC_RESPONSE.format(agent=agent)

        result_future: asyncio.Future = asyncio.Future()

        async def listen():
            async for event in self.client.subscribe(response_topic):
                if event.data.get("request_id") == request_id:
                    result_future.set_result(event.data)
                    return

        listener = asyncio.create_task(listen())

        try:
            await asyncio.sleep(0.05)
            await self.client.emit(trigger_topic, {
                "prompt": prompt,
                "session": session,
                "request_id": request_id,
            })

            async with asyncio.timeout(timeout):
                return await result_future
        finally:
            listener.cancel()


async def show_agents(agents: list[Agent]) -> None:
    """Display agent table."""
    table = Table(title="Available Agents", show_header=True, header_style="bold cyan")
    table.add_column("#", style="dim", width=3)
    table.add_column("Agent", style="green")
    table.add_column("Budget", justify="right")
    table.add_column("Working Directory")

    for i, agent in enumerate(agents, 1):
        table.add_row(
            str(i),
            agent.name,
            f"${agent.budget_usd:.2f}" if agent.budget_usd else "unlimited",
            agent.cwd or "-",
        )

    console.print()
    console.print(table)
    console.print()


async def chat_loop(client: ClaudeClient, agent: Agent) -> bool:
    """Run chat REPL. Returns True to go back to selection, False to exit."""
    console.print(Panel(
        f"Chat with [bold green]{agent.name}[/]\n"
        f"Budget: {'unlimited' if agent.budget_usd is None else f'${agent.budget_usd:.2f}'} | CWD: {agent.cwd or 'N/A'}\n\n"
        "[dim]Type 'back' to select another agent, 'exit' to quit[/]",
        title="Chat Session",
    ))
    console.print()

    session: str | None = None

    while True:
        try:
            prompt = console.input("[bold cyan]> [/]").strip()
        except EOFError:
            return False

        if not prompt:
            continue
        if prompt.lower() == "exit":
            return False
        if prompt.lower() == "back":
            return True

        # Show spinner while waiting
        with console.status("[bold blue]Thinking...", spinner="dots"):
            try:
                response = await client.ask(agent.name, prompt, session)
            except asyncio.TimeoutError:
                console.print("[red]Timeout waiting for response[/]")
                continue
            except Exception as e:
                console.print(f"[red]Error: {e}[/]")
                continue

        # Update session
        if response.get("session"):
            session = response["session"]

        # Display response
        console.print()
        if response.get("is_error"):
            console.print(Panel(response.get("result", ""), title="Error", border_style="red"))
        else:
            result = response.get("result", "")
            try:
                console.print(Markdown(result))
            except Exception:
                console.print(result)

        # Show metadata
        meta = []
        if response.get("cost_usd"):
            meta.append(f"cost: ${response['cost_usd']:.4f}")
        if session:
            meta.append(f"session: {session[:8]}...")
        if meta:
            console.print(f"[dim][{' | '.join(meta)}][/]")
        console.print()


async def main_loop(server: str, api_key: str | None = None) -> None:
    """Main application loop."""
    async with ClaudeClient(server, api_key) as client:
        while True:
            # Discovery phase
            console.print("[bold]Discovering agents...[/]")
            with console.status("[bold blue]Scanning...", spinner="dots"):
                agents = await client.discover()

            if not agents:
                console.print("[yellow]No agents found. Make sure workers are running.[/]")
                try:
                    action = console.input("[dim]Press Enter to retry, or 'exit' to quit: [/]").strip()
                    if action.lower() == "exit":
                        break
                    continue
                except EOFError:
                    break

            await show_agents(agents)

            # Selection
            try:
                choice = console.input(f"Select agent [1-{len(agents)}] or 'r' to refresh: ").strip()
            except EOFError:
                break

            if choice.lower() == "r":
                continue
            if choice.lower() in ("exit", "q"):
                break

            try:
                idx = int(choice) - 1
                if 0 <= idx < len(agents):
                    go_back = await chat_loop(client, agents[idx])
                    if not go_back:
                        break
                else:
                    console.print("[red]Invalid selection[/]")
            except ValueError:
                console.print("[red]Invalid selection[/]")


def main() -> None:
    parser = argparse.ArgumentParser(description="Claude Code interactive client")
    parser.add_argument("--server", "-s", default="https://api.notif.sh", help="Notif server URL")
    parser.add_argument("--api-key", "-k", help="Notif API key (or set NOTIF_API_KEY)")
    args = parser.parse_args()

    console.print(Panel.fit(
        "[bold]Claude Code Client[/]\n"
        "Discover and chat with Claude workers",
        border_style="blue",
    ))

    try:
        asyncio.run(main_loop(args.server, args.api_key))
    except KeyboardInterrupt:
        pass

    console.print("\n[dim]Goodbye![/]")


if __name__ == "__main__":
    main()
