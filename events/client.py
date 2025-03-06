# events/client.py
import json
import asyncio
import nats
import re
from datetime import datetime
from nats.js.api import StreamConfig
from typing import Dict, Any, Callable, Optional, Union, List

class EventClient:
    """Client for interacting with the event messaging system."""

    def __init__(self, nats_url="nats://nats:4222"):
        """Initialize the event client with NATS server URL."""
        self.nats_url = nats_url
        self.nc = None
        self.js = None
        self.subscriptions = {}
        self.request_reply_callbacks = {}

    async def connect(self):
        """Connect to NATS server and set up JetStream."""
        self.nc = await nats.connect(
                self.nats_url,
                reconnect_time_wait=2,
                max_reconnect_attempts=10,
                connect_timeout=10
        )
        self.js = self.nc.jetstream()

        # Ensure the streams exist
        streams = [
            # Market data streams
            ("MARKET_LIVE", ["market.live.*"]),
            ("MARKET_DAILY", ["market.daily.*"]),
            ("MARKET_HISTORICAL", ["market.historical.*"]),
            # Trading signals and recommendations
            ("SIGNALS", ["signals.*"]),
            ("RECOMMENDATIONS", ["recommendations.*"]),
            # Request streams
            ("REQUESTS", ["requests.*"]),
        ]

        for stream_name, subjects in streams:
            try:
                await self.js.add_stream(name=stream_name, subjects=subjects)
            except nats.js.errors.BadRequestError:
                # Stream already exists
                pass

    async def publish_market_data(self, ticker: str, data: Dict[str, Any]) -> None:
        """Publish market data for a ticker."""
        if not self.js:
            raise RuntimeError("Not connected to NATS")

        subject = f"market.live.{ticker}"
        payload = json.dumps(data).encode()
        await self.js.publish(subject, payload)

    async def publish_signal(self, ticker: str, signal_data: Dict[str, Any]) -> None:
        """Publish a trading signal."""
        if not self.js:
            raise RuntimeError("Not connected to NATS")

        subject = f"signals.{ticker}"
        payload = json.dumps(signal_data).encode()
        await self.js.publish(subject, payload)

    async def publish_recommendation(self, ticker: str, recommendation_data: Dict[str, Any]) -> None:
        """Publish an options recommendation."""
        if not self.js:
            raise RuntimeError("Not connected to NATS")

        subject = f"recommendations.{ticker}"
        payload = json.dumps(recommendation_data).encode()
        await self.js.publish(subject, payload)

    async def request_historical_data(self, ticker: str, days: int, interval: str = '15min') -> None:
        """Request historical data for a ticker."""
        if not self.js:
            raise RuntimeError("Not connected to NATS")

        subject = f"market.historical.request.{ticker}.{interval}.{days}"
        request = {
            "ticker": ticker,
            "days": days,
            "interval": interval,
            "timestamp": str(datetime.now())
        }
        payload = json.dumps(request).encode()
        await self.js.publish(subject, payload)

    async def subscribe_market_data(self, ticker: str, callback: Callable[[Dict[str, Any]], None]) -> None:
        """Subscribe to market data for a ticker."""
        if not self.js:
            raise RuntimeError("Not connected to NATS")

        # Use wildcard or specific ticker
        subject = f"market.live.{ticker}"

        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
                await callback(data)
                await msg.ack()
            except Exception as e:
                print(f"Error processing message: {e}")

        # Sanitize durable name - remove invalid characters
        safe_ticker = re.sub(r'[.*>]', '-', ticker)
        durable_name = f"market-data-consumer-{safe_ticker}"
        sub = await self.js.subscribe(subject, cb=message_handler, durable=durable_name)
        self.subscriptions[subject] = sub
        return sub

    async def subscribe_market_historical(self, ticker: str, callback: Callable[[Dict[str, Any]], None]) -> None:
        """Subscribe to historical market data responses."""
        if not self.js:
            raise RuntimeError("Not connected to NATS")

        # Use wildcard or specific ticker
        subject = f"market.historical.data.{ticker}"

        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
                await callback(data)
                await msg.ack()
            except Exception as e:
                print(f"Error processing message: {e}")

        # Sanitize durable name - remove invalid characters
        safe_ticker = re.sub(r'[.*>]', '-', ticker)
        durable_name = f"historical-data-consumer-{safe_ticker}"
        sub = await self.js.subscribe(subject, cb=message_handler, durable=durable_name)
        self.subscriptions[subject] = sub
        return sub

    async def subscribe_signals(self, ticker: str, callback: Callable[[Dict[str, Any]], None]) -> None:
        """Subscribe to signals for a ticker."""
        if not self.js:
            raise RuntimeError("Not connected to NATS")

        subject = f"signals.{ticker}"

        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
                await callback(data)
                await msg.ack()
            except Exception as e:
                print(f"Error processing message: {e}")

        # Sanitize durable name - remove invalid characters
        safe_ticker = re.sub(r'[.*>]', '-', ticker)
        durable_name = f"signals-consumer-{safe_ticker}"
        sub = await self.js.subscribe(subject, cb=message_handler, durable=durable_name)
        self.subscriptions[subject] = sub
        return sub

    async def subscribe_recommendations(self, ticker: str, callback: Callable[[Dict[str, Any]], None]) -> None:
        """Subscribe to recommendations for a ticker."""
        if not self.js:
            raise RuntimeError("Not connected to NATS")

        subject = f"recommendations.{ticker}"

        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
                await callback(data)
                await msg.ack()
            except Exception as e:
                print(f"Error processing message: {e}")

        # Sanitize durable name - remove invalid characters
        safe_ticker = re.sub(r'[.*>]', '-', ticker)
        durable_name = f"recommendations-consumer-{safe_ticker}"
        sub = await self.js.subscribe(subject, cb=message_handler, durable=durable_name)
        self.subscriptions[subject] = sub
        return sub

    async def close(self) -> None:
        """Close all subscriptions and connection."""
        for sub in self.subscriptions.values():
            await sub.unsubscribe()

        if self.nc:
            await self.nc.close()