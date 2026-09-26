#!/usr/bin/env bash
# =====================================================================
# Añade el satélite PSec (modo netstack, sin privilegios) a una Container App
# como contenedor sidecar, con identidad persistente en Azure Files.
#
#   PSEC_ROLES=scanner PSEC_ENROLL_TOKEN=<token-de-flota> \
#     infrastructure/azure/deploy-satellite.sh vecom-dashboard [PSEC_PUBLISH]
#
# Qué hace:
#   1. Compila el binario del satélite desde el repo PSec (go build estático).
#   2. Construye la imagen infrastructure/psec y la sube al ACR.
#   3. Crea un share de Azure Files para los certs del nodo y lo registra en el
#      entorno (la identidad sobrevive reinicios y redeploys).
#   4. Añade el satélite como sidecar de la app destino, con el token de enrol
#      como secreto y el volumen de certs montado.
#
# Reejecutable: para "traer el satélite" a otra app, córrelo con ese nombre.
# Requisitos: az login, docker, go, y en el entorno:
#   - PSEC_ROLES         (obligatorio: scanner|proxy|wifi, coma-separado)
#   - PSEC_ENROLL_TOKEN  (obligatorio la 1ª vez que enrola un nodo; el token
#                         nunca se imprime ni se sube a git, solo va a un secreto
#                         de la Container App)
# El nodo se enrola dentro del contenedor en su primer arranque.
# =====================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PSEC_DIR="$(cd "$SCRIPT_DIR/../psec" && pwd)"

TARGET_APP="${1:?uso: deploy-satellite.sh <container-app> [PSEC_PUBLISH]}"
PUBLISH_ARG="${2:-${PSEC_PUBLISH:-}}"

# ---------- Parámetros ----------
SUBSCRIPTION_ID="${SUBSCRIPTION_ID:-d88e33f1-2295-43b7-bb43-a2e20c3e4b60}"
RESOURCE_GROUP="${RESOURCE_GROUP:-VECOM}"
LOCATION="${LOCATION:-southcentralus}"
ACR_NAME="${ACR_NAME:-acrvecom}"
ENVIRONMENT_NAME="${ENVIRONMENT_NAME:-vecom-env}"
STORAGE_ACCOUNT="${STORAGE_ACCOUNT:-vecomacafiles}"
# Repo de PSec con el satélite (fuera de este repo).
PSEC_SRC="${PSEC_SRC:-$HOME/PycharmProjects/PSec/psec-scan}"
# Identidad del nodo y del central.
NODE_ID="${PSEC_NODE_ID:-$TARGET_APP}"
PSEC_SERVER="${PSEC_SERVER:-173.254.237.151:7791}"
PSEC_ENROLL_URL="${PSEC_ENROLL_URL:-http://173.254.237.151:8098}"
PSEC_SERVER_NAME="${PSEC_SERVER_NAME:-psec-central}"

ACR_SERVER="${ACR_NAME}.azurecr.io"
IMAGE="${ACR_SERVER}/psec/satellite:latest"
SHARE="psec-certs-${TARGET_APP}"
SECRET_NAME="psec-enroll-token"
IDENTITY_NAME="${IDENTITY_NAME:-vecom-apps}"

die() { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }
say() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }

: "${PSEC_ROLES:?define PSEC_ROLES, p.ej. PSEC_ROLES=scanner}"
[[ -d "$PSEC_SRC" ]] || die "no encuentro el repo PSec en $PSEC_SRC (exporta PSEC_SRC)"
for bin in az docker go jq python3; do command -v "$bin" >/dev/null || die "falta $bin"; done
az containerapp show -n "$TARGET_APP" -g "$RESOURCE_GROUP" >/dev/null 2>&1 \
  || die "la Container App '$TARGET_APP' no existe en $RESOURCE_GROUP"

az account set --subscription "$SUBSCRIPTION_ID"

