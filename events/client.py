# events/client.py
import json
import asyncio
import nats
from nats.js.api import StreamConfig
from typing import Dict, Any, Callable, Optional

class EventClient:
    """Client for interacting with the event messaging system."""

    def __init__(self, nats_url="nats://nats:4222"):
        """Initialize the event client with NATS server URL."""
        self.nats_url = nats_url
        self.nc = None
        self.js = None
        self.subscriptions = {}

    async def connect(self):
        """Connect to NATS server and set up JetStream."""
        self.nc = await nats.connect(self.nats_url)
        self.js = self.nc.jetstream()

        # Ensure the streams exist
        try:
            await self.js.add_stream(name="MARKET_DATA", subjects=["market.data.*"])
        except nats.js.errors.BadRequestError:
            # Stream already exists
            pass

        try:
            await self.js.add_stream(name="SIGNALS", subjects=["signals.*"])
        except nats.js.errors.BadRequestError:
            # Stream already exists
            pass

    async def publish_market_data(self, ticker: str, data: Dict[str, Any]):
        """Publish market data for a ticker."""
        subject = f"market.data.{ticker}"
        payload = json.dumps(data).encode()
        await self.js.publish(subject, payload)

    async def publish_signal(self, ticker: str, signal_data: Dict[str, Any]):
        """Publish a trading signal."""
        subject = f"signals.{ticker}"
        payload = json.dumps(signal_data).encode()
        await self.js.publish(subject, payload)

    async def subscribe_market_data(self, ticker: str, callback: Callable[[Dict[str, Any]], None]):
        """Subscribe to market data for a ticker."""
        subject = f"market.data.{ticker}"

        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
                await callback(data)
                await msg.ack()
            except Exception as e:
                print(f"Error processing message: {e}")

        sub = await self.js.subscribe(subject, cb=message_handler, durable="market-data-consumer")
        self.subscriptions[subject] = sub
        return sub

    async def subscribe_signals(self, ticker: str, callback: Callable[[Dict[str, Any]], None]):
        """Subscribe to signals for a ticker."""
        subject = f"signals.{ticker}"

        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
                await callback(data)
                await msg.ack()
            except Exception as e:
                print(f"Error processing message: {e}")

        sub = await self.js.subscribe(subject, cb=message_handler, durable="signals-consumer")
        self.subscriptions[subject] = sub
        return sub

    async def close(self):
        """Close all subscriptions and connection."""
        for sub in self.subscriptions.values():
            await sub.unsubscribe()

        if self.nc:
            await self.nc.close()