import os
import grpc
import pandas as pd
from concurrent import futures
from datetime import datetime
import logging
import json
import sys

# Find the proto files directory - they should be in the 'proto' directory
# at the project root
current_dir = os.path.dirname(os.path.abspath(__file__))
project_root = os.path.dirname(current_dir)
proto_dir = os.path.join(project_root, 'proto')

# Add the project root and proto directory to the Python path
sys.path.insert(0, project_root)
sys.path.insert(0, proto_dir)

try:
    # Try importing the generated proto files
    import trading_pb2
    import trading_pb2_grpc
except ImportError:
    logging.error("Could not import trading_pb2 or trading_pb2_grpc. Make sure protoc has been run.")
    raise

# Import trading system components
from data import AlphaVantageDataProvider, CachingDataProvider
from strategy import RedCandleStrategy
from analysis import OptionsRecommender, StrategyBacktester
from core import StrategyRunner

# Set up logging
logging.basicConfig(
        level=logging.INFO,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

class TradingServiceServicer(trading_pb2_grpc.TradingServiceServicer):
    """Implementation of the TradingService gRPC server."""

    def __init__(self):
        """Initialize the trading service with required components."""
        # Create data provider
        api_key = os.getenv('ALPHA_VANTAGE_API_KEY', '')
        self.base_provider = AlphaVantageDataProvider(interval="5min", api_key=api_key)

        # Wrap with caching provider
        cache_dir = os.getenv('CACHE_DIR', '/app/data_cache')
        self.data_provider = CachingDataProvider(
                data_provider=self.base_provider,
                cache_dir=cache_dir,
                cache_expiry_days=1
        )

        # Initialize strategy, recommender, and runner components
        self.strategies = {
            'RedCandle': RedCandleStrategy()
        }

        self.recommender = OptionsRecommender(
                min_delta=0.30,
                max_delta=0.60,
                target_delta=0.45
        )

        self.runner = StrategyRunner(
                data_provider=self.data_provider,
                strategy=self.strategies['RedCandle'],  # Default strategy
                recommender=self.recommender,
                visualizer=None  # No visualizer for server mode
        )

        logger.info("Trading service initialized")

    def GetHistoricalData(self, request, context):
        """Get historical data for a ticker."""
        try:
            ticker = request.ticker
            days = request.days
            interval = request.interval if request.interval else '15min'  # Default to 15min if not specified

            logger.info(f"Getting historical data for {ticker}, {days} days, interval {interval}")

            # Get data from provider
            df = self.data_provider.get_historical_data(ticker, days, interval)

            if df is None:
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"Failed to get historical data for {ticker}")
                return trading_pb2.HistoricalDataResponse()

            # Convert to response format
            response = trading_pb2.HistoricalDataResponse()

            for index, row in df.iterrows():
                candle = response.candles.add()
                candle.date = index.strftime('%Y-%m-%d %H:%M:%S')
                candle.open = float(row['open'])
                candle.high = float(row['high'])
                candle.low = float(row['low'])
                candle.close = float(row['close'])
                candle.volume = int(row['volume'])

            return response

        except Exception as e:
            logger.error(f"Error in GetHistoricalData: {str(e)}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return trading_pb2.HistoricalDataResponse()

    def GenerateSignals(self, request, context):
        """Generate trading signals based on a strategy."""
        try:
            ticker = request.ticker
            days = request.days
            strategy_name = request.strategy
            interval = request.interval if request.interval else '15min'  # Default to 15min if not specified

            logger.info(f"Generating signals for {ticker}, strategy: {strategy_name}, interval: {interval}")

            # Check if strategy exists
            if strategy_name not in self.strategies:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details(f"Strategy {strategy_name} not found")
                return trading_pb2.SignalResponse()

            # Get data and generate signals
            df = self.data_provider.get_historical_data(ticker, days, interval)

            if df is None:
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"Failed to get historical data for {ticker}")
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
                signal.date = date.strftime('%Y-%m-%d %H:%M:%S')
                signal.signal_type = row['signal_type']
                signal.entry_price = float(row['close'])

                if not pd.isna(row['stoploss']):
                    signal.stoploss = float(row['stoploss'])

            return response

        except Exception as e:
            logger.error(f"Error in GenerateSignals: {str(e)}")
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

            logger.info(f"Running backtest for {ticker}, strategy: {strategy_name}, interval: {interval}")

            # Check if strategy exists
            if strategy_name not in self.strategies:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details(f"Strategy {strategy_name} not found")
                return trading_pb2.BacktestResponse()

            # Get data and generate signals
            df = self.data_provider.get_historical_data(ticker, days, interval)
            if df is None:
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"Failed to get historical data for {ticker}")
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

            # Add results to the map properly
            for test_name, stats in summary.items():
                # Access the map entry - this creates a default entry if it doesn't exist
                result_entry = response.results[test_name]

                # Now set each field individually
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
            logger.error(f"Error in RunBacktest: {str(e)}")
            import traceback
            logger.error(traceback.format_exc())  # Add stack trace for better debugging
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return trading_pb2.BacktestResponse()

    def GetOptionsRecommendations(self, request, context):
        """Get options recommendations for a ticker."""
        try:
            ticker = request.ticker
            days = request.days
            strategy_name = request.strategy
            interval = request.interval if request.interval else '15min'  # Default to 15min if not specified

            logger.info(f"Getting options recommendations for {ticker}, strategy: {strategy_name}, interval: {interval}")

            # Check if strategy exists
            if strategy_name not in self.strategies:
                context.set_code(grpc.StatusCode.INVALID_ARGUMENT)
                context.set_details(f"Strategy {strategy_name} not found")
                return trading_pb2.RecommendationResponse()

            # Update runner with selected strategy
            self.runner.strategy = self.strategies[strategy_name]

            # Run strategy and get recommendations
            df, recommendations = self.runner.run(
                    ticker=ticker,
                    days=days,
                    visualize=False,
                    save_recommendations=False,
                    interval=interval
            )

            if df is None:
                context.set_code(grpc.StatusCode.INTERNAL)
                context.set_details(f"Failed to get historical data for {ticker}")
                return trading_pb2.RecommendationResponse()

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
            logger.error(f"Error in GetOptionsRecommendations: {str(e)}")
            context.set_code(grpc.StatusCode.INTERNAL)
            context.set_details(f"Internal error: {str(e)}")
            return trading_pb2.RecommendationResponse()


def serve():
    """Start the gRPC server."""
    # Get server port from environment or use default
    port = os.getenv('GRPC_PORT', '50052')
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))

    # Add the servicer to the server
    trading_pb2_grpc.add_TradingServiceServicer_to_server(
            TradingServiceServicer(), server
    )

    # Enable reflection for easier debugging and testing
    # This allows tools like grpcurl to introspect the service
    from grpc_reflection.v1alpha import reflection
    service_names = (
        trading_pb2.DESCRIPTOR.services_by_name['TradingService'].full_name,
        reflection.SERVICE_NAME,
    )
    reflection.enable_server_reflection(service_names, server)

    # Start listening
    server.add_insecure_port(f'[::]:{port}')
    server.start()

    logger.info(f"gRPC server running on port {port} with reflection enabled")

    # Keep the server running
    server.wait_for_termination()


if __name__ == '__main__':
    serve()