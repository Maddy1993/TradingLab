FROM python:3.10-slim

WORKDIR /app

# Install dependencies
COPY gateway/requirements.txt ./
RUN pip install --no-cache-dir -r requirements.txt

# Copy proto files and generate gRPC code
COPY proto/ /app/proto/
RUN pip install grpcio-tools && \
    python -m grpc_tools.protoc \
    -I/app/proto \
    --python_out=/app/proto \
    --grpc_python_out=/app/proto \
    /app/proto/trading.proto && \
    touch /app/proto/__init__.py

# Copy API gateway code
COPY gateway/app.py /app/

# Expose the API Gateway port
EXPOSE 5000

# Command to run the app
CMD ["python", "app.py"]