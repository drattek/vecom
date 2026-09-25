#!/usr/bin/env bash
# =====================================================================
# Bootstrap de Azure Container Apps para el middleware vecom.
#
# Crea (o actualiza) en el resource group VECOM:
#   - Entorno de Container Apps INTERNO (sin IP pública) sobre vecom-vnet/aca-subnet
#     (10.50.0.0/23), con peering a odoo-vegusa-vnet para que OVEG (VM de Odoo,
#     10.0.1.4) vea el middleware, y zona DNS privada del entorno enlazada a ambas redes
#   - Identidad administrada para hacer pull del ACR sin contraseñas
#   - Azure Files (cuenta vecomacafiles) para que Redis y RabbitMQ no pierdan
#     sus datos al reiniciarse
#   - 5 Container Apps. En un entorno interno, ingress "external" = visible desde la
#     VNet y sus peerings (OVEG), nunca desde internet; "internal" = solo entorno:
#       vecom-dashboard     admin-dashboard (nginx + cloudflared), visible en la VNet
#       vecom-orchestrator  core-orchestrator (HTTP 8080), visible en la VNet
#       vecom-synapse       synapse-bridge    (HTTP 8080)
#       vecom-redis         redis             (TCP 6379, datos en Azure Files)
#       vecom-rabbitmq      rabbitmq          (TCP 5672, datos en Azure Files)
#   - Cloudflare Tunnel "vecom-middleware" gestionado desde Cloudflare:
#       https://vecom-api.odo.mx -> cloudflared -> nginx del dashboard (localhost:80)
#     No toca el túnel de Odoo (odoo19-tunnel-vegusa) ni el DNS de vecom.odo.mx.
#   - Private endpoint de MySQL (vecomdb) en la VNet, sin reglas de firewall por IP
#   - Credencial federada OIDC para que GitHub Actions despliegue desde 'vecom'
#
# Secretos: se leen de vecom.env (raíz del repo, en .gitignore). Toda clave que
# contenga PASSWORD, SECRET, TOKEN o termine en _KEY se crea como secreto de la
# Container App y la variable la referencia con secretref:. El token del túnel
# se obtiene de la API de Cloudflare y va directo a un secreto. Los valores NUNCA
# se imprimen, no van a GitHub y no quedan en el repo.
#
# Después de esto, cada push a 'vecom' despliega solo con
# .github/workflows/deploy-containerapps.yml (que solo cambia la imagen).
#
# Requisitos: az login, gh auth login, jq, python3, vecom.env relleno y
# CLOUDFLARE_API_TOKEN (en vecom.env o en el entorno) con permisos
# Account > Cloudflare Tunnel:Edit y Zone > DNS:Edit sobre odo.mx.
# Es idempotente: reejecutarlo actualiza secretos, variables y volúmenes.
# =====================================================================
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# ---------- Parámetros ----------
SUBSCRIPTION_ID="${SUBSCRIPTION_ID:-d88e33f1-2295-43b7-bb43-a2e20c3e4b60}"
TENANT_ID="${TENANT_ID:-e60ad600-4568-4f55-bdad-195ca7bc2861}"
RESOURCE_GROUP="${RESOURCE_GROUP:-VECOM}"
# southcentralus = misma región que MySQL, ACR y la VNet
LOCATION="${LOCATION:-southcentralus}"
ACR_NAME="${ACR_NAME:-acrvecom}"
ENVIRONMENT_NAME="${ENVIRONMENT_NAME:-vecom-env}"
VNET_NAME="${VNET_NAME:-vecom-vnet}"
# containerapps-subnet (172.30.0.0/23) no sirve: Container Apps reserva 172.30.0.0/16
# y 172.31.0.0/16. Se añade un rango 10.50.0.0/16 a la VNet con su propia subnet.
SUBNET_NAME="${SUBNET_NAME:-aca-subnet}"
VNET_EXTRA_PREFIX="${VNET_EXTRA_PREFIX:-10.50.0.0/16}"
SUBNET_PREFIX="${SUBNET_PREFIX:-10.50.0.0/23}"
PE_SUBNET_PREFIX="${PE_SUBNET_PREFIX:-10.50.2.0/27}"
LOG_WORKSPACE="${LOG_WORKSPACE:-workspace-hNab}"
IDENTITY_NAME="${IDENTITY_NAME:-vecom-apps}"
STORAGE_ACCOUNT="${STORAGE_ACCOUNT:-vecomacafiles}"
MYSQL_SERVER="${MYSQL_SERVER:-vecomdb}"
GITHUB_REPO="${GITHUB_REPO:-drattek/vecom}"
APP_REGISTRATION="${APP_REGISTRATION:-gh-actions-vecom}"
DEPLOY_BRANCH="${DEPLOY_BRANCH:-vecom}"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/vecom.env}"

