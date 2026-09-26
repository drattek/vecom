#!/usr/bin/env bash
# Atajo para desplegar el satélite PSec sin teclear el token ni las variables.
# Lee el token de flota del repo PSec y llama a deploy-satellite.sh con valores
# por defecto. Todo es sobreescribible por argumento o variable de entorno.
#
#   ./infrastructure/psec/install.sh                 # vecom-dashboard, roles scanner,proxy
#   ./infrastructure/psec/install.sh vecom-orchestrator
#   PSEC_ROLES=scanner ./infrastructure/psec/install.sh vecom-synapse
#   ./infrastructure/psec/install.sh vecom-dashboard 80:localhost:80
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

APP="${1:-vecom-dashboard}"
PUBLISH="${2:-80:localhost:80}"
export PSEC_ROLES="${PSEC_ROLES:-scanner,proxy}"

TOKEN_FILE="${PSEC_TOKEN_FILE:-$HOME/PycharmProjects/PSec/psec-scan/deploy-certs/auth.token}"
[[ -f "$TOKEN_FILE" ]] || { echo "ERROR: no encuentro el token en $TOKEN_FILE (exporta PSEC_TOKEN_FILE)" >&2; exit 1; }
export PSEC_ENROLL_TOKEN="$(tr -d '\r\n' < "$TOKEN_FILE")"

exec "$SCRIPT_DIR/../azure/deploy-satellite.sh" "$APP" "$PUBLISH"
