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
        import logging
        max_attempts = 3
        attempt = 0
        
        while attempt < max_attempts:
            try:
                logging.info(f"Connecting to NATS at {self.nats_url} (attempt {attempt+1}/{max_attempts})")
                self.nc = await nats.connect(
                        self.nats_url,
                        reconnect_time_wait=2,
                        max_reconnect_attempts=20,  # Increased reconnect attempts
                        connect_timeout=15,         # Increased connect timeout
                        ping_interval=20,           # More frequent ping to detect disconnections
                        max_outstanding_pings=5,    # Allow more outstanding pings before considering a connection broken
                        error_cb=self._on_nats_error,
                        disconnected_cb=self._on_nats_disconnected,
                        reconnected_cb=self._on_nats_reconnected,
                        closed_cb=self._on_nats_closed
                )
                logging.info("Connected to NATS server")
                self.js = self.nc.jetstream(timeout=15)  # Increased JetStream timeout
                
                # Ensure the streams exist
                await self._setup_streams()
                return  # Connection successful, exit loop
                
            except Exception as e:
                attempt += 1
                logging.error(f"NATS connection attempt {attempt} failed: {str(e)}")
                if attempt < max_attempts:
                    # Wait with an exponential backoff before trying again
                    wait_time = 2 ** attempt
                    logging.info(f"Retrying in {wait_time} seconds...")
                    await asyncio.sleep(wait_time)
                else:
                    logging.error(f"Failed to connect to NATS after {max_attempts} attempts")
                    raise

    async def _on_nats_error(self, e):
        """Callback for NATS errors."""
        import logging
        logging.error(f"NATS error: {str(e)}")

    async def _on_nats_disconnected(self):
        """Callback for NATS disconnection."""
        import logging
        logging.warning("Disconnected from NATS server")

    async def _on_nats_reconnected(self):
        """Callback for NATS reconnection."""
        import logging
        logging.info("Reconnected to NATS server")

    async def _on_nats_closed(self):
        """Callback for NATS connection closure."""
        import logging
        logging.info("NATS connection closed")

    async def _setup_streams(self):
        """Set up JetStream streams with appropriate configuration."""
        import logging
        
        # Define streams with explicit configuration
        streams = [
            # Market data streams
            ("MARKET_LIVE", ["market.live.*"], {
                "max_msgs_per_subject": 1,  # Only keep latest message per subject
                "max_msgs": 10000,
                "max_bytes": 1024 * 1024 * 50,  # 50MB
                "discard": "old",  # Discard old messages when limits are reached
            }),
            ("MARKET_DAILY", ["market.daily.*"], {
                "max_msgs_per_subject": 30,  # Keep last 30 days
                "max_msgs": 10000,
                "max_bytes": 1024 * 1024 * 100,  # 100MB
                "discard": "old",
            }),
            ("MARKET_HISTORICAL", ["market.historical.*"], {
                "max_age": 24 * 60 * 60 * 1000 * 1000 * 1000,  # 24 hours in nanoseconds
                "max_msgs": 5000,
                "max_bytes": 1024 * 1024 * 200,  # 200MB
                "discard": "old",
            }),
            # Trading signals and recommendations
            ("SIGNALS", ["signals.*"], {
                "max_age": 24 * 60 * 60 * 1000 * 1000 * 1000,  # 24 hours
                "max_msgs": 10000,
                "max_bytes": 1024 * 1024 * 50,
                "discard": "old",
            }),
            ("RECOMMENDATIONS", ["recommendations.*"], {
                "max_age": 24 * 60 * 60 * 1000 * 1000 * 1000,  # 24 hours
                "max_msgs": 5000,
                "max_bytes": 1024 * 1024 * 50,
                "discard": "old",
            }),
            # Request streams
            ("REQUESTS", ["requests.*"], {
                "max_age": 5 * 60 * 1000 * 1000 * 1000,  # 5 minutes
                "max_msgs": 1000,
                "max_bytes": 1024 * 1024 * 10,  # 10MB
                "discard": "old",
            }),
        ]

        for stream_name, subjects, config in streams:
            try:
                # Create stream configuration
                stream_config = StreamConfig()
                stream_config.name = stream_name
                stream_config.subjects = subjects
                stream_config.retention = "limits"
                
                # Apply custom configuration
                for key, value in config.items():
                    setattr(stream_config, key, value)
                
                # Try to add the stream
                await self.js.add_stream(config=stream_config)
                logging.info(f"Created new stream: {stream_name}")
            except nats.js.errors.BadRequestError as e:
                if "already in use" in str(e):
                    logging.info(f"Stream {stream_name} already exists")
                    # Consider updating the stream config here if needed
                else:
                    logging.error(f"Error creating stream {stream_name}: {str(e)}")
            except Exception as e:
                logging.error(f"Unexpected error creating stream {stream_name}: {str(e)}")

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
        import logging
        
        if not self.js:
            logging.error("NATS connection not available for historical data request")
            raise RuntimeError("Not connected to NATS")

        subject = f"market.historical.request.{ticker}.{interval}.{days}"
        request_id = f"{datetime.now().timestamp():.6f}"
        
        request = {
            "ticker": ticker,
            "days": days,
            "interval": interval,
            "timestamp": str(datetime.now()),
            "request_id": request_id
        }
        
        payload = json.dumps(request).encode()
        
        # Add retry logic for the publish operation
        max_retries = 3
        backoff_base = 1  # seconds
        
        for attempt in range(max_retries):
            try:
                # Publish with acknowledgment and timeout
                ack = await self.js.publish(subject, payload, timeout=5.0)
                logging.info(f"Historical data request for {ticker} published successfully: stream={ack.stream}, seq={ack.seq}")
                return
            except Exception as e:
                if attempt < max_retries - 1:
                    # Calculate backoff with jitter to avoid thundering herd
                    backoff = backoff_base * (2 ** attempt) * (0.5 + 0.5 * asyncio.get_running_loop().time() % 1)
                    logging.warning(f"Failed to publish historical data request (attempt {attempt+1}/{max_retries}): {e}. Retrying in {backoff:.2f}s")
                    await asyncio.sleep(backoff)
                else:
                    logging.error(f"Failed to publish historical data request after {max_retries} attempts: {e}")
                    raise

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

        try:
            # Replace any wildcard or special characters with 'all' for wildcard case
            if ticker == '*':
                safe_ticker = 'all'
            else:
                # Only allow alphanumeric and hyphen in durable names
                safe_ticker = re.sub(r'[^a-zA-Z0-9-]', '-', ticker)
                
            durable_name = f"market-data-consumer-{safe_ticker}"
            sub = await self.js.subscribe(subject, cb=message_handler, durable=durable_name)
            self.subscriptions[subject] = sub
            return sub
        except Exception as e:
            import logging
            logging.error(f"Failed to subscribe to market data for {ticker}: {e}")
            # Return None instead of raising the exception
            return None

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

        try:
            # Replace any wildcard or special characters with 'all' for wildcard case
            if ticker == '*':
                safe_ticker = 'all'
            else:
                # Only allow alphanumeric and hyphen in durable names
                safe_ticker = re.sub(r'[^a-zA-Z0-9-]', '-', ticker)
                
            durable_name = f"historical-data-consumer-{safe_ticker}"
            sub = await self.js.subscribe(subject, cb=message_handler, durable=durable_name)
            self.subscriptions[subject] = sub
            return sub
        except Exception as e:
            import logging
            logging.error(f"Failed to subscribe to historical data for {ticker}: {e}")
            # Return None instead of raising the exception
            return None

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

        try:
            # Replace any wildcard or special characters with 'all' for wildcard case
            if ticker == '*':
                safe_ticker = 'all'
            else:
                # Only allow alphanumeric and hyphen in durable names
                safe_ticker = re.sub(r'[^a-zA-Z0-9-]', '-', ticker)
                
            durable_name = f"signals-consumer-{safe_ticker}"
            sub = await self.js.subscribe(subject, cb=message_handler, durable=durable_name)
            self.subscriptions[subject] = sub
            return sub
        except Exception as e:
            import logging
            logging.error(f"Failed to subscribe to signals for {ticker}: {e}")
            # Return None instead of raising the exception
            return None

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

        try:
            # Replace any wildcard or special characters with 'all' for wildcard case
            if ticker == '*':
                safe_ticker = 'all'
            else:
                # Only allow alphanumeric and hyphen in durable names
                safe_ticker = re.sub(r'[^a-zA-Z0-9-]', '-', ticker)
                
            durable_name = f"recommendations-consumer-{safe_ticker}"
            sub = await self.js.subscribe(subject, cb=message_handler, durable=durable_name)
            self.subscriptions[subject] = sub
            return sub
        except Exception as e:
            import logging
            logging.error(f"Failed to subscribe to recommendations for {ticker}: {e}")
            # Return None instead of raising the exception
            return None

    async def close(self) -> None:
        """Close all subscriptions and connection."""
        for sub in self.subscriptions.values():
            await sub.unsubscribe()

        if self.nc:
            await self.nc.close()