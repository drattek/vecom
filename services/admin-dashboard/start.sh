#!/bin/sh
# nginx + cloudflared en el mismo contenedor, sin recursos extra de un sidecar.
#
# - nginx arranca con el entrypoint oficial (procesa /etc/nginx/templates).
# - Si TUNNEL_TOKEN está definido (Azure), arranca el túnel de Cloudflare cuyo
#   ingress (gestionado en Cloudflare) apunta a http://localhost:80.
# - Si cualquiera de los dos procesos muere, el contenedor termina para que la
#   plataforma lo reinicie, en lugar de quedar a medias.
set -eu

/docker-entrypoint.sh nginx -g 'daemon off;' &
NGINX_PID=$!

TUNNEL_PID=""
if [ -n "${TUNNEL_TOKEN:-}" ]; then
  # El token se lee de la variable de entorno TUNNEL_TOKEN (no va en la línea de comandos).
  cloudflared tunnel --no-autoupdate run &
  TUNNEL_PID=$!
fi

shutdown() {
  kill -TERM "$NGINX_PID" ${TUNNEL_PID:+"$TUNNEL_PID"} 2>/dev/null || true
  wait
  exit 0
}
trap shutdown TERM INT

while kill -0 "$NGINX_PID" 2>/dev/null; do
  if [ -n "$TUNNEL_PID" ] && ! kill -0 "$TUNNEL_PID" 2>/dev/null; then
    echo "cloudflared terminó; saliendo para que el contenedor se reinicie" >&2
    kill -TERM "$NGINX_PID" 2>/dev/null || true
    wait "$NGINX_PID" 2>/dev/null || true
    exit 1
  fi
  sleep 5
done

echo "nginx terminó; saliendo" >&2
exit 1
