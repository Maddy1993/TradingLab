FROM python:3.10-slim

WORKDIR /app

# Install build dependencies
RUN apt-get update && apt-get install -y \
    build-essential \
    protobuf-compiler \
    python3-dev \
    && rm -rf /var/lib/apt/lists/*

# Set environment variables
ENV PYTHONDONTWRITEBYTECODE=1 \
    PYTHONUNBUFFERED=1 \
    CACHE_DIR=/app/data_cache

# Copy requirements first for caching
COPY requirements.txt .
RUN pip install --no-cache-dir -r requirements.txt

# Install NATS client for event streaming
RUN pip install nats-py==2.4.0 asyncio-nats-streaming>=0.4.0

# Copy protobuf definition files
COPY proto/ /app/proto/

# Make sure proto directory is a package
RUN touch /app/proto/__init__.py

# Generate Python gRPC code from proto files directly in the proto directory
RUN cd /app && python -m grpc_tools.protoc \
    -I./proto \
    --python_out=./proto \
    --grpc_python_out=./proto \
    ./proto/trading.proto

# Copy project code
COPY data/ /app/data/
COPY strategy/ /app/strategy/
COPY analysis/ /app/analysis/
COPY core/ /app/core/
COPY server/ /app/server/
COPY events/ /app/events/
COPY __init__.py /app/

# Create cache directory
RUN mkdir -p ${CACHE_DIR} && chmod -R 777 ${CACHE_DIR}

# Expose gRPC port
EXPOSE 50052

# Health check for the container
HEALTHCHECK --interval=30s --timeout=30s --start-period=5s --retries=3 \
  CMD python -c "import socket; s = socket.socket(socket.AF_INET, socket.SOCK_STREAM); s.connect(('localhost', 50052))" || exit 1

# Command to run the server
CMD ["python", "server/main.py"]