#!/usr/bin/env bash
# =====================================================================
# Bootstrap de Azure Container Apps para el middleware vecom.
#
# Crea (o actualiza) en el resource group VECOM:
#   - Entorno de Container Apps sobre vecom-vnet/containerapps-subnet
#   - Identidad administrada para hacer pull del ACR sin contraseñas
#   - 5 Container Apps:
#       vecom-dashboard     admin-dashboard  (ingress EXTERNO, único punto público)
#       vecom-orchestrator  core-orchestrator (interno, HTTP 8080)
#       vecom-synapse       synapse-bridge    (interno, HTTP 8080)
#       vecom-redis         redis             (interno, TCP 6379)
#       vecom-rabbitmq      rabbitmq          (interno, TCP 5672)
#   - Reglas de firewall de MySQL para las IPs de salida del entorno
#   - Credencial federada OIDC para que GitHub Actions despliegue desde 'vecom'
#
# Secretos: se leen de vecom.env (raíz del repo, en .gitignore). Toda clave que
# contenga PASSWORD, SECRET, TOKEN o termine en _KEY se crea como secreto de la
# Container App y la variable la referencia con secretref:. Los valores NUNCA
# se imprimen, no van a GitHub y no quedan en el repo.
#
# Después de esto, cada push a 'vecom' despliega solo con
# .github/workflows/deploy-containerapps.yml (que solo cambia la imagen).
#
# Requisitos: az login, gh auth login, vecom.env relleno.
# Es idempotente: reejecutarlo actualiza secretos y variables.
# =====================================================================
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

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
MYSQL_SERVER="${MYSQL_SERVER:-vecomdb}"
GITHUB_REPO="${GITHUB_REPO:-drattek/vecom}"
APP_REGISTRATION="${APP_REGISTRATION:-gh-actions-vecom}"
DEPLOY_BRANCH="${DEPLOY_BRANCH:-vecom}"
ENV_FILE="${ENV_FILE:-$REPO_ROOT/vecom.env}"

# Deben coincidir con .github/workflows/deploy-containerapps.yml
APP_DASHBOARD=vecom-dashboard
APP_ORCH=vecom-orchestrator
APP_SYNAPSE=vecom-synapse
APP_REDIS=vecom-redis
APP_RABBIT=vecom-rabbitmq

ACR_SERVER="${ACR_NAME}.azurecr.io"
# Imagen temporal para crear las apps antes del primer despliegue del workflow
PLACEHOLDER_IMAGE="mcr.microsoft.com/k8se/quickstart:latest"

say() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }
die() { printf '\033[1;31mERROR: %s\033[0m\n' "$*" >&2; exit 1; }

[[ -f "$ENV_FILE" ]] || die "falta $ENV_FILE (cópialo de vecom.env.example)"

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
  ERP_JDBC_URL ERP_USERNAME ERP_PASSWORD
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

az account set --subscription "$SUBSCRIPTION_ID"

# ---------------------------------------------------------------------
say "1/8  Proveedores de recursos"
az provider register --namespace Microsoft.App --wait
az provider register --namespace Microsoft.OperationalInsights --wait

# ---------------------------------------------------------------------
say "2/8  Identidad administrada $IDENTITY_NAME (pull del ACR)"
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
say "3/8  Imágenes de Redis y RabbitMQ en el ACR (evita el rate limit de Docker Hub)"
for img in redis:7-alpine rabbitmq:4-management; do
  if ! az acr repository show --name "$ACR_NAME" --image "$img" &>/dev/null; then
    az acr import --name "$ACR_NAME" --source "docker.io/library/${img}" --image "$img" --output none
    echo "     + $img"
  fi
done

