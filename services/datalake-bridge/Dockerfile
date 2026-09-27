# =========================
# Build Stage
# =========================
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY go.mod .
COPY go.sum .

RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o datalake-bridge \
    ./cmd

# =========================
# Runtime Stage
# =========================
FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=builder /app/datalake-bridge .

USER app

# Solo expone la sonda de salud (liveness). No recibe tráfico entrante.
EXPOSE 8080

CMD ["./datalake-bridge"]
