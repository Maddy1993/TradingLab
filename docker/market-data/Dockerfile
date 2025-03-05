FROM golang:1.24-alpine AS builder

WORKDIR /app

# Install git and build dependencies
RUN apk add --no-cache git gcc musl-dev

# Copy Go module files
COPY go.mod go.sum* ./
RUN go mod download

# Copy source code
COPY cmd/ cmd/
COPY pkg/ pkg/
COPY proto/ proto/

# Build executable
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/market-data-service ./cmd/market-data-service

# Final stage
FROM alpine:3.18

WORKDIR /app

# Copy binary from builder
COPY --from=builder /app/bin/market-data-service .

# Set executable as entrypoint
ENTRYPOINT ["/app/market-data-service"]