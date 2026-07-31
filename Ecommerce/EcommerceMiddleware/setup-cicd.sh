#!/usr/bin/env bash
# =====================================================================
# Bootstrap del CI/CD: infraestructura Azure + OIDC + secrets de GitHub.
#
# Se ejecuta UNA SOLA VEZ. Despues, cada push a dev/prod despliega solo
# a traves de .github/workflows/deploy-middleware.yml
#
# Requisitos:
#   - az login  (con MFA: az login --tenant <tenant-id>)
#   - gh auth login
#   - .env relleno con las credenciales reales
#
# Es idempotente: puedes reejecutarlo sin romper nada.
# =====================================================================
set -euo pipefail

# ---------- Parametros ----------
SUBSCRIPTION_ID="${SUBSCRIPTION_ID:-d88e33f1-2295-43b7-bb43-a2e20c3e4b60}"
TENANT_ID="${TENANT_ID:-e60ad600-4568-4f55-bdad-195ca7bc2861}"
RESOURCE_GROUP="${RESOURCE_GROUP:-VECOM}"
# southcentralus = misma region que el MySQL vecomdb, minima latencia
LOCATION="${LOCATION:-southcentralus}"
ACR_NAME="${ACR_NAME:-acrvecom}"
ENVIRONMENT_NAME="${ENVIRONMENT_NAME:-vecom-env}"
APP_BASE="${APP_BASE:-ecommerce-middleware}"
GITHUB_REPO="${GITHUB_REPO:-drattek/vecom}"
APP_REGISTRATION="${APP_REGISTRATION:-gh-actions-vecom}"
ENV_FILE="${ENV_FILE:-.env}"

BRANCHES=(dev prod)

say() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }

[[ -f "$ENV_FILE" ]] || { echo "ERROR: falta $ENV_FILE (copialo de .env.example)" >&2; exit 1; }

az account set --subscription "$SUBSCRIPTION_ID"

# ---------------------------------------------------------------------
say "1/6  Proveedores de recursos"
az provider register --namespace Microsoft.App --wait
az provider register --namespace Microsoft.OperationalInsights --wait
az provider register --namespace Microsoft.ContainerRegistry --wait

# ---------------------------------------------------------------------
say "2/6  Container Registry: $ACR_NAME"
if ! az acr show --name "$ACR_NAME" --resource-group "$RESOURCE_GROUP" &>/dev/null; then
  # admin deshabilitado: el pull se hace con managed identity, no con password
  az acr create --resource-group "$RESOURCE_GROUP" --name "$ACR_NAME" \
    --sku Basic --admin-enabled false --location "$LOCATION" --output none
fi
ACR_ID="$(az acr show --name "$ACR_NAME" --resource-group "$RESOURCE_GROUP" --query id -o tsv)"

say "     Entorno de Container Apps: $ENVIRONMENT_NAME"
if ! az containerapp env show --name "$ENVIRONMENT_NAME" --resource-group "$RESOURCE_GROUP" &>/dev/null; then
  az containerapp env create --name "$ENVIRONMENT_NAME" --resource-group "$RESOURCE_GROUP" \
    --location "$LOCATION" --output none
fi

# ---------------------------------------------------------------------
say "3/6  App registration para GitHub Actions: $APP_REGISTRATION"
CLIENT_ID="$(az ad app list --display-name "$APP_REGISTRATION" --query '[0].appId' -o tsv)"
if [[ -z "$CLIENT_ID" ]]; then
  CLIENT_ID="$(az ad app create --display-name "$APP_REGISTRATION" --query appId -o tsv)"
  az ad sp create --id "$CLIENT_ID" --output none
  sleep 15   # propagacion en Entra ID
fi
SP_OBJECT_ID="$(az ad sp show --id "$CLIENT_ID" --query id -o tsv)"
echo "     client id: $CLIENT_ID"

say "     Federated credentials (OIDC, sin contrasenas)"
for branch in "${BRANCHES[@]}"; do
  name="gh-${branch}"
  if ! az ad app federated-credential list --id "$CLIENT_ID" --query "[?name=='${name}']" -o tsv | grep -q .; then
    az ad app federated-credential create --id "$CLIENT_ID" --parameters "{
      \"name\": \"${name}\",
      \"issuer\": \"https://token.actions.githubusercontent.com\",
      \"subject\": \"repo:${GITHUB_REPO}:ref:refs/heads/${branch}\",
      \"audiences\": [\"api://AzureADTokenExchange\"]
    }" --output none
    echo "     + rama ${branch}"
  fi
done
# Los jobs usan 'environment:', que emite un subject distinto
for gh_env in development production; do
  name="gh-env-${gh_env}"
  if ! az ad app federated-credential list --id "$CLIENT_ID" --query "[?name=='${name}']" -o tsv | grep -q .; then
    az ad app federated-credential create --id "$CLIENT_ID" --parameters "{
      \"name\": \"${name}\",
      \"issuer\": \"https://token.actions.githubusercontent.com\",
      \"subject\": \"repo:${GITHUB_REPO}:environment:${gh_env}\",
      \"audiences\": [\"api://AzureADTokenExchange\"]
    }" --output none
    echo "     + environment ${gh_env}"
  fi
done

