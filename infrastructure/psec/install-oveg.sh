#!/usr/bin/env bash
# Instala el satélite PSec NATIVO en la VM OVEG (Odoo de refaccionesvegusa.com),
# con P2P directo (UDP 51820 entrante abierto en su NSG) y salida a la malla del
# MySQL de vecom por su private endpoint. Al ser una VM con /dev/net/tun, el
# satélite corre en modo malla normal (no netstack) y como servicio systemd, que
# sobrevive reinicios.
#
# Lo corres TÚ con !, porque inyecta el token de flota (deploy-certs/auth.token)
# en la VM vía az run-command; el asistente tiene bloqueado materializar ese token.
#
#   ! ~/PycharmProjects/vecom/infrastructure/psec/install-oveg.sh
#
# Requisitos: az login (sesión VEGUSA con MFA), y el central con -enable-enroll.
set -euo pipefail

# ---------- Parámetros ----------
VM_RG="${VM_RG:-ODOO19-RG-VEGUSA}"
VM_NAME="${VM_NAME:-OVEG}"
NSG_RG="${NSG_RG:-$VM_RG}"
NSG_NAME="${NSG_NAME:-odoo-vegusa-nsg}"
NODE="${PSEC_NODE_ID:-sat-oveg}"

CENTRAL_BASE="${PSEC_ENROLL_URL:-http://173.254.237.151:8098}"
CENTRAL_GRPC="${PSEC_SERVER:-173.254.237.151:7791}"
CENTRAL_NAME="${PSEC_SERVER_NAME:-psec-central}"
WG_PORT="${PSEC_WG_PORT:-51820}"

# MySQL a publicar en la malla. Se usa la IP del private endpoint (no el nombre)
# para no depender de que la zona DNS privada esté enlazada a odoo-vegusa-vnet.
MYSQL_PE_IP="${MYSQL_PE_IP:-10.50.2.4}"
# Publicaciones del satélite: MySQL, más lo que añadas en PSEC_PUBLISH.
PUBLISH="3306:${MYSQL_PE_IP}:3306${PSEC_PUBLISH:+,$PSEC_PUBLISH}"
ROLES="${PSEC_ROLES:-scanner}"

TOKEN_FILE="${PSEC_TOKEN_FILE:-$HOME/PycharmProjects/PSec/psec-scan/deploy-certs/auth.token}"

die() { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }
say() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }

[[ -f "$TOKEN_FILE" ]] || die "no encuentro el token en $TOKEN_FILE (exporta PSEC_TOKEN_FILE)"
for b in az; do command -v "$b" >/dev/null || die "falta $b"; done
TOKEN="$(tr -d '\r\n' < "$TOKEN_FILE")"

# ---------------------------------------------------------------------
say "1/3 Abriendo UDP $WG_PORT entrante en el NSG $NSG_NAME (P2P directo de WireGuard)"
# WireGuard es seguro por diseño (solo responde a peers con clave válida); se
# permite desde cualquier origen porque los peers marcan desde IPs públicas variables.
az network nsg rule create -g "$NSG_RG" --nsg-name "$NSG_NAME" -n AllowPSecWG \
  --priority 150 --direction Inbound --access Allow --protocol Udp \
  --destination-port-ranges "$WG_PORT" \
  --source-address-prefixes '*' --destination-address-prefixes '*' \
  --description "PSec WireGuard (satélite OVEG)" --output none 2>/dev/null \
  || az network nsg rule update -g "$NSG_RG" --nsg-name "$NSG_NAME" -n AllowPSecWG \
       --priority 150 --access Allow --protocol Udp --destination-port-ranges "$WG_PORT" --output none
echo "     regla AllowPSecWG lista"

# ---------------------------------------------------------------------
say "2/3 Instalando el satélite en $VM_NAME (nodo $NODE) como servicio systemd"
# Script que corre como root dentro de la VM. Enrola solo si no hay identidad
# previa (sobrevive reinicios y reejecuciones): la llave privada nunca sale de OVEG.
REMOTE="$(cat <<REOF
set -e
BASE='$CENTRAL_BASE'; GRPC='$CENTRAL_GRPC'; SRV='$CENTRAL_NAME'
NODE='$NODE'; DEST=/opt/psec; PUB='$PUBLISH'; ROLES='$ROLES'
mkdir -p "\$DEST/certs"
echo "Descargando el binario del central..."
curl -fsSL "\$BASE/bin/psec-satellite-linux-amd64" -o "\$DEST/psec-satellite"
chmod +x "\$DEST/psec-satellite"
if [ ! -f "\$DEST/certs/client.key" ]; then
  echo "Enrolando \$NODE..."
  "\$DEST/psec-satellite" -enroll -enroll-url "\$BASE" -enroll-token '$TOKEN' -id "\$NODE" -certs-dir "\$DEST/certs"
else
  echo "Identidad existente: reutilizando (sin re-enrolar)"
fi
TOK="\$(tr -d '\r\n' < "\$DEST/certs/auth.token")"
"\$DEST/psec-satellite" -service uninstall 2>/dev/null || true
"\$DEST/psec-satellite" -service install -server "\$GRPC" -auth "\$TOK" -id "\$NODE" -roles "\$ROLES" \
  -tls-ca "\$DEST/certs/ca.crt" -tls-cert "\$DEST/certs/client.crt" -tls-key "\$DEST/certs/client.key" \
  -tls-server-name "\$SRV" -overlay-iface wg0 -wg-listen $WG_PORT -publish "\$PUB"
sleep 6
echo "--- estado del servicio ---"
systemctl is-active psec-satellite || true
journalctl -u psec-satellite --no-pager -n 20 2>/dev/null | sed -E 's/(token|auth)=[^ ]*/\1=***/Ig' || true
echo "--- IP de malla asignada ---"
ip -4 addr show wg0 2>/dev/null | grep -oE 'inet 10\.99\.[0-9.]+' || echo "(wg0 aún levantando; revisa el central)"
REOF
)"

az vm run-command invoke -g "$VM_RG" -n "$VM_NAME" \
  --command-id RunShellScript --scripts "$REMOTE" \
  --query "value[0].message" -o tsv 2>&1 | tail -30

# ---------------------------------------------------------------------
say "3/3 LISTO"
echo "  Satélite '$NODE' instalado en $VM_NAME (servicio systemd, arranca al boot)."
echo "  Publica en la malla: $PUBLISH"
echo "  Desde un nodo de la malla:  mysql -h <ip-malla-de-$NODE> -P 3306 -u <user> -p"
echo "  (la IP de malla de $NODE sale en la salida de arriba, o en el panel del central)"
echo
echo "  P2P directo: al tener UDP $WG_PORT entrante, los peers hacen handshake directo"
echo "  con OVEG sin pasar por el relay del hub."
