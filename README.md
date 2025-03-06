# TradingLab gRPC Service

This project exposes the Python-based trading system as a gRPC service that can be deployed to a Kubernetes cluster.

## Overview

TradingLab gRPC service provides the following capabilities over gRPC:

- Fetching historical price data for stocks
- Generating trading signals using built-in strategies
- Running backtests for trading strategies
- Getting options recommendations based on trading signals

## Development Setup

### Prerequisites

- Python 3.8+
- gRPC tools
- Docker
- Kubernetes CLI (kubectl)

### Installing Dependencies

```bash
pip install -r requirements.txt
```

### Generating gRPC Code

To generate the Python gRPC code from the protobuf definition:

```bash
# Using the provided script
python scripts/generate_proto.py

# Or manually
python -m grpc_tools.protoc \
    -I./proto \
    --python_out=./proto \
    --grpc_python_out=./proto \
    ./proto/trading.proto
```

This will generate `trading_pb2.py` and `trading_pb2_grpc.py` in the `proto` directory.

### Troubleshooting Import Issues

If you encounter import errors like `No module named 'trading_pb2'`:

1. Make sure you've generated the protobuf files using the script above
2. Check that your Python path includes the project root and proto directory
3. Verify that `proto/__init__.py` exists to make the directory a package

You can check your Python path by running:

```python
import sys
print(sys.path)
```

### Environment Variables

Create a `.env` file with:

```
ALPHA_VANTAGE_API_KEY=your_api_key
CACHE_DIR=./data_cache
GRPC_PORT=50052
TIMEZONE=America/Los_Angeles  # Configure for PST/PDT timezone
```

Available timezones follow the [IANA timezone database](https://www.iana.org/time-zones) format (e.g., 'America/New_York', 'Europe/London', 'Asia/Tokyo').

### Running the Server Locally

```bash
python server/main.py
```

## Docker Build

Build the Docker image:

```bash
docker build -t tradinglab:latest -f docker/Dockerfile .
```

Run the container:

```bash
docker run -p 50052:50052 \
  -e ALPHA_VANTAGE_API_KEY=your_api_key \
  -e TIMEZONE=America/Los_Angeles \
  tradinglab:latest
```

## Kubernetes Deployment

### Prerequisites

- GKE Cluster (same as FinanceTracker)
- Artifact Registry Repository
- Kubernetes Secret with API key

### Setup

1. Create the Kubernetes secret for API keys:

```bash
kubectl apply -f kube/tradinglab-secret.yaml
```

2. Apply the timezone configuration:

```bash
kubectl apply -f kube/config/timezone-config.yaml
```

3. Deploy the service:

```bash
kubectl apply -f kube/tradinglab-server.yaml
kubectl apply -f kube/tradinglab-service.yaml
```

You can change the timezone by editing the `kube/config/timezone-config.yaml` file and reapplying it.

4. Verify deployment:

```bash
kubectl get pods -l app=tradinglab-service
kubectl get svc tradinglab-service
```

## CI/CD with Cloud Build

1. Create a Cloud Build trigger pointing to your repository
2. Configure the trigger to use `cloudbuild-tradinglab.yaml`
3. Push changes to trigger the build and deployment

## Service Integration

The TradingLab gRPC service can be accessed from other services in the cluster at:

```
tradinglab-service:50052
```

## Client Example

Here's a sample Python client:

```python
import grpc
from tradinglab.proto import trading_pb2, trading_pb2_grpc

def run():
    with grpc.insecure_channel('localhost:50052') as channel:
        stub = trading_pb2_grpc.TradingServiceStub(channel)
        
        # Example: Get historical data
        response = stub.GetHistoricalData(
            trading_pb2.HistoricalDataRequest(ticker='AAPL', days=30)
        )
        
        for candle in response.candles[:5]:
            print(f"{candle.date}: Open={candle.open}, Close={candle.close}")

if __name__ == '__main__':