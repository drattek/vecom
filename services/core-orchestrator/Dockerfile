# =========================
# Build Stage
# =========================
FROM golang:1.26-alpine AS builder

WORKDIR /app

COPY services/core-orchestrator/go.mod .
COPY services/core-orchestrator/go.sum .

RUN go mod download

COPY services/core-orchestrator/ .

RUN CGO_ENABLED=0 GOOS=linux go build \
    -o orchestrator \
    ./cmd

# =========================
# Runtime Stage
# =========================
FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && addgroup -S app && adduser -S -G app app

WORKDIR /app

COPY --from=builder /app/orchestrator .

# Las imágenes de MercadoLibre se descargan en ./downloads (MERCADOLIBRE_IMAGE_DOWNLOAD_DIR);
# el usuario no-root necesita poder escribir ahí.
RUN mkdir -p /app/downloads && chown -R app:app /app

USER app

# El keystore ya no va dentro de la imagen (*.p12 está en .gitignore y el build
# de CI no lo tiene). En local, docker-compose lo monta como volumen y define
# SERVER_TLS_KEYSTORE_PATH; en Azure Container Apps el TLS lo termina el ingress,
# la app corre en HTTP con SERVER_PORT=8080 y sin keystore.
EXPOSE 443 8080

CMD ["./orchestrator"]
