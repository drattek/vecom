#!/bin/sh
# Arranque del satélite PSec dentro de un contenedor sin privilegios (Azure
# Container Apps): usa el modo -netstack (WireGuard userspace sobre gVisor, sin
# /dev/net/tun ni CAP_NET_ADMIN).
#
# Identidad persistente: enrola SOLO si no hay client.key en el volumen montado.
# Con la carpeta de certs en un volumen (Azure Files), el nodo conserva su llave
# y el central le reasigna la MISMA IP de malla entre reinicios y redeploys.
set -eu

CERTS="${PSEC_CERTS:-/var/psec/certs}"
NODE="${PSEC_NODE_ID:-$(hostname)}"
SERVER="${PSEC_SERVER:-173.254.237.151:7791}"
ENROLL_URL="${PSEC_ENROLL_URL:-http://173.254.237.151:8098}"
SERVER_NAME="${PSEC_SERVER_NAME:-psec-central}"
# Rol explícito (scanner|proxy|wifi, coma-separado). Sin default silencioso.
ROLES="${PSEC_ROLES:?define PSEC_ROLES, p.ej. scanner}"

mkdir -p "$CERTS"

if [ ! -f "$CERTS/client.key" ]; then
  # Primer arranque (o volumen vacío): hay que enrolar con el token de flota.
  if [ -z "${PSEC_ENROLL_TOKEN:-}" ]; then
    echo "[psec] no hay identidad en $CERTS y falta PSEC_ENROLL_TOKEN: no puedo enrolar" >&2
    exit 1
  fi
  echo "[psec] enrolando nodo '$NODE' en la malla ($ENROLL_URL)..."
  psec-satellite -enroll \
    -enroll-url "$ENROLL_URL" \
    -enroll-token "$PSEC_ENROLL_TOKEN" \
    -id "$NODE" \
    -certs-dir "$CERTS"
  echo "[psec] enroll OK: identidad escrita en $CERTS (la llave privada nunca salió del nodo)"
else
  echo "[psec] identidad existente en $CERTS: reutilizando (sin re-enrolar)"
fi

# -netstack: overlay en userspace, sin TUN. Los servicios del contenedor se
# exponen a la malla con PSEC_PUBLISH (p.ej. "8080:localhost:8080"): en ACA los
# contenedores del mismo app comparten red, así que el destino es localhost.
set -- psec-satellite -netstack \
  -server "$SERVER" \
  -id "$NODE" \
  -roles "$ROLES" \
  -tls-ca "$CERTS/ca.crt" \
  -tls-cert "$CERTS/client.crt" \
  -tls-key "$CERTS/client.key" \
  -tls-server-name "$SERVER_NAME" \
  -auth "$(cat "$CERTS/auth.token")"

[ -n "${PSEC_REGION:-}" ] && set -- "$@" -region "$PSEC_REGION"
[ -n "${PSEC_PUBLISH:-}" ] && set -- "$@" -publish "$PSEC_PUBLISH"

echo "[psec] arrancando satélite '$NODE' (roles=$ROLES) contra $SERVER"
exec "$@"
