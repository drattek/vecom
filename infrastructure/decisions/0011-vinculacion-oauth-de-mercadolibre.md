# ADR 0011 — Vinculación OAuth de las conexiones de MercadoLibre

- Estado: IMPLEMENTADO (2026-11-25); pendiente de probar contra MercadoLibre.
- Alcance: `core-orchestrator` y `admin-dashboard`.
- Sin cambios de esquema.

## Contexto

La autorización se iniciaba con una ruta pública que no enviaba ningún dato de
la conexión. El callback, también público, aceptaba un `connectionId` en la URL
y, si faltaba, usaba la conexión 2. Esto permitía guardar tokens en la conexión
equivocada y que un tercero reemplazara los tokens de una conexión autorizando
su propia cuenta. No había botón en el dashboard.

## Decisión

- El inicio es una ruta protegida:
  `POST /api/channel-connections/{id}/mercadolibre/authorize`
  (`connections.manage_credentials`). Devuelve la URL de MercadoLibre. Se
  eliminó `GET /api/marketplaces/mercadolibre/oauth/authorize`.
- La URL lleva `state`: 32 bytes aleatorios, guardado en Redis durante 10
  minutos y consumido con `GETDEL` (un solo uso). Guarda la conexión, quién
  inició y si se permite cambiar de cuenta. Si Redis se pierde, solo hay que
  iniciar de nuevo.
- `GET /meli_callback` sigue público y su dirección no cambia. Sin un `state`
  válido no toca ninguna conexión. Ya no acepta `connectionId` ni tiene una
  conexión por defecto.
- Antes de guardar, la cuenta autorizada debe coincidir con la de la conexión.
  Si no se conoce la anterior, no puede ser la de otra conexión activa. La
  opción "Quiero cambiar la cuenta" omite ambas comprobaciones.
- La cuenta se guarda en el ajuste `meli_user_id`, al vincular y en cada
  renovación del token, para conocerla incluso si después cambia la aplicación.
- Tokens, expiración y cuenta se guardan en una sola transacción. Un intento
  fallido no modifica la conexión.
- El callback redirige a `/settings/marketplace-connections` con un código de
  resultado; el `code` y el texto de los errores no se propagan.
- La redirección es relativa: `redirect_url` debe apuntar al dominio del
  dashboard, cuyo nginx reenvía `/meli_callback` al core.

## Editar aplicación

- `GET` y `PUT /api/channel-connections/{id}/mercadolibre/app`
  (`connections.manage_credentials`) leen y guardan `app_id`, `redirect_url` y
  `client_secret` de una conexión, sin cambiar su ID ni sus publicaciones.
- El `client_secret` se guarda cifrado y nunca se devuelve. Vacío conserva el
  actual; es obligatorio si cambia el `app_id`.
- `redirect_url` debe ser https, terminar en `/meli_callback` y no llevar
  parámetros.
- Al cambiar de aplicación sin cuenta registrada, se identifica con el access
  token vigente y se guarda en la misma transacción. No se renuevan tokens: un
  respaldo hecho antes de guardar sigue siendo válido.

## Límites

- La comprobación de cuenta no aplica si la conexión no tiene cuenta conocida
  y la autorizada no está en ninguna otra (primera vinculación).
- No implementa PKCE: la aplicación de MercadoLibre debe tenerlo desactivado.

## Cambiar de aplicación de MercadoLibre

Los tokens de una aplicación no sirven con otra.

Se puede migrar una conexión a la vez: cada una guarda su aplicación.

1. Desplegar esta versión.
2. Configurar en la aplicación nueva los mismos permisos funcionales (incluido
   "Venta y envío de productos"), el `redirect_url` y PKCE desactivado.
3. Respaldar los ajustes justo antes del cambio (el `refresh_token` cambia en
   cada renovación).
4. Usar "Editar aplicación" en la conexión y vincularla de inmediato. Hasta
   entonces sus tokens dejan de renovarse, y la publicación y las ventas fallan
   al vencer el token vigente.
5. Probar una publicación existente y "Sincronizar ahora" en Ventas.
