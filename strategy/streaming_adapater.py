# strategy/streaming_adapter.py
import asyncio
import logging
from typing import Dict, Any, List, Optional
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
        self.data_buffers = {}  # ticker -> list of candles
        self.running = False
        self.buffer_size = buffer_size
        self.subscribers = {}  # ticker -> subscription

    async def start(self, tickers: List[str]):
        """
        Start the adapter for the given tickers.

        Args:
            tickers: List of ticker symbols to process
        """
        self.running = True

        # Initialize data buffers
        for ticker in tickers:
            self.data_buffers[ticker] = []

            # Subscribe to market data
            try:
                subscription = await self.event_client.subscribe_market_data(
                        ticker,
                        callback=lambda data, t=ticker: self._handle_market_data(t, data)
                )
                if subscription:
                    self.subscribers[ticker] = subscription
                    logging.info(f"Subscribed to market data for {ticker}")
                else:
                    logging.warning(f"Subscription for {ticker} returned None, possibly already subscribed")
            except Exception as e:
                logging.error(f"Failed to subscribe to market data for {ticker}: {e}")

    async def _handle_market_data(self, ticker: str, data: Dict[str, Any]):
        """Process incoming market data."""
        # Add to buffer
        self.data_buffers[ticker].append(data)

        # Trim buffer if needed
        if len(self.data_buffers[ticker]) > self.buffer_size:
            self.data_buffers[ticker] = self.data_buffers[ticker][-self.buffer_size:]

        # Only process if we have enough data
        if len(self.data_buffers[ticker]) < 3:
            return

        # Convert buffer to DataFrame
        try:
            df = pd.DataFrame(self.data_buffers[ticker])

            # Convert timestamp string to datetime if needed
            if 'timestamp' in df.columns and df['timestamp'].dtype == 'object':
                try:
                    # First try ISO format with more flexible parsing
                    df['timestamp'] = pd.to_datetime(df['timestamp'], format='ISO8601')
                except Exception as e:
                    logging.warning(f"Error parsing timestamps in ISO8601 format: {e}")
                    try:
                        # Fall back to mixed format with flexible parsing
                        df['timestamp'] = pd.to_datetime(df['timestamp'], format='mixed')
                    except Exception as e:
                        logging.error(f"Failed to parse timestamps: {e}")
                        return

            # Set index to timestamp if available
            if 'timestamp' in df.columns:
                df = df.set_index('timestamp')

            # Rename columns to match expected names if needed
            column_mapping = {
                'p': 'close',
                'o': 'open',
                'h': 'high',
                'l': 'low',
                'v': 'volume',
                'price': 'close'
            }
            df = df.rename(columns={col: mapped for col, mapped in column_mapping.items()
                                    if col in df.columns and mapped not in df.columns})

            # Ensure all required columns exist
            required_columns = ['open', 'high', 'low', 'close', 'volume']
            missing_columns = [col for col in required_columns if col not in df.columns]

            if missing_columns:
                logging.warning(f"Missing columns in market data for {ticker}: {missing_columns}")
                # Could try to derive missing columns if needed
                return

            # Sort by index
            df = df.sort_index()

        except Exception as e:
            logging.error(f"Error processing market data for {ticker}: {e}")
            return

        try:
            # Run strategy on updated data
            signals_df = self.strategy.generate_signals(df)

            # Check for new signals
            if 'entry_signal' in signals_df.columns:
                entry_signals = signals_df[signals_df['entry_signal']]

                # Publish new signals
                for idx, row in entry_signals.iterrows():
                    # Skip if already processed (using index as timestamp)
                    if self._is_signal_already_processed(ticker, idx):
                        continue

                    # Prepare signal data
                    signal_data = {
                        'ticker': ticker,
                        'timestamp': idx.isoformat() if hasattr(idx, 'isoformat') else str(idx),
                        'strategy': self.strategy.__class__.__name__,
                        'signal_type': row.get('signal_type', 'UNKNOWN'),
                        'entry_price': float(row['close']),
                        'stoploss': float(row['stoploss']) if 'stoploss' in row else None,
                        'metadata': {
                            k: str(v) for k, v in row.items()
                            if k not in ['close', 'signal_type', 'entry_signal', 'stoploss']

                        }
                    }

                    # Publish the signal
                    try:
                        await self.event_client.publish_signal(ticker, signal_data)
                        logging.info(f"Published {signal_data['signal_type']} signal for {ticker} at {signal_data['timestamp']}")
                        # Store signal to avoid duplicate processing
                        self._mark_signal_processed(ticker, idx)
                    except Exception as e:
                        logging.error(f"Failed to publish signal for {ticker}: {e}")
        except Exception as e:
            logging.error(f"Error generating signals for {ticker}: {e}")

    def _is_signal_already_processed(self, ticker: str, timestamp) -> bool:
        """Check if a signal has already been processed."""
        # This is a simple implementation - in production you'd want a more robust solution
        # such as storing processed signals in a database or using NATS durable subscriptions
        key = f"{ticker}_{timestamp}"
        return key in getattr(self, '_processed_signals', set())

    def _mark_signal_processed(self, ticker: str, timestamp) -> None:
        """Mark a signal as processed to avoid duplicates."""
        if not hasattr(self, '_processed_signals'):
            self._processed_signals = set()

        key = f"{ticker}_{timestamp}"
        self._processed_signals.add(key)

        # Limit the size of the processed signals set
        if len(self._processed_signals) > 1000:
            # Remove oldest items (simple approach - not truly FIFO but works for this purpose)
            self._processed_signals = set(list(self._processed_signals)[-500:])

    async def add_ticker(self, ticker: str) -> None:
        """Add a new ticker to the adapter."""
        if ticker in self.data_buffers:
            logging.info(f"Ticker {ticker} already being monitored")
            return

        self.data_buffers[ticker] = []

        # Subscribe to market data
        try:
            subscription = await self.event_client.subscribe_market_data(
                    ticker,
                    callback=lambda data, t=ticker: self._handle_market_data(t, data)
            )
            if subscription:
                self.subscribers[ticker] = subscription
                logging.info(f"Started monitoring {ticker}")
            else:
                logging.warning(f"Subscription for {ticker} returned None, possibly already subscribed")
        except Exception as e:
            logging.error(f"Failed to subscribe to market data for {ticker}: {e}")

    async def remove_ticker(self, ticker: str) -> None:
        """Remove a ticker from the adapter."""
        if ticker not in self.data_buffers:
            return

        # Clean up
        if ticker in self.subscribers:
            await self.subscribers[ticker].unsubscribe()
            del self.subscribers[ticker]

        del self.data_buffers[ticker]
        logging.info(f"Stopped monitoring {ticker}")

    async def stop(self) -> None:
        """Stop the adapter."""
        self.running = False

        # Clean up subscriptions
        for ticker, subscription in self.subscribers.items():
            try:
                await subscription.unsubscribe()
            except Exception as e:
                logging.error(f"Error unsubscribing from {ticker}: {e}")

        self.subscribers.clear()
        self.data_buffers.clear()