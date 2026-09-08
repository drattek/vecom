# API Guide

## Propósito

Estandarizar contratos HTTP y reglas de diseño para endpoints entre UI y backend.

## Convenciones REST

- Recursos en plural.
- Verbos HTTP semánticos (`GET`, `POST`, `PUT`, `PATCH`, `DELETE`).
- Respuestas consistentes con códigos HTTP correctos.

## Reglas de seguridad

- Endpoints privados requieren autenticación.
- No exponer secretos o credenciales en payloads.
- Validar entradas en el borde (handlers/controllers).

## Contratos mínimos

- Requests y responses tipados.
- Errores con estructura estable y mensaje accionable.
- Paginación para listados (`offset`, `pageSize`, `total`) cuando aplique.

## Versionado y compatibilidad

- Evitar cambios breaking sin migración planificada.
- Documentar cambios de contrato antes de liberar.

## Checklist previo a publicar endpoint

1. ¿La lógica está en capa de aplicación o dominio?
2. ¿El contrato está alineado a entidades del core?
3. ¿La validación de entrada está completa?
4. ¿El endpoint evita dependencias prohibidas (ERP/Synapse directos)?

## Autenticación

- `POST /api/login` — público. Body `{ username, password }`. Devuelve `{ accessToken, tokenType: "Bearer", expiresAt, user }` y persiste el token en `ecom_api_token`.
- `POST /api/logout` — protegido (Authorization Bearer). Revoca el token del llamador (`ecom_api_token.revoked_at`). Responde `204 No Content`. Idempotente: revocar un token ya revocado/expirado/desconocido no falla. El cliente debe limpiar su sesión local pase lo que pase.
- El resto de rutas `/api/*` (salvo callbacks de marketplaces) requieren `Authorization: Bearer <accessToken>`; `JWTMiddleware.RequireAuth` valida firma, emisor, expiración y que la fila en `ecom_api_token` siga activa.

## Mercado Libre - Category Predictor

- Endpoint: GET /api/marketplaces/mercadolibre/category-predictor
- Seguridad: requiere Authorization Bearer (ruta protegida por JWT).
- Query params:
	- connectionId (required): id de ecom_channel_connections a usar para cargar ecom_connection_settings.
	- title (required): titulo del producto para prediction.
	- siteId (optional): si no se envía, se intenta resolver desde ecom_connection_settings con key site_id.
	- limit (optional): cantidad de resultados (1 a 8). Default: 1.

### Flujo interno

1. Valida entrada (connectionId, title, limit).
2. Resuelve site_id (query param o setting site_id).
3. Llama a EnsureValidAccessToken(connectionId):
	 - revisa expiration_time con margen preventivo;
	 - si expiró o está por expirar, hace refresh en /oauth/token;
	 - persiste access_token, refresh_token y expiration_time con is_encrypted=true.
4. Ejecuta GET https://api.mercadolibre.com/sites/{SITE_ID}/domain_discovery/search?q={TITLE}&limit={LIMIT} con Bearer token.

### Organización técnica Mercado Libre

- Capa aplicación (orquestación):
	- internal/application/sync/sync_mercadolibre.go
- Capa infraestructura (consumo API por tópico):
	- internal/infrastructure/marketplace/mercadolibre/handler_auth.go
	- internal/infrastructure/marketplace/mercadolibre/handler_categories.go
	- internal/infrastructure/marketplace/mercadolibre/client.go
- Esta separación permite escalar por topics (`handler_items`, `handler_orders`, etc.) sin mezclar lógica de negocio con transporte HTTP.

### Respuesta

- 200 OK:

```json
{
	"connectionId": 12,
	"siteId": "MLA",
	"query": "celular iphone",
	"predictions": [
		{
			"domain_id": "MLA-CELLPHONES",
			"domain_name": "Celulares",
			"category_id": "MLA1055",
			"category_name": "Celulares y Smartphones",
			"attributes": []
		}
	]
}
```

- 400 Bad Request: faltan settings obligatorios, connectionId inválido, title vacío o siteId no disponible.
- 500 Internal Server Error: errores de red/integración con Mercado Libre u otros errores internos.

## Odoo - Test Connection

- Endpoint: GET /api/marketplaces/odoo/test-connection
- Seguridad: requiere Authorization Bearer (ruta protegida por JWT).
- Query params:
	- connectionId (required): id de ecom_channel_connections a usar para cargar ecom_connection_settings / ecom_connection_credentials.
	- productId (optional): si se envía, restringe la búsqueda a ese product.template id (para probar un producto puntual); si se omite, devuelve hasta 20 productos vendibles/publicados.

### Flujo interno

