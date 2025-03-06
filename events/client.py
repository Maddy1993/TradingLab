# events/client.py
import json
import asyncio
import nats
from nats.js.api import StreamConfig
from typing import Dict, Any, Callable, Optional, List, Union

class EventClient:
    """Client for interacting with the event messaging system."""

    def __init__(self, nats_url="nats://nats:4222"):
        """Initialize the event client with NATS server URL."""
        self.nats_url = nats_url
        self.nc = None
        self.js = None
        self.subscriptions = {}
        self.streams = {}

    async def connect(self):
        """Connect to NATS server and set up JetStream."""
        self.nc = await nats.connect(self.nats_url)
        self.js = self.nc.jetstream()

        # Ensure all required streams exist
        await self._setup_streams()

    async def _setup_streams(self):
        """Set up all the required streams."""
        # Market live data stream
        try:
            await self.js.add_stream(
                    name="MARKET_LIVE",
                    subjects=["market.live.*"]
            )
            self.streams["MARKET_LIVE"] = True
        except nats.js.errors.BadRequestError:
            # Stream already exists
            self.streams["MARKET_LIVE"] = True

        # Market daily data stream
        try:
            await self.js.add_stream(
                    name="MARKET_DAILY",
                    subjects=["market.daily.*"]
            )
            self.streams["MARKET_DAILY"] = True
        except nats.js.errors.BadRequestError:
            # Stream already exists
            self.streams["MARKET_DAILY"] = True

        # Market historical data stream
        try:
            await self.js.add_stream(
                    name="MARKET_HISTORICAL",
                    subjects=["market.historical.data.*"]
            )
            self.streams["MARKET_HISTORICAL"] = True
        except nats.js.errors.BadRequestError:
            # Stream already exists
            self.streams["MARKET_HISTORICAL"] = True

        # Trading signals stream
        try:
            await self.js.add_stream(
                    name="SIGNALS",
                    subjects=["signals.*"]
            )
            self.streams["SIGNALS"] = True
        except nats.js.errors.BadRequestError:
            # Stream already exists
            self.streams["SIGNALS"] = True

        # Requests stream
        try:
            await self.js.add_stream(
                    name="REQUESTS",
                    subjects=["requests.*"]
            )
            self.streams["REQUESTS"] = True
        except nats.js.errors.BadRequestError:
            # Stream already exists
            self.streams["REQUESTS"] = True

    async def publish_market_live_data(self, ticker: str, data: Dict[str, Any]):
        """Publish live market data."""
        subject = f"market.live.{ticker}"
        # Make sure data type is set correctly
        if isinstance(data, dict) and "data_type" not in data:
            data["data_type"] = "live"

        payload = json.dumps(data).encode()
        await self.js.publish(subject, payload)

    async def publish_market_daily_data(self, ticker: str, data: Dict[str, Any]):
        """Publish daily market data."""
        subject = f"market.daily.{ticker}"
        # Make sure data type is set correctly
        if isinstance(data, dict) and "data_type" not in data:
            data["data_type"] = "daily"

        payload = json.dumps(data).encode()
        await self.js.publish(subject, payload)

    async def publish_historical_data(self, ticker: str, timeframe: str, days: int, data: Dict[str, Any]):
        """Publish historical market data."""
        subject = f"market.historical.data.{ticker}.{timeframe}.{days}"

        # Ensure metadata is present
        if "metadata" not in data:
            data["metadata"] = {
                "ticker": ticker,
                "timeframe": timeframe,
                "days": days,
                "data_type": "historical"
            }

        payload = json.dumps(data).encode()
        await self.js.publish(subject, payload)

    async def request_historical_data(self, ticker: str, timeframe: str, days: int,
                                      request_data: Optional[Dict[str, Any]] = None):
        """Request historical data for a ticker."""
        subject = f"requests.historical.{ticker}.{timeframe}.{days}"

        if request_data is None:
            request_data = {}

        # Add default metadata if not provided
        if "timestamp" not in request_data:
            from datetime import datetime
            request_data["timestamp"] = datetime.now().isoformat()

        if "source" not in request_data:
            request_data["source"] = "python_client"

        payload = json.dumps(request_data).encode()
        await self.js.publish(subject, payload)

    async def publish_signal(self, ticker: str, signal_data: Dict[str, Any]):
        """Publish a trading signal."""
        subject = f"signals.{ticker}"
        payload = json.dumps(signal_data).encode()
        await self.js.publish(subject, payload)

    async def subscribe_market_live_data(self, ticker: str, callback: Callable[[Dict[str, Any]], None]):
        """Subscribe to live market data for a ticker."""
        subject = f"market.live.{ticker}"

        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
                await callback(data)
                await msg.ack()
            except Exception as e:
                print(f"Error processing message: {e}")

        sub = await self.js.subscribe(subject, cb=message_handler, durable="market-live-consumer")
        self.subscriptions[subject] = sub
        return sub

    async def subscribe_market_daily_data(self, ticker: str, callback: Callable[[Dict[str, Any]], None]):
        """Subscribe to daily market data for a ticker."""
        subject = f"market.daily.{ticker}"

        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
                await callback(data)
                await msg.ack()
            except Exception as e:
                print(f"Error processing message: {e}")

        sub = await self.js.subscribe(subject, cb=message_handler, durable="market-daily-consumer")
        self.subscriptions[subject] = sub
        return sub

    async def subscribe_historical_data(self, ticker: str, timeframe: str, days: int,
                                        callback: Callable[[Dict[str, Any]], None]):
        """Subscribe to historical data for a ticker."""
        subject = f"market.historical.data.{ticker}.{timeframe}.{days}"

        async def message_handler(msg):
            try:
                data = json.loads(msg.data.decode())
                await callback(data)
                await msg.ack()
            except Exception as e:
                print(f"Error processing message: {e}")

        sub = await self.js.subscribe(subject, cb=message_handler, durable="historical-data-consumer")
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

    async def subscribe_historical_requests(self, callback: Callable[[str, str, int, Dict[str, Any]], None]):
        """Subscribe to historical data requests."""
        subject = "requests.historical.*.*.*"

        async def message_handler(msg):
            try:
                # Parse subject to extract parameters
                parts = msg.subject.split(".")
                if len(parts) >= 5:
                    ticker = parts[2]
                    timeframe = parts[3]
                    days = int(parts[4])

                    data = json.loads(msg.data.decode())
                    await callback(ticker, timeframe, days, data)
                    await msg.ack()
            except Exception as e:
                print(f"Error processing request: {e}")

        sub = await self.js.subscribe(subject, cb=message_handler, durable="historical-requests-consumer")
        self.subscriptions["requests.historical"] = sub
        return sub

    async def close(self):
        """Close all subscriptions and connection."""
        for sub in self.subscriptions.values():
            await sub.unsubscribe()

        if self.nc:
            await self.nc.close()