# Red de OVEG (VM de Odoo de refaccionesvegusa.com): debe ver el middleware
ODOO_VNET_RG="${ODOO_VNET_RG:-odoo19-rg-vegusa}"
ODOO_VNET="${ODOO_VNET:-odoo-vegusa-vnet}"

# Cloudflare
CF_ZONE="${CF_ZONE:-odo.mx}"
CF_HOSTNAME="${CF_HOSTNAME:-vecom-api.odo.mx}"
CF_TUNNEL_NAME="${CF_TUNNEL_NAME:-vecom-middleware}"

# Deben coincidir con .github/workflows/deploy-containerapps.yml
APP_DASHBOARD=vecom-dashboard
APP_ORCH=vecom-orchestrator
APP_SYNAPSE=vecom-synapse
APP_REDIS=vecom-redis
APP_RABBIT=vecom-rabbitmq

ACR_SERVER="${ACR_NAME}.azurecr.io"
# Imagen temporal para crear las apps antes del primer despliegue del workflow
PLACEHOLDER_IMAGE="mcr.microsoft.com/k8se/quickstart:latest"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

say() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
die() { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }

[[ -f "$ENV_FILE" ]] || die "falta $ENV_FILE (cópialo de vecom.env.example)"
for bin in az gh jq python3 curl; do
  command -v "$bin" >/dev/null || die "falta el comando $bin"
done

