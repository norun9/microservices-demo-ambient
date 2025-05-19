FROM golang:1.24 AS builder

ENV GOBIN=/app/bin

RUN go install go.k6.io/xk6/cmd/xk6@v0.12.0

RUN /app/bin/xk6 build v0.45.1 \
    --with github.com/grafana/xk6-grpc@v0.1.3 \
    --output /k6

FROM alpine:latest
WORKDIR /app

COPY --from=builder /k6 /usr/bin/k6
COPY script.js /app/script.js
COPY proto /app/proto

ENTRYPOINT ["k6", "run", "/app/script.js"]