# ADR 0011 — Vinculación OAuth de las conexiones de MercadoLibre

- Estado: IMPLEMENTADO (2026-11-25); pendiente de probar contra MercadoLibre.
- Alcance: `core-orchestrator` y `admin-dashboard`.
- DDL: `infrastructure/mysql/2026-11-25_connection_settings_backups.sql` (se
  ejecuta a mano, antes de desplegar).

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
- Usa PKCE con `S256`: cada intento genera un verificador aleatorio que se
  guarda junto al `state` (misma caducidad y uso único). La URL lleva
  `code_challenge` y `code_challenge_method`; el canje del `code` lleva
  `code_verifier` además del `client_secret`. Quien valida es MercadoLibre. La
  renovación de tokens no cambia.
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

## Respaldo automático y restauración

- Antes de editar la aplicación, vincular o restaurar se respaldan, de esa
  conexión, `app_id`, `client_secret`, `redirect_url`, los tokens, su
  expiración y `meli_user_id`. Respaldo y cambio van en la misma transacción:
  si el respaldo falla, no cambia nada.
- `ecom_connection_settings_backups` guarda un JSON cifrado completo. La API
  solo devuelve fecha, usuario, motivo y `app_id`.
- Se muestran los 5 más recientes por conexión; los anteriores se dan de baja
  lógica. Sus datos cifrados siguen en la base; falta definir cuándo
  eliminarlos.
- `GET /api/channel-connections/{id}/mercadolibre/backups` y
  `POST …/backups/{backupId}/restore` (`connections.manage_credentials`).
- Restaurar repone todos los ajustes respaldados y da de baja los de esa lista
  que no existían entonces. Después comprueba los tokens (renovándolos si
  vencieron) y avisa si no sirven; eso no deshace la restauración.
- Un respaldo tomado al vincular después de cambiar la aplicación contiene la
  aplicación nueva con los tokens anteriores.
- Un candado por conexión serializa renovación, vinculación, edición y
  restauración, para no respaldar una renovación a medias. Vale para una sola
  réplica; con más, habría que moverlo a la base o a Redis. La edición genérica
  de ajustes (`/api/connection-settings`) no lo usa ni genera respaldo.

### Pruebas

`mercadolibre_backups_integration_test.go` (MercadoLibre simulado, se omite sin
`BACKUPS_TEST_MYSQL_DSN`) pasó con MySQL 8.4 antes de agregar PKCE; la
comprobación del verificador que se le añadió después no se ha ejecutado. Detectó que reescribir un valor
sin cifrar sin cambios, en el mismo segundo, hacía fallar el guardado (MySQL
reporta 0 filas y `Update` lo toma como "no encontrado"). Ahora no se reescribe
lo que no cambió. La API genérica de ajustes usa `Update` directamente y
conserva ese comportamiento.

## Límites

- La comprobación de cuenta no aplica si la conexión no tiene cuenta conocida
  y la autorizada no está en ninguna otra (primera vinculación).
- PKCE se envía siempre. La documentación lo marca opcional y obligatorio si la
  aplicación lo activa; falta comprobar contra MercadoLibre una aplicación con
  la opción desactivada y otra con ella activada.

## Cambiar de aplicación de MercadoLibre

Los tokens de una aplicación no sirven con otra.

Se puede migrar una conexión a la vez: cada una guarda su aplicación.

1. Ejecutar el script de respaldos.
2. Desplegar esta versión.
3. Configurar en la aplicación nueva los mismos permisos funcionales (incluido
   "Venta y envío de productos") y el `redirect_url`. PKCE puede activarse.
4. Usar "Editar aplicación" en la conexión y vincularla de inmediato. Hasta
   entonces sus tokens dejan de renovarse, y la publicación y las ventas fallan
   al vencer el token vigente.
5. Probar una publicación existente y "Sincronizar ahora" en Ventas.
