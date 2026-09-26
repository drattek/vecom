# Imagen del satélite PSec para Azure Container Apps (contexto: infrastructure/psec).
#
# El binario se compila aparte con deploy-satellite.sh (go build estático) y se
# deja en build/psec-satellite antes de construir esta imagen, para no arrastrar
# todo el árbol de PSec al contexto de build.
FROM alpine:3.22

# ca-certificates: TLS del gRPC/mTLS y del enroll. libcap no hace falta: netstack
# no usa capacidades de red.
RUN apk add --no-cache ca-certificates \
    && addgroup -S psec && adduser -S -G psec -u 10001 psec

WORKDIR /home/psec

# Binario estático (linux/amd64, CGO off) y entrypoint.
COPY build/psec-satellite /usr/local/bin/psec-satellite
COPY entrypoint.sh /usr/local/bin/psec-entrypoint.sh
RUN chmod +x /usr/local/bin/psec-satellite /usr/local/bin/psec-entrypoint.sh

# La identidad vive en un volumen montado aquí (ver deploy-satellite.sh).
ENV PSEC_CERTS=/var/psec/certs
RUN mkdir -p /var/psec/certs && chown -R psec:psec /var/psec

USER psec

ENTRYPOINT ["/usr/local/bin/psec-entrypoint.sh"]