# ---------------------------------------------------------------------
say "1/5 Compilando el satélite (linux/amd64 estático) desde $PSEC_SRC"
( cd "$PSEC_SRC" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w" -o "$PSEC_DIR/build/psec-satellite" ./cmd/psec-satellite )
echo "     binario: $(du -h "$PSEC_DIR/build/psec-satellite" | cut -f1)"

# ---------------------------------------------------------------------
say "2/5 Imagen $IMAGE -> ACR"
az acr login --name "$ACR_NAME"
docker buildx build --platform linux/amd64 -t "$IMAGE" "$PSEC_DIR" --push

# ---------------------------------------------------------------------
say "3/5 Azure Files para la identidad del nodo: $SHARE"
STORAGE_KEY="$(az storage account keys list --account-name "$STORAGE_ACCOUNT" \
  --resource-group "$RESOURCE_GROUP" --query '[0].value' -o tsv)"
az storage share-rm create --storage-account "$STORAGE_ACCOUNT" --resource-group "$RESOURCE_GROUP" \
  --name "$SHARE" --quota 1 --output none 2>/dev/null || true
az containerapp env storage set --name "$ENVIRONMENT_NAME" --resource-group "$RESOURCE_GROUP" \
  --storage-name "$SHARE" --azure-file-account-name "$STORAGE_ACCOUNT" \
  --azure-file-account-key "$STORAGE_KEY" --azure-file-share-name "$SHARE" \
  --access-mode ReadWrite --output none

# ---------------------------------------------------------------------
say "4/5 Token de enrol como secreto de $TARGET_APP"
if [[ -n "${PSEC_ENROLL_TOKEN:-}" ]]; then
  az containerapp secret set -n "$TARGET_APP" -g "$RESOURCE_GROUP" \
    --secrets "${SECRET_NAME}=${PSEC_ENROLL_TOKEN}" --output none
  echo "     secreto $SECRET_NAME actualizado"
else
  # Si ya hay identidad en el share, no hace falta token; si no, fallará el enroll.
  az containerapp secret show -n "$TARGET_APP" -g "$RESOURCE_GROUP" --secret-name "$SECRET_NAME" &>/dev/null \
    || echo "     AVISO: sin PSEC_ENROLL_TOKEN y sin secreto previo; el nodo no podrá enrolar si el share está vacío"
fi

# ---------------------------------------------------------------------
say "5/5 Añadiendo el sidecar 'psec-satellite' a $TARGET_APP"
az containerapp show -n "$TARGET_APP" -g "$RESOURCE_GROUP" -o json > "$SCRIPT_DIR/.satellite-show.json"
IMAGE="$IMAGE" SHARE="$SHARE" NODE_ID="$NODE_ID" TARGET_APP="$TARGET_APP" \
PSEC_ROLES="$PSEC_ROLES" PSEC_SERVER="$PSEC_SERVER" PSEC_ENROLL_URL="$PSEC_ENROLL_URL" \
PSEC_SERVER_NAME="$PSEC_SERVER_NAME" PUBLISH_ARG="$PUBLISH_ARG" SECRET_NAME="$SECRET_NAME" \
python3 - "$SCRIPT_DIR/.satellite-show.json" "$SCRIPT_DIR/.satellite-patch.json" <<'PY'
import json, os, sys
app = json.load(open(sys.argv[1]))
tpl = app["properties"]["template"]
env = os.environ

# Volumen de certs (Azure Files ya registrado en el entorno con name==share).
tpl.setdefault("volumes", [])
tpl["volumes"] = [v for v in (tpl["volumes"] or []) if v.get("name") != "psec-certs"]
tpl["volumes"].append({"name": "psec-certs", "storageType": "AzureFile", "storageName": env["SHARE"]})

envvars = [
    {"name": "PSEC_NODE_ID", "value": env["NODE_ID"]},
    {"name": "PSEC_ROLES", "value": env["PSEC_ROLES"]},
    {"name": "PSEC_SERVER", "value": env["PSEC_SERVER"]},
    {"name": "PSEC_ENROLL_URL", "value": env["PSEC_ENROLL_URL"]},
    {"name": "PSEC_SERVER_NAME", "value": env["PSEC_SERVER_NAME"]},
    {"name": "PSEC_ENROLL_TOKEN", "secretRef": env["SECRET_NAME"]},
]
if env.get("PUBLISH_ARG"):
    envvars.append({"name": "PSEC_PUBLISH", "value": env["PUBLISH_ARG"]})

sidecar = {
    "name": "psec-satellite",
    "image": env["IMAGE"],
    "resources": {"cpu": 0.25, "memory": "0.5Gi"},
    "env": envvars,
    "volumeMounts": [{"volumeName": "psec-certs", "mountPath": "/var/psec/certs"}],
}
tpl["containers"] = [c for c in tpl["containers"] if c.get("name") != "psec-satellite"] + [sidecar]
json.dump({"properties": {"template": tpl}}, open(sys.argv[2], "w"), indent=2)
PY

az containerapp update -n "$TARGET_APP" -g "$RESOURCE_GROUP" \
  --yaml "$SCRIPT_DIR/.satellite-patch.json" --output none
rm -f "$SCRIPT_DIR/.satellite-show.json" "$SCRIPT_DIR/.satellite-patch.json"

say "LISTO"
echo "  Satélite '$NODE_ID' añadido a $TARGET_APP (roles=$PSEC_ROLES)."
echo "  Identidad persistente en el share $SHARE (sobrevive reinicios y redeploys)."
[[ -n "$PUBLISH_ARG" ]] && echo "  Publicando en la malla: $PUBLISH_ARG"
echo
echo "  Ver que se une a la malla:"
echo "    az containerapp logs show -n $TARGET_APP -g $RESOURCE_GROUP --container psec-satellite --follow"
echo
echo "  EGRESS que debe permitir la red (saliente):"
echo "    TCP ${PSEC_SERVER} (gRPC/mTLS) · UDP 51820 (WireGuard) · ${PSEC_ENROLL_URL} (solo 1er enroll)"
