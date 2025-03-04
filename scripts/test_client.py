#!/usr/bin/env python3
"""
Simple test client for the TradingLab gRPC service.
This script can be used to verify that the server is functioning correctly.

Usage:
  python test_client.py [host:port] [test_type]

Arguments:
  host:port - The address of the gRPC server (default: localhost:50052)
  test_type - Type of test to run: all, historical, signals, backtest, recommendations (default: all)

Example:
  python test_client.py localhost:50052 all
  python test_client.py tradinglab-service:50052 backtest
"""
import sys
import os
import grpc
import logging
import argparse

# Set up logging
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

# Add the project root to the Python path
current_dir = os.path.dirname(os.path.abspath(__file__))
project_root = os.path.dirname(current_dir)
sys.path.insert(0, project_root)
sys.path.insert(0, os.path.join(project_root, 'proto'))

try:
    # Import the generated gRPC classes
    import proto.trading_pb2 as trading_pb2
    import proto.trading_pb2_grpc as trading_pb2_grpc
except ImportError:
    logger.error("Could not import proto files. Make sure they have been generated.")
    logger.error("Run 'python scripts/generate_proto.py' to generate them.")
    sys.exit(1)

def test_get_historical_data(stub):
    """Test the GetHistoricalData endpoint"""
    logger.info("Testing GetHistoricalData...")
    request = trading_pb2.HistoricalDataRequest(ticker="SPY", days=5)
    response = stub.GetHistoricalData(request)

    # Print the first few candles
    candle_count = len(response.candles)
    logger.info(f"Received {candle_count} candles")

    if candle_count > 0:
        logger.info("Sample data:")
        for i, candle in enumerate(response.candles[:3]):
            logger.info(f"  {candle.date}: Open={candle.open:.2f}, Close={candle.close:.2f}")
        logger.info("GetHistoricalData test successful!")
        return True
    else:
        logger.warning("Received empty response from GetHistoricalData")
        return False

def test_generate_signals(stub):
    """Test the GenerateSignals endpoint"""
    logger.info("\nTesting GenerateSignals...")
    signal_request = trading_pb2.SignalRequest(
            ticker="SPY",
            days=10,
            strategy="RedCandle"
    )
    signal_response = stub.GenerateSignals(signal_request)

    # Print signals
    signal_count = len(signal_response.signals)
    logger.info(f"Received {signal_count} signals")

    if signal_count > 0:
        logger.info("Sample signals:")
        for i, signal in enumerate(signal_response.signals[:3]):
            logger.info(f"  {signal.date}: {signal.signal_type} at ${signal.entry_price:.2f}, Stoploss=${signal.stoploss:.2f}")
        logger.info("GenerateSignals test successful!")
        return True
    else:
        logger.info("No signals generated for the specified period (this is normal)")
        return True

def test_run_backtest(stub):
    """Test the RunBacktest endpoint"""
    logger.info("\nTesting RunBacktest...")

    # Create a backtest request with various parameters
    backtest_request = trading_pb2.BacktestRequest(
            ticker="SPY",
            days=10,
            strategy="RedCandle",
            interval="15min",
            profit_targets=[5.0, 10.0, 15.0],
            risk_reward_ratios=[1.0, 2.0, 3.0],
            profit_targets_dollar=[100.0, 250.0, 500.0]
    )

    # Call the RunBacktest method
    backtest_response = stub.RunBacktest(backtest_request)

    # Check the response
    result_count = len(backtest_response.results)
    logger.info(f"Received {result_count} backtest results")

    if result_count > 0:
        logger.info("Sample backtest results:")
        count = 0
        for test_name, result in backtest_response.results.items():
            if count < 3:  # Only show first 3 results
                logger.info(f"  {test_name}:")
                logger.info(f"    Win Rate: {result.win_rate:.2f}%")
                logger.info(f"    Profit Factor: {result.profit_factor:.2f}")
                logger.info(f"    Total Trades: {result.total_trades}")
                logger.info(f"    Win/Loss: {result.winning_trades}/{result.losing_trades}")
                count += 1
        logger.info("RunBacktest test successful!")
        return True
    else:
        logger.warning("Received empty response from RunBacktest")
        return False

def test_get_recommendations(stub):
    """Test the GetOptionsRecommendations endpoint"""
    logger.info("\nTesting GetOptionsRecommendations...")
    rec_request = trading_pb2.RecommendationRequest(
            ticker="SPY",
            days=10,
            strategy="RedCandle"
    )
    rec_response = stub.GetOptionsRecommendations(rec_request)

    # Check the response
    rec_count = len(rec_response.recommendations)
    logger.info(f"Received {rec_count} recommendations")

    if rec_count > 0:
        logger.info("Sample recommendations:")
        for i, rec in enumerate(rec_response.recommendations[:3]):
            logger.info(f"  {rec.date}: {rec.option_type} @ ${rec.strike:.2f} exp {rec.expiration}")
            logger.info(f"    Signal: {rec.signal_type}, Stock: ${rec.stock_price:.2f}, Stoploss: ${rec.stoploss:.2f}")
        logger.info("GetOptionsRecommendations test successful!")
        return True
    else:
        logger.info("No recommendations generated (this may be normal)")
        return True

def run_tests(target="localhost:50052", test_type="all"):
    """Run specified tests against the gRPC server"""
    logger.info(f"Testing connection to {target}...")

    try:
        # Create a gRPC channel
        channel = grpc.insecure_channel(target)

        # Create a stub (client)
        stub = trading_pb2_grpc.TradingServiceStub(channel)

        success = True

        # Run tests based on the test_type
        if test_type in ["all", "historical"]:
            success = test_get_historical_data(stub) and success

        if test_type in ["all", "signals"]:
            success = test_generate_signals(stub) and success

        if test_type in ["all", "backtest"]:
            success = test_run_backtest(stub) and success

        if test_type in ["all", "recommendations"]:
            success = test_get_recommendations(stub) and success

        logger.info("\nAll requested tests completed!")
        return success

    except grpc.RpcError as e:
        logger.error(f"RPC error: {e.details()}")
        status_code = e.code()
        logger.error(f"Status code: {status_code.name} ({status_code.value})")
        return False
    except Exception as e:
        logger.error(f"Error: {str(e)}")
        return False

if __name__ == "__main__":
    # Parse command line arguments
    parser = argparse.ArgumentParser(description='Test client for TradingLab gRPC service')
    parser.add_argument('target', nargs='?', default='localhost:50052', help='gRPC server address (default: localhost:50052)')
    parser.add_argument('test_type', nargs='?', default='all', choices=['all', 'historical', 'signals', 'backtest', 'recommendations'],
                        help='Type of test to run (default: all)')

    args = parser.parse_args()

    # Run the tests
    success = run_tests(args.target, args.test_type)
    sys.exit(0 if success else 1)