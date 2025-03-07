import os
import sys
import logging
import asyncio
import pandas as pd
import grpc
import json
from datetime import datetime, timedelta

# Import local modules
from strategy import RedCandleStrategy, StreamingStrategyAdapter
from analysis import OptionsRecommender, StrategyBacktester
from events.client import EventClient
from utils.timezone import now, format_datetime, parse_datetime

# Import generated proto files
from proto import trading_pb2
from proto import trading_pb2_grpc

class TradingServiceServicer(trading_pb2_grpc.TradingServiceServicer):
    """Implementation of the TradingService gRPC server."""

    def __init__(self):
        """Initialize the trading service with required components."""
        # Initialize strategy components
        self.strategies = {
            'RedCandle': RedCandleStrategy()
        }

        self.recommender = OptionsRecommender(
                min_delta=0.30,
                max_delta=0.60,
                target_delta=0.45
        )

        # Initialize event client as None (will be initialized async)
        self.event_client = None
        self.adapters = {}
        self.active_tickers = []
        self.historical_data_cache = {}

        # Default watchlist tickers
        self.default_tickers = ['SPY', 'AAPL', 'MSFT', 'GOOGL', 'AMZN']

        # Custom watchlist from environment
        if custom_tickers := os.getenv('WATCH_TICKERS'):
            self.default_tickers = custom_tickers.split(',')

        logging.info(f"Default watchlist: {self.default_tickers}")

    async def init_event_client(self):
        """Initialize the event client asynchronously."""
        try:
            nats_url = os.getenv('NATS_URL', 'nats://nats:4222')
            logging.info(f"Connecting to NATS at {nats_url}")

            self.event_client = EventClient(nats_url)
            await self.event_client.connect()

            # Initialize streaming adapters for each strategy
            for name, strategy in self.strategies.items():
                logging.info(f"Initializing streaming adapter for {name} strategy")
                adapter = StreamingStrategyAdapter(strategy, self.event_client)
                # Start with default watchlist
                await adapter.start(self.default_tickers)
                self.adapters[name] = adapter

            # Set up subscription for historical data responses
            await self._setup_historical_data_subscription()

            logging.info("Event client and strategy adapters initialized successfully")
        except Exception as e:
            logging.error(f"Failed to initialize event client: {e}")
            # Retry after delay
            await asyncio.sleep(5)
            await self.init_event_client()

    async def _setup_historical_data_subscription(self):
        """Subscribe to historical data responses."""
        if not self.event_client:
            return

        async def handle_historical_data(data):
            ticker = data.get('ticker')
            timeframe = data.get('interval')
            days = data.get('days')

            # Create cache key
            cache_key = f"{ticker}_{timeframe}_{days}"

            # Store in cache
            self.historical_data_cache[cache_key] = data
            logging.info(f"Received historical data for {cache_key}")

        # Subscribe to historical data responses
        await self.event_client.subscribe_market_historical('*', handle_historical_data)
        logging.info("Subscribed to historical data responses")

    async def _get_historical_data(self, ticker, days, interval='15min', timeout=10):
        """Get historical data through the event system."""
        if not self.event_client:
            raise ValueError("Event client not initialized")

        # Create cache key
        cache_key = f"{ticker}_{interval}_{days}"

        # Check cache first
        if cache_key in self.historical_data_cache:
            return self.historical_data_cache[cache_key]

        # Request historical data
        request = {
            'ticker': ticker,
            'days': days,
            'interval': interval
        }

        # Publish request
        await self.event_client.request_historical_data(ticker, days, interval)

        # Wait for response with timeout
        start_time = now()
        while (now() - start_time).total_seconds() < timeout:
            if cache_key in self.historical_data_cache:
                return self.historical_data_cache[cache_key]
            await asyncio.sleep(0.1)

        raise TimeoutError(f"Timeout waiting for historical data for {cache_key}")

    def GetHistoricalData(self, request, context):
        """Get historical data for a ticker."""
        try:
            ticker = request.ticker
            days = request.days
            interval = request.interval if request.interval else '15min'

            logging.info(f"GetHistoricalData request for {ticker}, {days} days, interval {interval}")

            # Create a new event loop for this thread if one doesn't exist
            try:
                loop = asyncio.get_event_loop()
            except RuntimeError:
                # "There is no current event loop in thread" error
                loop = asyncio.new_event_loop()
                asyncio.set_event_loop(loop)

            try:
                data = loop.run_until_complete(self._get_historical_data(ticker, days, interval))
                df = pd.DataFrame(data)
            except (TimeoutError, ValueError) as e:
                logging.warning(f"Failed to get data from event system: {e}")
                context.set_code(grpc.StatusCode.UNAVAILABLE)
                context.set_details(f"Historical data for {ticker} is currently unavailable: {e}")
                return trading_pb2.HistoricalDataResponse()
            except Exception as e:
                logging.error(f"Error retrieving historical data: {e}")
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"Error retrieving historical data: {e}")
                return trading_pb2.HistoricalDataResponse()

            if df.empty:
                context.set_code(grpc.StatusCode.NOT_FOUND)
                context.set_details(f"No historical data found for {ticker}")
                return trading_pb2.HistoricalDataResponse()

            # Convert to response format
            response = trading_pb2.HistoricalDataResponse()

            for index, row in df.iterrows():
                candle = response.candles.add()
                candle.date = format_datetime(index, '%Y-%m-%d %H:%M:%S')
                candle.open = float(row['open'])
                candle.high = float(row['high'])
                candle.low = float(row['low'])
                candle.close = float(row['close'])
                candle.volume = int(row['volume'])

            return response

        except Exception as e:
            logging.error(f"Error in GetHistoricalData: {str(e)}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return trading_pb2.HistoricalDataResponse()

    def GenerateSignals(self, request, context):
        """Generate trading signals based on a strategy."""
        try:
            ticker = request.ticker
            days = request.days
            strategy_name = request.strategy
            interval = request.interval if request.interval else '15min'

            logging.info(f"GenerateSignals request for {ticker}, strategy: {strategy_name}, interval: {interval}")

            # Check if strategy exists
            if strategy_name not in self.strategies:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details(f"Strategy {strategy_name} not found")
                return trading_pb2.SignalResponse()

            # Get data and generate signals
            # In fully event-driven system, we would subscribe to signals from the event stream
            loop = asyncio.get_event_loop()
            try:
                data = loop.run_until_complete(self._get_historical_data(ticker, days, interval))
                df = pd.DataFrame(data)
            except (TimeoutError, ValueError) as e:
                logging.warning(f"Failed to get data from event system: {e}")
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"Failed to get historical data: {e}")
                return trading_pb2.SignalResponse()

            # Apply strategy
            strategy = self.strategies[strategy_name]
            df = strategy.generate_signals(df)

            # Filter for entry signals only
            entry_signals = df[df['entry_signal']]

            # Convert to response format
            response = trading_pb2.SignalResponse()

            for date, row in entry_signals.iterrows():
                signal = response.signals.add()
                signal.date = format_datetime(date, '%Y-%m-%d %H:%M:%S')
                signal.signal_type = row['signal_type']
                signal.entry_price = float(row['close'])

                if 'stoploss' in row and not pd.isna(row['stoploss']):
                    signal.stoploss = float(row['stoploss'])

            return response

        except Exception as e:
            logging.error(f"Error in GenerateSignals: {str(e)}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return trading_pb2.SignalResponse()

    def RunBacktest(self, request, context):
        """Run a backtest for a specific strategy."""
        try:
            # Extract parameters
            ticker = request.ticker
            days = request.days
            strategy_name = request.strategy
            interval = request.interval if request.interval else '15min'

            # Safely extract lists from repeated fields
            profit_targets = [pt for pt in request.profit_targets] if request.profit_targets else None
            risk_reward_ratios = [rr for rr in request.risk_reward_ratios] if request.risk_reward_ratios else None
            profit_targets_dollar = [pd for pd in request.profit_targets_dollar] if request.profit_targets_dollar else None

            logging.info(f"RunBacktest request for {ticker}, strategy: {strategy_name}, interval: {interval}")

            # Check if strategy exists
            if strategy_name not in self.strategies:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details(f"Strategy {strategy_name} not found")
                return trading_pb2.BacktestResponse()

            # Get data and generate signals
            loop = asyncio.get_event_loop()
            try:
                data = loop.run_until_complete(self._get_historical_data(ticker, days, interval))
                df = pd.DataFrame(data)
            except (TimeoutError, ValueError) as e:
                logging.warning(f"Failed to get data from event system: {e}")
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"Failed to get historical data: {e}")
                return trading_pb2.BacktestResponse()

            # Apply strategy
            strategy = self.strategies[strategy_name]
            df = strategy.generate_signals(df)

            # Run backtest
            backtester = StrategyBacktester(strategy_name=strategy_name)
            backtester.backtest(
                    df=df,
                    profit_targets=profit_targets,
                    risk_reward_ratios=risk_reward_ratios,
                    profit_targets_dollar=profit_targets_dollar,
                    contracts=2,
                    contract_value=50
            )

            # Get summary stats
            summary = backtester.get_summary_stats()

            # Create a new response
            response = trading_pb2.BacktestResponse()

            # Add results to the map
            for test_name, stats in summary.items():
                # Access the map entry - this creates a default entry if it doesn't exist
                result_entry = response.results[test_name]

                # Set each field individually
                result_entry.win_rate = float(stats['win_rate'])

                # Handle infinity for profit_factor
                pf = stats['profit_factor']
                result_entry.profit_factor = 999999.0 if pf == float('inf') else float(pf)

                result_entry.total_return = float(stats['total_return'])
                result_entry.total_return_pct = float(stats.get('total_return_pct', 0))
                result_entry.total_trades = int(stats['total_trades'])
                result_entry.winning_trades = int(stats['winning_trades'])
                result_entry.losing_trades = int(stats['losing_trades'])
                result_entry.max_drawdown = float(stats.get('max_drawdown', 0))
                result_entry.max_drawdown_pct = float(stats.get('max_drawdown_pct', 0))

            return response

        except Exception as e:
            logging.error(f"Error in RunBacktest: {str(e)}")
            import traceback
            logging.error(traceback.format_exc())
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return trading_pb2.BacktestResponse()

    def GetOptionsRecommendations(self, request, context):
        """Get options recommendations for a ticker."""
        try:
            ticker = request.ticker
            days = request.days
            strategy_name = request.strategy
            interval = request.interval if request.interval else '15min'

            logging.info(f"GetOptionsRecommendations request for {ticker}, strategy: {strategy_name}, interval: {interval}")

            # Check if strategy exists
            if strategy_name not in self.strategies:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details(f"Strategy {strategy_name} not found")
                return trading_pb2.RecommendationResponse()

            # Get data and generate signals
            loop = asyncio.get_event_loop()
            try:
                data = loop.run_until_complete(self._get_historical_data(ticker, days, interval))
                df = pd.DataFrame(data)
            except (TimeoutError, ValueError) as e:
                logging.warning(f"Failed to get data from event system: {e}")
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"Failed to get historical data: {e}")
                return trading_pb2.RecommendationResponse()

            # Apply strategy
            strategy = self.strategies[strategy_name]
            df = strategy.generate_signals(df)

            # Generate options recommendations
            # Note: In an event-driven system, this might be provided by a dedicated service
            # For now, we'll maintain backward compatibility
            recommendations = self.recommender.generate_recommendations(df, None)

            # Convert to response format
            response = trading_pb2.RecommendationResponse()

            for rec in recommendations:
                recommendation = response.recommendations.add()
                recommendation.date = rec['date']
                recommendation.signal_type = rec['signal_type']
                recommendation.stock_price = float(rec['stock_price'])
                recommendation.stoploss = float(rec['stoploss'])
                recommendation.option_type = rec['option_type']
                recommendation.strike = float(rec['strike'])
                recommendation.expiration = rec['expiration']
                recommendation.delta = float(rec['delta'])

                # Handle optional fields
                if rec.get('iv') is not None:
                    recommendation.iv = float(rec['iv'])

                if rec.get('price') is not None:
                    recommendation.price = float(rec['price'])

            return response

        except Exception as e:
            logging.error(f"Error in GetOptionsRecommendations: {str(e)}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return trading_pb2.RecommendationResponse()