# ---------------------------------------------------------------------
say "4/8  Entorno de Container Apps: $ENVIRONMENT_NAME"
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
        --set-env-vars "${ENVVARS[@]}" --output none
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
  elif [[ "$ingress" == "internal" ]]; then
    # Tráfico interno del entorno por HTTP (http://<app>), sin redirección a HTTPS
    args+=(--allow-insecure)
  fi
  if [[ ${#SECRETS[@]} -gt 0 ]]; then args+=(--secrets "${SECRETS[@]}"); fi
  if [[ ${#ENVVARS[@]} -gt 0 ]]; then args+=(--env-vars "${ENVVARS[@]}"); fi

  az containerapp create "${args[@]}"
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
say "5/8  Redis y RabbitMQ"
SECRETS=(); ENVVARS=()
upsert_app "$APP_REDIS" "${ACR_SERVER}/redis:7-alpine" internal tcp 6379 6379 0.25 0.5Gi

build_config RABBITMQ_PASSWORD
ENVVARS=(
  "RABBITMQ_DEFAULT_USER=$(env_get RABBITMQ_USER)"
  "RABBITMQ_DEFAULT_PASS=secretref:$(secret_name RABBITMQ_PASSWORD)"
)
upsert_app "$APP_RABBIT" "${ACR_SERVER}/rabbitmq:4-management" internal tcp 5672 5672 0.5 1Gi

# Hosts y puertos internos, comunes a orchestrator y synapse (sustituyen a los del .env)
PLATFORM_ENV=(
  "SERVER_PORT=8080"
  "REDIS_HOST=${APP_REDIS}" "REDIS_PORT=6379"
  "RABBITMQ_HOST=${APP_RABBIT}" "RABBITMQ_PORT=5672"
)

# ---------------------------------------------------------------------
say "6/8  synapse-bridge y core-orchestrator"
build_config \
  ERP_DRIVER_CLASS ERP_JDBC_URL ERP_USERNAME ERP_PASSWORD ERP_POOL_SIZE ERP_CONNECTION_TIMEOUT \
  NISSAN_DRIVER_CLASS NISSAN_JDBC_URL NISSAN_USERNAME NISSAN_PASSWORD \
  FABRIC_DRIVER_CLASS FABRIC_JDBC_URL FABRIC_USERNAME FABRIC_PASSWORD \
  FABRIC_POOL_SIZE FABRIC_CONNECTION_TIMEOUT FABRIC_MAX_LIFETIME \
  SYNC_STOCK_CRON SYNC_ITEMS_CRON SYNC_EXISTENCIAS_CRON SYNC_CRON_ZONE \
  RABBITMQ_USER RABBITMQ_PASSWORD
ENVVARS+=("${PLATFORM_ENV[@]}")
upsert_app "$APP_SYNAPSE" "$(initial_image synapse-bridge)" internal http 8080 0 1.0 2Gi

# Sin SERVER_TLS_*: el TLS lo termina el ingress y la app sirve HTTP.
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
say "7/8  admin-dashboard (ingress público)"
SECRETS=()
ENVVARS=("API_UPSTREAM=http://${APP_ORCH}")
upsert_app "$APP_DASHBOARD" "$(initial_image admin-dashboard)" external http 80 0 0.25 0.5Gi

say "     Firewall de MySQL para las IPs de salida del entorno"
OUTBOUND_IPS="$(az containerapp show --name "$APP_ORCH" --resource-group "$RESOURCE_GROUP" \
  --query 'properties.outboundIpAddresses' -o tsv | tr '\t' '\n' | sort -u)"
for ip in $OUTBOUND_IPS; do
  rule="aca-${ENVIRONMENT_NAME}-${ip//./-}"
  az mysql flexible-server firewall-rule create --resource-group "$RESOURCE_GROUP" \
    --name "$MYSQL_SERVER" --rule-name "$rule" \
    --start-ip-address "$ip" --end-ip-address "$ip" --output none
  echo "     + $ip"
done

# ---------------------------------------------------------------------
say "8/8  OIDC de GitHub Actions para la rama $DEPLOY_BRANCH"
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
  echo "     + rama ${DEPLOY_BRANCH}"
fi
# Solo los identificadores de Azure van a GitHub; las contraseñas se quedan en Azure.
existing_secrets="$(gh secret list --repo "$GITHUB_REPO" --json name -q '.[].name')"
for pair in "AZURE_CLIENT_ID=$CLIENT_ID" "AZURE_TENANT_ID=$TENANT_ID" "AZURE_SUBSCRIPTION_ID=$SUBSCRIPTION_ID"; do
  if ! grep -qx "${pair%%=*}" <<<"$existing_secrets"; then
    gh secret set "${pair%%=*}" --repo "$GITHUB_REPO" --body "${pair#*=}"
  fi
done

# ---------------------------------------------------------------------
FQDN="$(az containerapp show --name "$APP_DASHBOARD" --resource-group "$RESOURCE_GROUP" \
  --query 'properties.configuration.ingress.fqdn' -o tsv)"
say "LISTO"
echo "  Dashboard:  https://${FQDN}"
echo "  MercadoLibre redirect_uri:      https://${FQDN}/meli_callback"
echo "  MercadoLibre notificaciones:    https://${FQDN}/meli_notifications"
echo
echo "  Si alguna app quedó con la imagen temporal, lanza el primer despliegue:"
echo "    gh workflow run deploy-containerapps.yml --repo ${GITHUB_REPO} --ref ${DEPLOY_BRANCH}"
