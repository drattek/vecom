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

PUBLISH="${PSEC_PUBLISH:-}"

# Rol proxy: el SOCKS5 SOLO arranca con credenciales (PSEC_PROXY_USER/PASS). Así
# no queda un proxy abierto en la malla. Escucha autenticado en 0.0.0.0:1080
# (interno del contenedor) y se expone a la malla con un publish 1080. Sin
# credenciales no se pasan flags de proxy: el satélite salta el rol sin reiniciar.
case ",$ROLES," in
  *,proxy,*)
    if [ -n "${PSEC_PROXY_USER:-}" ] && [ -n "${PSEC_PROXY_PASS:-}" ]; then
      PROXY_LISTEN="${PSEC_PROXY_LISTEN:-0.0.0.0:1080}"
      PROXY_PORT="${PROXY_LISTEN##*:}"
      set -- "$@" -proxy-listen "$PROXY_LISTEN" \
        -proxy-user "$PSEC_PROXY_USER" -proxy-pass "$PSEC_PROXY_PASS"
      # Publica el SOCKS5 en la malla (overlayIP:PORT -> localhost:PORT).
      PUBLISH="${PUBLISH:+$PUBLISH,}${PROXY_PORT}:localhost:${PROXY_PORT}"
      echo "[psec] rol proxy: SOCKS5 autenticado en $PROXY_LISTEN, publicado en la malla puerto $PROXY_PORT"
    else
      echo "[psec] rol proxy configurado pero sin PSEC_PROXY_USER/PSEC_PROXY_PASS: el SOCKS5 NO arranca (evita un proxy abierto); la malla sigue"
    fi
    ;;
esac

[ -n "$PUBLISH" ] && set -- "$@" -publish "$PUBLISH"

echo "[psec] arrancando satélite '$NODE' (roles=$ROLES) contra $SERVER"
exec "$@"