1. Valida connectionId (y productId si viene).
2. Carga y fusiona ecom_connection_settings y ecom_connection_credentials de la conexión en un mapa por key_name (case-insensitive), ya que odoo_url/ApiKey/X-Odoo-Database pueden vivir en cualquiera de las dos tablas según cómo se haya configurado la conexión.
3. Resuelve odoo_url (obligatorio, en settings), ApiKey (obligatorio, en credentials o settings) y X-Odoo-Database (opcional, solo instancias multi-base).
4. Llama a Odoo External JSON-2 API: POST {odoo_url}/json/2/product.template/search_read con header `Authorization: bearer {ApiKey}` y, si aplica, `X-Odoo-Database: {database}`. Domain: `sale_ok = true AND website_published = true` (más `id in [productId]` si se pasó productId). Fields: `id, name, default_code, list_price, qty_available, attribute_line_ids, show_availability, taxes_id, tax_string`. A diferencia de Mercado Libre, Odoo usa API key estática — no hay OAuth ni refresh de token.
5. Persiste el resultado (autenticado/error) en ecom_connection_status vía Upsert; ExpiresAt nunca se setea (la key no expira).

### Organización técnica Odoo

- Capa aplicación (orquestación):
	- internal/application/sync/sync_odoo.go
- Capa infraestructura (consumo API por tópico):
	- internal/infrastructure/marketplace/odoo/client.go
	- internal/infrastructure/marketplace/odoo/handler_products.go
- Mismo criterio de escalado por topics que Mercado Libre (`handler_orders.go`, etc. a futuro).

### Respuesta

- 200 OK:

```json
{
	"connectionId": 12,
	"products": [
		{
			"id": 202,
			"name": "Producto de ejemplo",
			"default_code": "SKU-001",
			"list_price": 199.9,
			"qty_available": 15,
			"attribute_line_ids": [4, 7],
			"show_availability": true,
			"taxes_id": [1],
			"tax_string": "IVA 16%"
		}
	]
}
```

- 400 Bad Request: connectionId inválido, o faltan odoo_url/ApiKey en settings/credentials de la conexión.
- 500 Internal Server Error: errores de red/integración con Odoo u otros errores internos.

## Categorías - Importación desde canales

Flujo de la vista Settings → Categorías para dar de alta categorías locales
navegando el árbol de categorías **externas** de un canal (MercadoLibre u Odoo)
e importando una hoja. Al importar se replica en `ecom_categories` la hoja y sus
ancestros faltantes, y se registra `ecom_channel_category_map`. Capa aplicación:
`internal/application/category_import/service.go`; handler:
`internal/interfaces/http/category_import_handler.go`.

- `GET /api/category-import/sources` — protegido. Un objeto por canal con ≥1
  conexión activa: `{ sources: [{ channelId, channelCode, channelName, supported,
  connections: [{ id, name, environment }] }] }`. `supported` es `true` solo para
  `MERCADOLIBRE` y `ODOO`.
- `GET /api/category-import/tree?connectionId=<id>&externalCategoryId=<id?>` —
  protegido. Un nivel del árbol externo: sin `externalCategoryId` devuelve las
  raíces; con él, los hijos de esa categoría. `{ nodes: [{ externalId, name,
  leaf }] }`. `leaf` es `null` cuando el canal no puede saberlo sin expandir
  (MercadoLibre): el cliente expande y trata "0 hijos" como hoja. `409` si el
  canal no soporta importación; `400` si a la conexión de MercadoLibre le falta
  `site_id` o no está autenticada.
- `POST /api/category-import` — protegido. Body `{ connectionId,
  externalCategoryId, onlySelectedConnection }`. Importa la hoja: crea/matchea
  (por nombre + padre) la jerarquía local y hace `Upsert` de
  `ecom_channel_category_map` para la hoja en **todas** las conexiones activas del
  canal, o solo en `connectionId` si `onlySelectedConnection` es `true`. Responde
  `{ categoryId, name, createdCount, alreadyLinked, mappedConnectionIds }`.
  `422` si la categoría externa no es hoja; `404` si no existe.
- `POST /api/category-import/mapping` — protegido. Body `{ categoryId,
  connectionId, externalCategoryId }`. Vincula (o reemplaza) el mapeo de una
  categoría local **ya existente** con una hoja externa en una conexión — la
  acción "Vincular / Reemplazar" del detalle de categoría. No crea categorías
  locales: valida que la hoja externa lo sea, resuelve su nombre y hace `Upsert`
  de `ecom_channel_category_map` solo para `(categoryId, connectionId)`. Responde
  `{ categoryId, connectionId, externalCategoryId, externalCategoryName,
  replaced }`. `404` si la categoría local no existe; `422` si la externa no es
  hoja.
