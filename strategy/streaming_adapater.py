# strategy/streaming_adapter.py
import asyncio
from typing import Dict, Any, List, Optional
import pandas as pd
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
            await self.event_client.subscribe_market_data(
                    ticker,
                    lambda data: self._handle_market_data(ticker, data)
            )

    async def _handle_market_data(self, ticker: str, data: Dict[str, Any]):
        """Process incoming market data."""
        # Add to buffer
        self.data_buffers[ticker].append(data)

        # Trim buffer if needed
        if len(self.data_buffers[ticker]) > self.buffer_size:
            self.data_buffers[ticker] = self.data_buffers[ticker][-self.buffer_size:]

        # Convert buffer to DataFrame
        df = pd.DataFrame(self.data_buffers[ticker])

        # Run strategy on updated data
        signals_df = self.strategy.generate_signals(df)

        # Check for new signals
        if not signals_df.empty and 'entry_signal' in signals_df.columns:
            entry_signals = signals_df[signals_df['entry_signal']]

            # Publish new signals
            for _, row in entry_signals.iterrows():
                signal_data = {
                    'ticker': ticker,
                    'timestamp': row.name.isoformat() if hasattr(row.name, 'isoformat') else str(row.name),
                    'strategy': self.strategy.__class__.__name__,
                    'signal_type': row.get('signal_type', 'UNKNOWN'),
                    'entry_price': float(row['close']),
                    'stop_loss': float(row['stop_loss']) if 'stop_loss' in row else None,
                    'metadata': {
                        k: str(v) for k, v in row.items()
                        if k not in ['close', 'signal_type', 'entry_signal', 'stop_loss']
                    }
                }
                await self.event_client.publish_signal(ticker, signal_data)

    async def stop(self):
        """Stop the adapter."""
        self.running = False
        await self.event_client.close()