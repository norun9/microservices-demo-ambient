### Builder stage
FROM golang:1.24-alpine AS builder

WORKDIR /app

# Copy go mod files
COPY go.mod go.sum ./
RUN go mod download

# Copy source code
COPY . .

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o paymentservice .


### Final stage
FROM alpine:latest

# Install CA certs and wget for downloading grpc_health_probe
RUN apk --no-cache add ca-certificates wget

WORKDIR /root/

# Download and install grpc_health_probe for readiness/liveness probes
ARG GRPC_HEALTH_PROBE_VERSION=v0.4.11
RUN wget -qO /bin/grpc_health_probe \
    https://github.com/grpc-ecosystem/grpc-health-probe/releases/download/${GRPC_HEALTH_PROBE_VERSION}/grpc_health_probe-linux-amd64 \
 && chmod +x /bin/grpc_health_probe

# Copy the application binary from the builder
COPY --from=builder /app/paymentservice .

# Expose the application port (adjust if needed)
EXPOSE 8080

# Run the service
CMD ["./paymentservice"]
