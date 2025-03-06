FROM golang:1.20-alpine AS builder

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
COPY ui/build ui/build

# Build executable
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/bin/api-gateway ./cmd/api-gateway

# Final stage
FROM alpine:3.18

WORKDIR /app

# Copy binary and UI files from builder
COPY --from=builder /app/bin/api-gateway .
COPY --from=builder /app/ui/build ./ui/build

# Expose port
EXPOSE 5000

# Command to run the server
CMD ["/app/api-gateway"]