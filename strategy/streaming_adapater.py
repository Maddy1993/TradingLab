# strategy/streaming_adapter.py
import asyncio
import json
from typing import Dict, Any, List, Optional, Callable
import pandas as pd
from datetime import datetime
from events.client import EventClient

class StreamingStrategyAdapter:
    """Adapter to connect existing strategies with streaming data."""

    def __init__(self, strategy, event_client: EventClient, buffer_size: int = 100):
        """
        Initialize the adapter.

        Args:
            strategy: The strategy instance to run
            event_client: The event client for message publishing/subscribing
            buffer_size: Number of candles to keep in memory
        """
        self.strategy = strategy
        self.event_client = event_client
        self.data_buffers = {}  # ticker -> {timeframe -> list of candles}
        self.running = False
        self.buffer_size = buffer_size
        self.signal_callbacks = []
        self.tasks = []

    async def register_signal_callback(self, callback: Callable):
        """Register a callback function to be called when signals are generated."""
        self.signal_callbacks.append(callback)

    async def start(self, tickers: List[str], timeframes: List[str] = ["15M"]):
        """
        Start the adapter for the given tickers and timeframes.

        Args:
            tickers: List of ticker symbols to process
            timeframes: List of timeframes to process (default: ["15M"])
        """
        self.running = True

        # Initialize data buffers
        for ticker in tickers:
            if ticker not in self.data_buffers:
                self.data_buffers[ticker] = {}

            for timeframe in timeframes:
                if timeframe not in self.data_buffers[ticker]:
                    self.data_buffers[ticker][timeframe] = []

                # Subscribe to market data - both live and daily
                live_task = asyncio.create_task(
                        self._subscribe_and_process_live_data(ticker, timeframe)
                )
                daily_task = asyncio.create_task(
                        self._subscribe_and_process_daily_data(ticker, timeframe)
                )
                self.tasks.extend([live_task, daily_task])

    async def _subscribe_and_process_live_data(self, ticker: str, timeframe: str):
        """Subscribe to live market data and process it."""
        # Need to fetch historical data first to initialize
        await self._fetch_historical_data(ticker, timeframe, 10)  # Last 10 days

        # Now subscribe to live updates
        async def handle_live_data(data):
            await self._process_market_data(ticker, timeframe, data)

        await self.event_client.subscribe_market_live_data(ticker, handle_live_data)

    async def _subscribe_and_process_daily_data(self, ticker: str, timeframe: str):
        """Subscribe to daily market data and process it."""
        async def handle_daily_data(data):
            if timeframe.upper() == "1D" or timeframe.upper() == "1DAY":
                # Only process daily data for daily timeframe
                await self._process_market_data(ticker, timeframe, data)
            else:
                # For other timeframes, daily data is just informational
                print(f"Received daily data for {ticker}, but using timeframe {timeframe}")

        await self.event_client.subscribe_market_daily_data(ticker, handle_daily_data)

    async def _fetch_historical_data(self, ticker: str, timeframe: str, days: int):
        """Fetch historical data to initialize the buffer."""
        # Create a future to wait for the data
        historical_data_future = asyncio.Future()
        chunks_received = []
        total_chunks = 0

        # Handler for historical data responses
        async def handle_historical_data(data):
            nonlocal chunks_received, total_chunks

            # Extract the data array and metadata
            if 'data' not in data or 'metadata' not in data:
                print(f"Invalid historical data format: {data}")
                return

            candles = data['data']
            metadata = data['metadata']

            # Check if this is part of a multi-chunk response
            chunk = metadata.get('chunk', 1)
            total_chunks = metadata.get('total_chunks', 1)

            print(f"Received historical data chunk {chunk}/{total_chunks} for {ticker} ({timeframe}, {days} days)")

            # Add data to our chunks
            chunks_received.append((chunk, candles))

            # Check if we have all chunks
            if len(chunks_received) == total_chunks:
                # Sort chunks by chunk number
                chunks_received.sort(key=lambda x: x[0])

                # Combine all chunks
                all_data = []
                for _, chunk_data in chunks_received:
                    all_data.extend(chunk_data)

                # Resolve the future with the combined data
                if not historical_data_future.done():
                    historical_data_future.set_result(all_data)

        # Subscribe to historical data
        subscription = await self.event_client.subscribe_historical_data(
                ticker, timeframe, days, handle_historical_data
        )

        # Request the historical data
        await self.event_client.request_historical_data(
                ticker, timeframe, days,
                {"source": "strategy_adapter", "timestamp": datetime.now().isoformat()}
        )

        # Wait for the data with a timeout
        try:
            data = await asyncio.wait_for(historical_data_future, timeout=30.0)
            print(f"Received {len(data)} historical candles for {ticker} ({timeframe}, {days} days)")

            # Update the buffer with historical data
            self.data_buffers[ticker][timeframe] = data[-self.buffer_size:]

            # Cancel the subscription now that we have the data
            await subscription.unsubscribe()

            return data
        except asyncio.TimeoutError:
            print(f"Timeout waiting for historical data for {ticker} ({timeframe}, {days} days)")
            await subscription.unsubscribe()
            return []

    async def _process_market_data(self, ticker: str, timeframe: str, data: Dict[str, Any]):
        """Process incoming market data."""
        if not self.running:
            return

        # Add to buffer
        self.data_buffers[ticker][timeframe].append(data)

        # Trim buffer if needed
        if len(self.data_buffers[ticker][timeframe]) > self.buffer_size:
            self.data_buffers[ticker][timeframe] = self.data_buffers[ticker][timeframe][-self.buffer_size:]

        # Convert buffer to DataFrame
        df = pd.DataFrame(self.data_buffers[ticker][timeframe])

        # Ensure datetime index if not already
        if 'timestamp' in df.columns and not isinstance(df.index, pd.DatetimeIndex):
            try:
                df['date'] = pd.to_datetime(df['timestamp'])
                df = df.set_index('date')
            except:
                print(f"Failed to convert timestamp to datetime: {df['timestamp'].iloc[0]}")

        # Run strategy on updated data
        try:
            signals_df = self.strategy.generate_signals(df)
        except Exception as e:
            print(f"Error running strategy on {ticker} ({timeframe}): {e}")
            return

        # Check for new signals
        if not signals_df.empty and 'entry_signal' in signals_df.columns:
            entry_signals = signals_df[signals_df['entry_signal']]

            # Publish new signals
            for idx, row in entry_signals.iterrows():
                # Check if this is a new signal (last row)
                if idx == entry_signals.index[-1]:
                    signal_data = {
                        'ticker': ticker,
                        'timestamp': idx.isoformat() if hasattr(idx, 'isoformat') else str(idx),
                        'strategy': self.strategy.__class__.__name__,
                        'signal_type': row.get('signal_type', 'UNKNOWN'),
                        'entry_price': float(row['close']),
                        'stoploss': float(row['stoploss']) if 'stoploss' in row else None,
                        'timeframe': timeframe,
                        'metadata': {
                            k: str(v) for k, v in row.items()
                            if k not in ['close', 'signal_type', 'entry_signal', 'stoploss']
                        }
                    }

                    # Publish the signal
                    await self.event_client.publish_signal(ticker, signal_data)
                    print(f"Published {signal_data['signal_type']} signal for {ticker} ({timeframe})")

                    # Notify callbacks
                    for callback in self.signal_callbacks:
                        try:
                            await callback(signal_data)
                        except Exception as e:
                            print(f"Error in signal callback: {e}")

    async def stop(self):
        """Stop the adapter."""
        self.running = False

        # Cancel all tasks
        for task in self.tasks:
            task.cancel()

        # Wait for tasks to complete
        if self.tasks:
            await asyncio.gather(*self.tasks, return_exceptions=True)

        # Clear the tasks list
        self.tasks = []

        # Close the event client connection
        await self.event_client.close()