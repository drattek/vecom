# =========================
# Build Stage
# =========================
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY services/core-orchestrator/go.mod .
COPY services/core-orchestrator/go.sum .

RUN go mod download

COPY services/core-orchestrator/ .
COPY keystore.p12 /app/keystore.p12

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o orchestrator \
    ./cmd

# =========================
# Runtime Stage
# =========================
FROM alpine:3.22

RUN apk add --no-cache ca-certificates

WORKDIR /app

COPY --from=builder /app/orchestrator .
COPY --from=builder /app/keystore.p12 ./keystore.p12

EXPOSE 443

CMD ["./orchestrator"]