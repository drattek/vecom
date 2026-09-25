#!/usr/bin/env bash
# =====================================================================
# Bootstrap de Azure Container Apps para el middleware vecom.
#
# Crea (o actualiza) en el resource group VECOM:
#   - Entorno de Container Apps sobre vecom-vnet/containerapps-subnet
#   - Identidad administrada para hacer pull del ACR sin contraseñas
#   - Azure Files (cuenta vecomacafiles) para que Redis y RabbitMQ no pierdan
#     sus datos al reiniciarse
#   - 5 Container Apps, todas con ingress INTERNO (ninguna URL pública de Azure):
#       vecom-dashboard     admin-dashboard (nginx + cloudflared en la misma imagen)
#       vecom-orchestrator  core-orchestrator (HTTP 8080)
#       vecom-synapse       synapse-bridge    (HTTP 8080)
#       vecom-redis         redis             (TCP 6379, datos en Azure Files)
#       vecom-rabbitmq      rabbitmq          (TCP 5672, datos en Azure Files)
#   - Cloudflare Tunnel "vecom-middleware" gestionado desde Cloudflare:
#       https://vecom-api.odo.mx -> cloudflared -> nginx del dashboard (localhost:80)
#     No toca el túnel de Odoo (odoo19-tunnel-vegusa) ni el DNS de vecom.odo.mx.
#   - Reglas de firewall de MySQL para las IPs de salida del entorno
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
SUBNET_NAME="${SUBNET_NAME:-containerapps-subnet}"
LOG_WORKSPACE="${LOG_WORKSPACE:-workspace-hNab}"
IDENTITY_NAME="${IDENTITY_NAME:-vecom-apps}"
STORAGE_ACCOUNT="${STORAGE_ACCOUNT:-vecomacafiles}"
MYSQL_SERVER="${MYSQL_SERVER:-vecomdb}"
GITHUB_REPO="${GITHUB_REPO:-drattek/vecom}"
APP_REGISTRATION="${APP_REGISTRATION:-gh-actions-vecom}"
DEPLOY_BRANCH="${DEPLOY_BRANCH:-vecom}"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/vecom.env}"

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
# Admite finales CRLF y valores entre comillas simples o dobles.
env_get() {
  local line value
  line="$(tr -d '\r' < "$ENV_FILE" | grep -E "^${1}=" | tail -1 || true)"
  [[ -z "$line" ]] && return 0
  value="${line#*=}"
  if [[ ${#value} -ge 2 ]]; then
    local first="${value:0:1}" last="${value: -1}"
    if [[ ( "$first" == '"' && "$last" == '"' ) || ( "$first" == "'" && "$last" == "'" ) ]]; then
      value="${value:1:${#value}-2}"
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
  SUBNET_ID="$(az network vnet subnet show --resource-group "$RESOURCE_GROUP" \
    --vnet-name "$VNET_NAME" --name "$SUBNET_NAME" --query id -o tsv)"
  LOG_ID="$(az monitor log-analytics workspace show --resource-group "$RESOURCE_GROUP" \
    --workspace-name "$LOG_WORKSPACE" --query customerId -o tsv)"
  LOG_KEY="$(az monitor log-analytics workspace get-shared-keys --resource-group "$RESOURCE_GROUP" \
    --workspace-name "$LOG_WORKSPACE" --query primarySharedKey -o tsv)"
  # La VNet es necesaria para el ingress TCP de Redis y RabbitMQ.
  az containerapp env create --name "$ENVIRONMENT_NAME" --resource-group "$RESOURCE_GROUP" \
    --location "$LOCATION" \
    --infrastructure-subnet-resource-id "$SUBNET_ID" \
    --logs-workspace-id "$LOG_ID" --logs-workspace-key "$LOG_KEY" \
    --output none
fi

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
upsert_app "$APP_ORCH" "$(initial_image core-orchestrator)" internal http 8080 0 0.5 1Gi

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
# Ingress interno: la única entrada pública es el túnel de Cloudflare.
# nginx y cloudflared comparten 0.25 vCPU / 0.5 GiB (ambos consumen muy poco).
upsert_app "$APP_DASHBOARD" "$(initial_image admin-dashboard)" internal http 80 0 0.25 0.5Gi
unset CF_TUNNEL_TOKEN

# ---------------------------------------------------------------------
say "10/10 Firewall de MySQL y OIDC de GitHub Actions"
OUTBOUND_IPS="$(az containerapp show --name "$APP_ORCH" --resource-group "$RESOURCE_GROUP" \
  --query 'properties.outboundIpAddresses' -o tsv | tr '\t' '\n' | sort -u)"
for ip in $OUTBOUND_IPS; do
  rule="aca-${ENVIRONMENT_NAME}-${ip//./-}"
  az mysql flexible-server firewall-rule create --resource-group "$RESOURCE_GROUP" \
    --name "$MYSQL_SERVER" --rule-name "$rule" \
    --start-ip-address "$ip" --end-ip-address "$ip" --output none
  echo "     + MySQL permite $ip"
done

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
echo "  Si alguna app quedó con la imagen temporal, lanza el primer despliegue:"
echo "    gh workflow run deploy-containerapps.yml --repo ${GITHUB_REPO} --ref ${DEPLOY_BRANCH}"
