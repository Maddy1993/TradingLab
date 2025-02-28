#!/usr/bin/env python3
"""
Simple test client for the TradingLab gRPC service.
This script can be used to verify that the server is functioning correctly.

Usage:
  python test_client.py [host:port]

Example:
  python test_client.py localhost:50052
  python test_client.py tradinglab-service:50052
"""
import sys
import os
import grpc
import logging

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
    import trading_pb2
    import trading_pb2_grpc
except ImportError:
    logger.error("Could not import proto files. Make sure they have been generated.")
    logger.error("Run 'python scripts/generate_proto.py' to generate them.")
    sys.exit(1)

def run_test(target="localhost:50052"):
    """Run a simple test against the gRPC server"""
    logger.info(f"Testing connection to {target}...")

    try:
        # Create a gRPC channel
        channel = grpc.insecure_channel(target)

        # Create a stub (client)
        stub = trading_pb2_grpc.TradingServiceStub(channel)

        # Test GetHistoricalData
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
        else:
            logger.warning("Received empty response from GetHistoricalData")

        # Test GenerateSignals
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
        else:
            logger.info("No signals generated for the specified period (this is normal)")

        logger.info("\nAll tests completed successfully!")
        return True

    except grpc.RpcError as e:
        logger.error(f"RPC error: {e.details()}")
        status_code = e.code()
        logger.error(f"Status code: {status_code.name} ({status_code.value})")
        return False
    except Exception as e:
        logger.error(f"Error: {str(e)}")
        return False

if __name__ == "__main__":
    # Get target from command line args or use default
    target = sys.argv[1] if len(sys.argv) > 1 else "localhost:50052"
    success = run_test(target)
    sys.exit(0 if success else 1)