#!/usr/bin/env python3
import os
import sys
import logging
from dotenv import load_dotenv

# Load environment variables from .env file if it exists
load_dotenv()

# Add project root to Python path
project_root = os.path.abspath(os.path.join(os.path.dirname(__file__), '..'))
sys.path.insert(0, project_root)

# This is where the generated proto files will be
proto_path = os.path.join(project_root, 'proto')
if os.path.exists(proto_path):
    sys.path.insert(0, proto_path)

# Import server module
from server.grpc_server import serve

if __name__ == "__main__":
    # Set up logging
    logging.basicConfig(
            level=logging.INFO,
            format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
    )
    logger = logging.getLogger(__name__)

    # Log environment information
    logger.info(f"Starting TradingLab gRPC server")
    logger.info(f"Python version: {sys.version}")
    logger.info(f"ALPHA_VANTAGE_API_KEY set: {'Yes' if os.getenv('ALPHA_VANTAGE_API_KEY') else 'No'}")
    logger.info(f"CACHE_DIR: {os.getenv('CACHE_DIR', '/app/data_cache')}")
    logger.info(f"GRPC_PORT: {os.getenv('GRPC_PORT', '50052')}")

    # Start the gRPC server
    serve()