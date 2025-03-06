#!/usr/bin/env python3
import os
import sys
import logging
import asyncio
from concurrent import futures
import grpc
from dotenv import load_dotenv

# Load environment variables
load_dotenv()

# Add project root to Python path
project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
sys.path.insert(0, project_root)

# Add proto directory to path
proto_path = os.path.join(project_root, 'proto')
if os.path.exists(proto_path):
    sys.path.insert(0, proto_path)

# Import strategy modules
from server.trading_service import TradingServiceServicer

# Import grpc modules
from proto import trading_pb2
from proto import trading_pb2_grpc

def serve():
    """Start the gRPC server."""
    # Get server port from environment or use default
    port = os.getenv('GRPC_PORT', '50052')
    server = grpc.server(futures.ThreadPoolExecutor(max_workers=10))

    # Add the servicer to the server
    servicer = TradingServiceServicer()
    trading_pb2_grpc.add_TradingServiceServicer_to_server(
            servicer, server
    )

    # Enable reflection for easier debugging and testing
    try:
        from grpc_reflection.v1alpha import reflection
        service_names = (
            trading_pb2.DESCRIPTOR.services_by_name['TradingService'].full_name,
            reflection.SERVICE_NAME,
        )
        reflection.enable_server_reflection(service_names, server)
    except ImportError:
        logging.warning("gRPC reflection not available")

    # Start listening
    server.add_insecure_port(f'[::]:{port}')
    server.start()

    logging.info(f"gRPC server running on port {port}")

    # Start the asyncio event loop for event client
    loop = asyncio.get_event_loop()
    try:
        loop.run_until_complete(servicer.init_event_client())
        # Keep the loop running for event handling
        loop.run_forever()
    except KeyboardInterrupt:
        pass
    finally:
        loop.close()
        server.stop(0)

if __name__ == '__main__':
    # Set up logging
    logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )

    # Log environment information
    logging.info(f"Starting TradingLab Strategy Service")
    logging.info(f"Python version: {sys.version}")
    logging.info(f"NATS_URL: {os.getenv('NATS_URL', 'nats://nats:4222')}")
    logging.info(f"GRPC_PORT: {os.getenv('GRPC_PORT', '50052')}")

    # Start the server
    serve()