FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o recommendationservice .

# Use a minimal alpine image for the final stage
FROM alpine:latest

RUN apk --no-cache add ca-certificates

WORKDIR /root/

# Download and install grpc_health_probe for readiness/liveness probes
ARG GRPC_HEALTH_PROBE_VERSION=v0.4.11
RUN wget -qO /bin/grpc_health_probe \
    https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 \
 && chmod +x /bin/grpc_health_probe

# Copy the binary from builder stage
COPY --from=builder /app/recommendationservice .

# Expose the port
EXPOSE 8080

# Run the binary
CMD ["./recommendationservice"] 