say "     Permisos (minimos: solo push al ACR y gestion de Container Apps)"
RG_SCOPE="/subscriptions/${SUBSCRIPTION_ID}/resourceGroups/${RESOURCE_GROUP}"
az role assignment create --assignee-object-id "$SP_OBJECT_ID" --assignee-principal-type ServicePrincipal \
  --role AcrPush --scope "$ACR_ID" --output none 2>/dev/null || true
az role assignment create --assignee-object-id "$SP_OBJECT_ID" --assignee-principal-type ServicePrincipal \
  --role "Contributor" --scope "$RG_SCOPE" --output none 2>/dev/null || true

# ---------------------------------------------------------------------
say "4/6  Imagen inicial en ACR"
IMAGE="${ACR_NAME}.azurecr.io/${APP_BASE}:bootstrap"
# Se usa buildx (BuildKit) y NO 'az acr build': el builder clasico de ACR Tasks
# falla al exportar las capas de Spring Boot ("layer does not exist").
# --platform linux/amd64 es obligatorio: Container Apps no corre arm64, asi que
# en un Mac Apple Silicon esto va emulado y tarda bastante.
if ! az acr repository show --name "$ACR_NAME" --image "${APP_BASE}:bootstrap" &>/dev/null; then
  az acr login --name "$ACR_NAME"
  docker buildx build --platform linux/amd64 -t "$IMAGE" --push .
fi

# ---------------------------------------------------------------------
say "5/6  Container Apps (dev y prod)"

# Secretos: se leen del .env y viven SOLO en Azure, nunca en GitHub.
SECRETS=(); ENVVARS=()
while IFS= read -r line; do
  [[ -z "$line" || "$line" == \#* || "$line" != *=* ]] && continue
  key="${line%%=*}"; value="${line#*=}"
  # Los fija la plataforma mas abajo; no son secretos y no deben duplicarse.
  [[ "$key" == "SERVER_PORT" || "$key" == "SERVER_SSL_ENABLED" ]] && continue
  secret_name="$(echo "$key" | tr '[:upper:]_' '[:lower:]-')"
  SECRETS+=("${secret_name}=${value}")
  ENVVARS+=("${key}=secretref:${secret_name}")
done < "$ENV_FILE"
ENVVARS+=("SERVER_PORT=8080" "SERVER_SSL_ENABLED=false")

for branch in "${BRANCHES[@]}"; do
  app="${APP_BASE}-${branch}"
  echo "     -> $app"

  if az containerapp show --name "$app" --resource-group "$RESOURCE_GROUP" &>/dev/null; then
    az containerapp secret set --name "$app" --resource-group "$RESOURCE_GROUP" \
      --secrets "${SECRETS[@]}" --output none
    az containerapp update --name "$app" --resource-group "$RESOURCE_GROUP" \
      --set-env-vars "${ENVVARS[@]}" --output none
  else
    # prod arranca con mas recursos que dev
    if [[ "$branch" == "prod" ]]; then CPU=1.0; MEM=2.0Gi; MINR=1; MAXR=3
    else CPU=0.5; MEM=1.0Gi; MINR=0; MAXR=1; fi

    az containerapp create \
      --name "$app" --resource-group "$RESOURCE_GROUP" \
      --environment "$ENVIRONMENT_NAME" \
      --image "$IMAGE" \
      --registry-server "${ACR_NAME}.azurecr.io" --registry-identity system \
      --target-port 8080 --ingress external \
      --secrets "${SECRETS[@]}" --env-vars "${ENVVARS[@]}" \
      --cpu "$CPU" --memory "$MEM" \
      --min-replicas "$MINR" --max-replicas "$MAXR" \
      --system-assigned \
      --output none
  fi

  # La app se autentica contra el ACR con su managed identity (sin passwords)
  PRINCIPAL="$(az containerapp show --name "$app" --resource-group "$RESOURCE_GROUP" \
    --query identity.principalId -o tsv)"
  az role assignment create --assignee-object-id "$PRINCIPAL" --assignee-principal-type ServicePrincipal \
    --role AcrPull --scope "$ACR_ID" --output none 2>/dev/null || true

  # Probes: /actuator/health/liveness y /readiness
  az containerapp update --name "$app" --resource-group "$RESOURCE_GROUP" \
    --output none 2>/dev/null || true
done

# ---------------------------------------------------------------------
say "6/6  Secrets en GitHub"
gh secret set AZURE_CLIENT_ID       --repo "$GITHUB_REPO" --body "$CLIENT_ID"
gh secret set AZURE_TENANT_ID       --repo "$GITHUB_REPO" --body "$TENANT_ID"
gh secret set AZURE_SUBSCRIPTION_ID --repo "$GITHUB_REPO" --body "$SUBSCRIPTION_ID"

echo
say "LISTO"
for branch in "${BRANCHES[@]}"; do
  app="${APP_BASE}-${branch}"
  fqdn="$(az containerapp show --name "$app" --resource-group "$RESOURCE_GROUP" \
    --query 'properties.configuration.ingress.fqdn' -o tsv 2>/dev/null || echo '?')"
  printf '  %-28s https://%s\n' "$app" "$fqdn"
done
echo
echo "  A partir de ahora: push a 'dev' o 'prod' despliega automaticamente."