# ---------- Lectura de vecom.env (sin imprimir valores) ----------
# Misma semántica que docker-compose/dotenv: admite finales CRLF, comillas
# simples (literal) y dobles (donde \" y \\ son escapes).
env_get() {
  local line value
  line="$(tr -d '\r' < "$ENV_FILE" | grep -E "^${1}=" | tail -1 || true)"
  [[ -z "$line" ]] && return 0
  value="${line#*=}"
  if [[ ${#value} -ge 2 ]]; then
    local first="${value:0:1}" last="${value: -1}"
    if [[ "$first" == "'" && "$last" == "'" ]]; then
      value="${value:1:${#value}-2}"
    elif [[ "$first" == '"' && "$last" == '"' ]]; then
      value="${value:1:${#value}-2}"
      value="${value//\\\\/$'\x01'}"   # \\ -> marcador temporal
      value="${value//\\\"/\"}"         # \" -> "
      value="${value//$'\x01'/\\}"       # marcador -> \
    fi
  fi
  printf '%s' "$value"
}

is_secret() { [[ "$1" =~ (PASSWORD|SECRET|TOKEN|_KEY$) ]]; }

secret_name() { printf '%s' "$1" | tr '[:upper:]_' '[:lower:]-'; }

# Rellena SECRETS y ENVVARS con las claves indicadas que tengan valor.
SECRETS=(); ENVVARS=()
build_config() {
  SECRETS=(); ENVVARS=()
  local key value name
  for key in "$@"; do
    value="$(env_get "$key")"
    [[ -z "$value" ]] && continue
    if is_secret "$key"; then
      name="$(secret_name "$key")"
      SECRETS+=("${name}=${value}")
      ENVVARS+=("${key}=secretref:${name}")
    else
      ENVVARS+=("${key}=${value}")
    fi
  done
}

# ---------- Validación ----------
REQUIRED=(
  JWT_SECRET CONNECTION_CREDENTIALS_ENCRYPTION_KEY
  MYSQL_HOST MYSQL_USER MYSQL_PASSWORD
  RABBITMQ_USER RABBITMQ_PASSWORD
  FABRIC_JDBC_URL FABRIC_USERNAME FABRIC_PASSWORD
  SYNC_STOCK_CRON SYNC_ITEMS_CRON SYNC_EXISTENCIAS_CRON
)
missing=()
for key in "${REQUIRED[@]}"; do
  [[ -n "$(env_get "$key")" ]] || missing+=("$key")
done
if [[ ${#missing[@]} -gt 0 ]]; then
  die "faltan valores en $(basename "$ENV_FILE"): ${missing[*]}"
fi
# RabbitMQ solo permite el usuario guest desde localhost; entre contenedores falla.
if [[ "$(env_get RABBITMQ_USER)" == "guest" ]]; then
  die "RABBITMQ_USER=guest no funciona fuera de localhost; define otro usuario en $(basename "$ENV_FILE")"
fi
# El ERP se lee del lakehouse del Link to Fabric, no de Synapse ni de dyn365_lakehouse.
if ! grep -q 'dataverse_vegusa_cds2_workspace' <<<"$(env_get FABRIC_JDBC_URL)"; then
  die "FABRIC_JDBC_URL no apunta al lakehouse del Link to Fabric (ver vecom.env.example)"
fi
CLOUDFLARE_API_TOKEN="${CLOUDFLARE_API_TOKEN:-$(env_get CLOUDFLARE_API_TOKEN)}"
[[ -n "$CLOUDFLARE_API_TOKEN" ]] || die "falta CLOUDFLARE_API_TOKEN (en vecom.env o exportado)"

az account set --subscription "$SUBSCRIPTION_ID"

# ---------------------------------------------------------------------
say "1/10 Proveedores de recursos"
az provider register --namespace Microsoft.App --wait
az provider register --namespace Microsoft.OperationalInsights --wait
az provider register --namespace Microsoft.Storage --wait

# ---------------------------------------------------------------------
say "2/10 Identidad administrada $IDENTITY_NAME (pull del ACR)"
ACR_ID="$(az acr show --name "$ACR_NAME" --resource-group "$RESOURCE_GROUP" --query id -o tsv)"
if ! az identity show --name "$IDENTITY_NAME" --resource-group "$RESOURCE_GROUP" &>/dev/null; then
  az identity create --name "$IDENTITY_NAME" --resource-group "$RESOURCE_GROUP" \
    --location "$LOCATION" --output none
  NEW_IDENTITY=1
fi
IDENTITY_ID="$(az identity show --name "$IDENTITY_NAME" --resource-group "$RESOURCE_GROUP" --query id -o tsv)"
IDENTITY_PRINCIPAL="$(az identity show --name "$IDENTITY_NAME" --resource-group "$RESOURCE_GROUP" --query principalId -o tsv)"
az role assignment create --assignee-object-id "$IDENTITY_PRINCIPAL" --assignee-principal-type ServicePrincipal \
  --role AcrPull --scope "$ACR_ID" --output none 2>/dev/null || true
if [[ -n "${NEW_IDENTITY:-}" ]]; then
  echo "     esperando la propagación del rol AcrPull..."
  sleep 60
fi

# ---------------------------------------------------------------------
say "3/10 Imágenes de terceros en el ACR (evita el rate limit de Docker Hub)"
for img in redis:7-alpine rabbitmq:4-management; do
  if ! az acr repository show --name "$ACR_NAME" --image "$img" &>/dev/null; then
    az acr import --name "$ACR_NAME" --source "docker.io/library/${img}" --image "$img" --output none
    echo "     + $img"
  fi
done

# ---------------------------------------------------------------------
say "4/10 Entorno de Container Apps: $ENVIRONMENT_NAME"
if ! az containerapp env show --name "$ENVIRONMENT_NAME" --resource-group "$RESOURCE_GROUP" &>/dev/null; then
  if ! az network vnet subnet show --resource-group "$RESOURCE_GROUP" --vnet-name "$VNET_NAME" \
      --name "$SUBNET_NAME" &>/dev/null; then
    if ! az network vnet show --resource-group "$RESOURCE_GROUP" --name "$VNET_NAME" \
        --query addressSpace.addressPrefixes -o tsv | grep -qx "$VNET_EXTRA_PREFIX"; then
      az network vnet update --resource-group "$RESOURCE_GROUP" --name "$VNET_NAME" \
        --add addressSpace.addressPrefixes "$VNET_EXTRA_PREFIX" --output none
      echo "     + rango $VNET_EXTRA_PREFIX en $VNET_NAME"
    fi
    az network vnet subnet create --resource-group "$RESOURCE_GROUP" --vnet-name "$VNET_NAME" \
      --name "$SUBNET_NAME" --address-prefixes "$SUBNET_PREFIX" \
      --delegations Microsoft.App/environments --output none
    echo "     + subnet $SUBNET_NAME ($SUBNET_PREFIX)"
  fi
  SUBNET_ID="$(az network vnet subnet show --resource-group "$RESOURCE_GROUP" \
    --vnet-name "$VNET_NAME" --name "$SUBNET_NAME" --query id -o tsv)"
  LOG_ID="$(az monitor log-analytics workspace show --resource-group "$RESOURCE_GROUP" \
    --workspace-name "$LOG_WORKSPACE" --query customerId -o tsv)"
  LOG_KEY="$(az monitor log-analytics workspace get-shared-keys --resource-group "$RESOURCE_GROUP" \
    --workspace-name "$LOG_WORKSPACE" --query primarySharedKey -o tsv)"
  # La VNet es necesaria para el ingress TCP de Redis y RabbitMQ y para que OVEG
  # llegue por peering. --internal-only: sin IP pública; lo público va por Cloudflare.
  az containerapp env create --name "$ENVIRONMENT_NAME" --resource-group "$RESOURCE_GROUP" \
    --location "$LOCATION" \
    --infrastructure-subnet-resource-id "$SUBNET_ID" --internal-only true \
    --logs-workspace-id "$LOG_ID" --logs-workspace-key "$LOG_KEY" \
    --output none
fi

ENV_DOMAIN="$(az containerapp env show --name "$ENVIRONMENT_NAME" --resource-group "$RESOURCE_GROUP" \
  --query properties.defaultDomain -o tsv)"
ENV_STATIC_IP="$(az containerapp env show --name "$ENVIRONMENT_NAME" --resource-group "$RESOURCE_GROUP" \
  --query properties.staticIp -o tsv)"
echo "     dominio $ENV_DOMAIN -> $ENV_STATIC_IP"

say "     Peering con $ODOO_VNET (OVEG) y DNS privado del entorno"
VECOM_VNET_ID="$(az network vnet show --resource-group "$RESOURCE_GROUP" --name "$VNET_NAME" --query id -o tsv)"
ODOO_VNET_ID="$(az network vnet show --resource-group "$ODOO_VNET_RG" --name "$ODOO_VNET" --query id -o tsv)"
if ! az network vnet peering show --resource-group "$RESOURCE_GROUP" --vnet-name "$VNET_NAME" \
    --name vecom-to-odoo-vegusa &>/dev/null; then
  az network vnet peering create --resource-group "$RESOURCE_GROUP" --vnet-name "$VNET_NAME" \
    --name vecom-to-odoo-vegusa --remote-vnet "$ODOO_VNET_ID" --allow-vnet-access --output none
  echo "     + peering $VNET_NAME -> $ODOO_VNET"
fi
if ! az network vnet peering show --resource-group "$ODOO_VNET_RG" --vnet-name "$ODOO_VNET" \
    --name odoo-vegusa-to-vecom &>/dev/null; then
  az network vnet peering create --resource-group "$ODOO_VNET_RG" --vnet-name "$ODOO_VNET" \
    --name odoo-vegusa-to-vecom --remote-vnet "$VECOM_VNET_ID" --allow-vnet-access --output none
  echo "     + peering $ODOO_VNET -> $VNET_NAME"
fi
# Las apps se resuelven como <app>.<defaultDomain>: comodín a la IP privada del entorno.
if ! az network private-dns zone show --resource-group "$RESOURCE_GROUP" --name "$ENV_DOMAIN" &>/dev/null; then
  az network private-dns zone create --resource-group "$RESOURCE_GROUP" --name "$ENV_DOMAIN" --output none
fi
if ! az network private-dns record-set a show --resource-group "$RESOURCE_GROUP" \
    --zone-name "$ENV_DOMAIN" --name '*' &>/dev/null; then
  az network private-dns record-set a add-record --resource-group "$RESOURCE_GROUP" \
    --zone-name "$ENV_DOMAIN" --record-set-name '*' --ipv4-address "$ENV_STATIC_IP" --output none
fi
for link in "vecom:$VECOM_VNET_ID" "odoo-vegusa:$ODOO_VNET_ID"; do
  if ! az network private-dns link vnet show --resource-group "$RESOURCE_GROUP" \
      --zone-name "$ENV_DOMAIN" --name "${link%%:*}" &>/dev/null; then
    az network private-dns link vnet create --resource-group "$RESOURCE_GROUP" \
      --zone-name "$ENV_DOMAIN" --name "${link%%:*}" --virtual-network "${link#*:}" \
      --registration-enabled false --output none
    echo "     + DNS privado enlazado a ${link%%:*}"
  fi
done

# ---------------------------------------------------------------------
say "5/10 Azure Files para Redis y RabbitMQ: $STORAGE_ACCOUNT"
if ! az storage account show --name "$STORAGE_ACCOUNT" --resource-group "$RESOURCE_GROUP" &>/dev/null; then
  [[ "$(az storage account check-name --name "$STORAGE_ACCOUNT" --query nameAvailable -o tsv)" == "true" ]] \
    || die "el nombre de cuenta $STORAGE_ACCOUNT no está disponible; usa STORAGE_ACCOUNT=<otro>"
  az storage account create --name "$STORAGE_ACCOUNT" --resource-group "$RESOURCE_GROUP" \
    --location "$LOCATION" --sku Standard_LRS --kind StorageV2 \
    --min-tls-version TLS1_2 --allow-blob-public-access false --output none
fi
STORAGE_KEY="$(az storage account keys list --account-name "$STORAGE_ACCOUNT" \
  --resource-group "$RESOURCE_GROUP" --query '[0].value' -o tsv)"
for share in redis-data rabbitmq-data; do
  az storage share-rm create --storage-account "$STORAGE_ACCOUNT" --resource-group "$RESOURCE_GROUP" \
    --name "$share" --quota 10 --output none 2>/dev/null || true
  # El nombre de storage del entorno coincide con el del share
  az containerapp env storage set --name "$ENVIRONMENT_NAME" --resource-group "$RESOURCE_GROUP" \
    --storage-name "$share" --azure-file-account-name "$STORAGE_ACCOUNT" \
    --azure-file-account-key "$STORAGE_KEY" --azure-file-share-name "$share" \
    --access-mode ReadWrite --output none
  echo "     = $share"
done

# ---------------------------------------------------------------------
# upsert_app NOMBRE IMAGEN INGRESS TRANSPORTE TARGET_PORT EXPOSED_PORT CPU MEM
# Usa SECRETS y ENVVARS. En apps existentes no toca la imagen (la gestiona el
# workflow): solo actualiza secretos y variables.
upsert_app() {
  local name=$1 image=$2 ingress=$3 transport=$4 target=$5 exposed=$6 cpu=$7 mem=$8

  if az containerapp show --name "$name" --resource-group "$RESOURCE_GROUP" &>/dev/null; then
    echo "     ~ $name (actualizando secretos y variables)"
    if [[ ${#SECRETS[@]} -gt 0 ]]; then
      az containerapp secret set --name "$name" --resource-group "$RESOURCE_GROUP" \
        --secrets "${SECRETS[@]}" --output none
    fi
    if [[ ${#ENVVARS[@]} -gt 0 ]]; then
      az containerapp update --name "$name" --resource-group "$RESOURCE_GROUP" \
        --container-name "$name" --set-env-vars "${ENVVARS[@]}" --output none
    fi
    # Un cambio solo de secretos no crea revisión: se reinicia para que los lea.
    local revision
    revision="$(az containerapp show --name "$name" --resource-group "$RESOURCE_GROUP" \
      --query properties.latestRevisionName -o tsv)"
    az containerapp revision restart --name "$name" --resource-group "$RESOURCE_GROUP" \
      --revision "$revision" --output none
    return 0
  fi

  echo "     + $name"
  local args=(
    --name "$name" --resource-group "$RESOURCE_GROUP"
    --environment "$ENVIRONMENT_NAME"
    --image "$image"
    --container-name "$name"
    --user-assigned "$IDENTITY_ID"
    --registry-server "$ACR_SERVER" --registry-identity "$IDENTITY_ID"
    --ingress "$ingress" --target-port "$target"
    --cpu "$cpu" --memory "$mem"
    # Una sola réplica: orchestrator y synapse tienen jobs programados y
    # consumers que se duplicarían; Redis y RabbitMQ no escalan horizontalmente.
    --min-replicas 1 --max-replicas 1
    --output none
  )
  if [[ "$transport" == "tcp" ]]; then
    args+=(--transport tcp --exposed-port "$exposed")
  else
    # Tráfico interno del entorno por HTTP (http://<app>), sin redirección a HTTPS
    args+=(--allow-insecure)
  fi
  if [[ ${#SECRETS[@]} -gt 0 ]]; then args+=(--secrets "${SECRETS[@]}"); fi
  if [[ ${#ENVVARS[@]} -gt 0 ]]; then args+=(--env-vars "${ENVVARS[@]}"); fi

  az containerapp create "${args[@]}"
}

# patch_app NOMBRE SUBCOMANDO ARGS... -> patch_template.py sobre la plantilla actual
patch_app() {
  local name=$1; shift
  local cmd=$1; shift
  az containerapp show --name "$name" --resource-group "$RESOURCE_GROUP" -o json > "$TMP_DIR/show.json"
  python3 "$SCRIPT_DIR/patch_template.py" "$cmd" "$TMP_DIR/show.json" "$TMP_DIR/patch.json" "$@"
  az containerapp update --name "$name" --resource-group "$RESOURCE_GROUP" \
    --yaml "$TMP_DIR/patch.json" --output none
}

# Imagen de la app: la última del ACR si ya existe, si no la temporal.
initial_image() {
  if az acr repository show --name "$ACR_NAME" --image "vecom/$1:latest" &>/dev/null; then
    printf '%s' "${ACR_SERVER}/vecom/$1:latest"
  else
    printf '%s' "$PLACEHOLDER_IMAGE"
  fi
}

# ---------------------------------------------------------------------
say "6/10 Redis y RabbitMQ (con volumen de Azure Files)"
SECRETS=(); ENVVARS=()
upsert_app "$APP_REDIS" "${ACR_SERVER}/redis:7-alpine" internal tcp 6379 6379 0.25 0.5Gi
# AOF activado para que los datos del share sobrevivan a reinicios.
patch_app "$APP_REDIS" container --container "$APP_REDIS" --args redis-server --appendonly yes
# uid/gid del usuario redis de la imagen alpine
patch_app "$APP_REDIS" volume --volume redis-data --storage redis-data \
  --container "$APP_REDIS" --mount-path /data \
  --mount-options "uid=999,gid=1000,dir_mode=0750,file_mode=0640,nobrl"

build_config RABBITMQ_PASSWORD
ENVVARS=(
  "RABBITMQ_DEFAULT_USER=$(env_get RABBITMQ_USER)"
  "RABBITMQ_DEFAULT_PASS=secretref:$(secret_name RABBITMQ_PASSWORD)"
  # Nombre de nodo fijo: el directorio de datos va por nombre de nodo y el
  # hostname cambia en cada réplica; sin esto cada reinicio empezaría vacío.
  "RABBITMQ_NODENAME=rabbit@localhost"
)
upsert_app "$APP_RABBIT" "${ACR_SERVER}/rabbitmq:4-management" internal tcp 5672 5672 0.5 1Gi
# Solo mnesia va al share: .erlang.cookie (en /var/lib/rabbitmq) exige permisos
# 400 que SMB no respeta. uid/gid del usuario rabbitmq de la imagen oficial.
patch_app "$APP_RABBIT" volume --volume rabbitmq-data --storage rabbitmq-data \
  --container "$APP_RABBIT" --mount-path /var/lib/rabbitmq/mnesia \
  --mount-options "uid=999,gid=999,dir_mode=0750,file_mode=0640,nobrl"

# Hosts y puertos internos, comunes a orchestrator y synapse (sustituyen a los del .env)
PLATFORM_ENV=(
  "SERVER_PORT=8080"
  "REDIS_HOST=${APP_REDIS}" "REDIS_PORT=6379"
  "RABBITMQ_HOST=${APP_RABBIT}" "RABBITMQ_PORT=5672"
)

# ---------------------------------------------------------------------
say "7/10 synapse-bridge y core-orchestrator"
# El ERP se lee del lakehouse del Link to Fabric (FABRIC_*), ya no de Synapse.
build_config \
  FABRIC_DRIVER_CLASS FABRIC_JDBC_URL FABRIC_USERNAME FABRIC_PASSWORD \
  FABRIC_POOL_SIZE FABRIC_CONNECTION_TIMEOUT FABRIC_MAX_LIFETIME \
  NISSAN_DRIVER_CLASS NISSAN_JDBC_URL NISSAN_USERNAME NISSAN_PASSWORD \
  SYNC_STOCK_CRON SYNC_ITEMS_CRON SYNC_EXISTENCIAS_CRON SYNC_CRON_ZONE \
  RABBITMQ_USER RABBITMQ_PASSWORD
ENVVARS+=("${PLATFORM_ENV[@]}")
upsert_app "$APP_SYNAPSE" "$(initial_image synapse-bridge)" internal http 8080 0 1.0 2Gi

# Sin SERVER_TLS_*: el TLS lo termina Cloudflare y la app sirve HTTP.
build_config \
  JWT_SECRET JWT_ISSUER JWT_TTL_MINUTES CONNECTION_CREDENTIALS_ENCRYPTION_KEY \
  MYSQL_HOST MYSQL_PORT MYSQL_USER MYSQL_PASSWORD MYSQL_DATABASE \
  MYSQL_MAX_OPEN_CONNS MYSQL_MAX_IDLE_CONNS MYSQL_CONN_MAX_LIFETIME_MINUTES MYSQL_CONN_MAX_IDLE_TIME_MINUTES \
  RABBITMQ_USER RABBITMQ_PASSWORD SIE_API_TOKEN \
  TOKEN_REFRESH_POLL_INTERVAL_MINUTES TOKEN_REFRESH_LOOKAHEAD_MINUTES \
  SYNC_QUEUE_POLL_INTERVAL_SECONDS SYNC_QUEUE_BATCH_SIZE \
  COMPATIBILITIES_FIX_RUN_AT_HOUR COMPATIBILITIES_FIX_RUN_AT_MINUTE \
  LISTING_DISCOVERY_RUN_AT_HOUR LISTING_DISCOVERY_RUN_AT_MINUTE \
  EXCHANGE_RATE_RUN_AT_HOUR EXCHANGE_RATE_RUN_AT_MINUTE \
  SERVER_SHUTDOWN_TIMEOUT_SECONDS
ENVVARS+=("${PLATFORM_ENV[@]}")
# external en entorno interno = visible desde la VNet y OVEG, no desde internet
upsert_app "$APP_ORCH" "$(initial_image core-orchestrator)" external http 8080 0 0.5 1Gi

# ---------------------------------------------------------------------
say "8/10 Cloudflare Tunnel $CF_TUNNEL_NAME -> $CF_HOSTNAME"
cf_api() {
  local method=$1 path=$2 data=${3:-}
  local args=(-sS -X "$method" "https://api.cloudflare.com/client/v4/${path}"
    -H "Authorization: Bearer ${CLOUDFLARE_API_TOKEN}" -H "Content-Type: application/json")
  [[ -n "$data" ]] && args+=(--data "$data")
  local out
  out="$(curl "${args[@]}")"
  if [[ "$(jq -r '.success' <<<"$out")" != "true" ]]; then
    die "Cloudflare API $method $path: $(jq -c '.errors' <<<"$out")"
  fi
  printf '%s' "$out"
}

CF_ZONE_JSON="$(cf_api GET "zones?name=${CF_ZONE}")"
CF_ZONE_ID="$(jq -r '.result[0].id // empty' <<<"$CF_ZONE_JSON")"
CF_ACCOUNT_ID="$(jq -r '.result[0].account.id // empty' <<<"$CF_ZONE_JSON")"
[[ -n "$CF_ZONE_ID" ]] || die "zona $CF_ZONE no accesible con este token"

CF_TUNNEL_ID="$(cf_api GET "accounts/${CF_ACCOUNT_ID}/cfd_tunnel?name=${CF_TUNNEL_NAME}&is_deleted=false" \
  | jq -r '.result[0].id // empty')"
if [[ -z "$CF_TUNNEL_ID" ]]; then
  # config_src=cloudflare: el ingress se gestiona desde la API/panel, el
  # contenedor solo necesita el token.
  CF_TUNNEL_ID="$(cf_api POST "accounts/${CF_ACCOUNT_ID}/cfd_tunnel" \
    "{\"name\":\"${CF_TUNNEL_NAME}\",\"config_src\":\"cloudflare\"}" | jq -r '.result.id')"
  echo "     + túnel $CF_TUNNEL_ID"
fi
# cloudflared corre dentro de la imagen del dashboard (start.sh), así que el
# origen es el nginx en localhost:80.
cf_api PUT "accounts/${CF_ACCOUNT_ID}/cfd_tunnel/${CF_TUNNEL_ID}/configurations" "$(jq -nc \
  --arg h "$CF_HOSTNAME" \
  '{config: {ingress: [{hostname: $h, service: "http://localhost:80"}, {service: "http_status:404"}]}}')" >/dev/null

CF_TARGET="${CF_TUNNEL_ID}.cfargotunnel.com"
CF_RECORD="$(cf_api GET "zones/${CF_ZONE_ID}/dns_records?name=${CF_HOSTNAME}" | jq -c '.result[0] // empty')"
if [[ -z "$CF_RECORD" ]]; then
  cf_api POST "zones/${CF_ZONE_ID}/dns_records" "$(jq -nc --arg n "$CF_HOSTNAME" --arg c "$CF_TARGET" \
    '{type: "CNAME", name: $n, content: $c, proxied: true, comment: "vecom middleware (Azure Container Apps)"}')" >/dev/null
  echo "     + DNS $CF_HOSTNAME -> $CF_TARGET"
elif [[ "$(jq -r '.content' <<<"$CF_RECORD")" != "$CF_TARGET" ]]; then
  die "$CF_HOSTNAME ya existe y apunta a $(jq -r '.content' <<<"$CF_RECORD"); no se sobrescribe"
fi

# El token del conector va directo al secreto de la Container App.
CF_TUNNEL_TOKEN="$(cf_api GET "accounts/${CF_ACCOUNT_ID}/cfd_tunnel/${CF_TUNNEL_ID}/token" | jq -r '.result')"

# ---------------------------------------------------------------------
say "9/10 admin-dashboard (nginx + cloudflared)"
SECRETS=("cloudflare-tunnel-token=${CF_TUNNEL_TOKEN}")
ENVVARS=("API_UPSTREAM=http://${APP_ORCH}" "TUNNEL_TOKEN=secretref:cloudflare-tunnel-token")
# Visible en la VNet (entorno interno); desde internet solo por el túnel de Cloudflare.
# nginx y cloudflared comparten 0.25 vCPU / 0.5 GiB (ambos consumen muy poco).
upsert_app "$APP_DASHBOARD" "$(initial_image admin-dashboard)" external http 80 0 0.25 0.5Gi
unset CF_TUNNEL_TOKEN

# ---------------------------------------------------------------------
say "10/10 MySQL por private endpoint y OIDC de GitHub Actions"
# Nada de reglas de firewall por IP: las IPs de salida del entorno son el grupo
# compartido de la región (~180, de muchos clientes). MySQL se alcanza por un
# private endpoint en la VNet; MYSQL_HOST (vecomdb.mysql.database.azure.com)
# resuelve a la IP privada gracias a la zona privatelink enlazada a la VNet.
if ! az network vnet subnet show --resource-group "$RESOURCE_GROUP" --vnet-name "$VNET_NAME" \
    --name private-endpoints &>/dev/null; then
  az network vnet subnet create --resource-group "$RESOURCE_GROUP" --vnet-name "$VNET_NAME" \
    --name private-endpoints --address-prefixes "$PE_SUBNET_PREFIX" --output none
fi
if ! az network private-endpoint show --resource-group "$RESOURCE_GROUP" --name "${MYSQL_SERVER}-pe" &>/dev/null; then
  az network private-endpoint create --resource-group "$RESOURCE_GROUP" --name "${MYSQL_SERVER}-pe" \
    --location "$LOCATION" --vnet-name "$VNET_NAME" --subnet private-endpoints \
    --private-connection-resource-id "$(az mysql flexible-server show --resource-group "$RESOURCE_GROUP" \
      --name "$MYSQL_SERVER" --query id -o tsv)" \
    --group-id mysqlServer --connection-name "${MYSQL_SERVER}-pe-conn" --output none
  echo "     + private endpoint ${MYSQL_SERVER}-pe"
fi
MYSQL_ZONE=privatelink.mysql.database.azure.com
az network private-dns zone show --resource-group "$RESOURCE_GROUP" --name "$MYSQL_ZONE" &>/dev/null \
  || az network private-dns zone create --resource-group "$RESOURCE_GROUP" --name "$MYSQL_ZONE" --output none
az network private-dns link vnet show --resource-group "$RESOURCE_GROUP" --zone-name "$MYSQL_ZONE" --name vecom &>/dev/null \
  || az network private-dns link vnet create --resource-group "$RESOURCE_GROUP" --zone-name "$MYSQL_ZONE" \
    --name vecom --virtual-network "$VNET_NAME" --registration-enabled false --output none
az network private-endpoint dns-zone-group show --resource-group "$RESOURCE_GROUP" \
    --endpoint-name "${MYSQL_SERVER}-pe" --name default &>/dev/null \
  || az network private-endpoint dns-zone-group create --resource-group "$RESOURCE_GROUP" \
    --endpoint-name "${MYSQL_SERVER}-pe" --name default \
    --private-dns-zone "$MYSQL_ZONE" --zone-name mysql --output none

CLIENT_ID="$(az ad app list --display-name "$APP_REGISTRATION" --query '[0].appId' -o tsv)"
[[ -n "$CLIENT_ID" ]] || die "no existe la app registration $APP_REGISTRATION"
cred_name="gh-${DEPLOY_BRANCH}"
if ! az ad app federated-credential list --id "$CLIENT_ID" --query "[?name=='${cred_name}']" -o tsv | grep -q .; then
  az ad app federated-credential create --id "$CLIENT_ID" --parameters "{
    \"name\": \"${cred_name}\",
    \"issuer\": \"https://token.actions.githubusercontent.com\",
    \"subject\": \"repo:${GITHUB_REPO}:ref:refs/heads/${DEPLOY_BRANCH}\",
    \"audiences\": [\"api://AzureADTokenExchange\"]
  }" --output none
  echo "     + OIDC rama ${DEPLOY_BRANCH}"
fi
# Solo los identificadores de Azure van a GitHub; las contraseñas se quedan en Azure.
existing_secrets="$(gh secret list --repo "$GITHUB_REPO" --json name -q '.[].name')"
for pair in "AZURE_CLIENT_ID=$CLIENT_ID" "AZURE_TENANT_ID=$TENANT_ID" "AZURE_SUBSCRIPTION_ID=$SUBSCRIPTION_ID"; do
  if ! grep -qx "${pair%%=*}" <<<"$existing_secrets"; then
    gh secret set "${pair%%=*}" --repo "$GITHUB_REPO" --body "${pair#*=}"
  fi
done

# ---------------------------------------------------------------------
say "LISTO"
echo "  Dashboard:                      https://${CF_HOSTNAME}"
echo "  MercadoLibre redirect_uri:      https://${CF_HOSTNAME}/meli_callback"
echo "  MercadoLibre notificaciones:    https://${CF_HOSTNAME}/meli_notifications"
echo
echo "  Desde OVEG (red privada, peering):"
echo "    https://${APP_ORCH}.${ENV_DOMAIN}      (core-orchestrator)"
echo "    https://${APP_DASHBOARD}.${ENV_DOMAIN}         (dashboard)"
echo
echo "  Si alguna app quedó con la imagen temporal, lanza el primer despliegue:"
echo "    gh workflow run deploy-containerapps.yml --repo ${GITHUB_REPO} --ref ${DEPLOY_BRANCH}"
