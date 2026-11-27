# ADR 0012 — Notificaciones por Telegram

- Estado: IMPLEMENTADO (2026-11-26), las 4 fases; pendiente de probar con
  MySQL, un bot y marketplaces reales.
- Alcance: `core-orchestrator` y `admin-dashboard`.
- DDL: `infrastructure/mysql/2026-11-25_telegram_notifications.sql` (se
  ejecuta a mano, antes de desplegar).

## Contexto

Los errores de sincronización solo se ven en el log o al entrar al dashboard.
Los eventos de fallo del ERP solo se registran en el log. Se quiere avisar a
cada usuario, en privado y sin costo, de lo que él elija.

## Decisión

### Bot y vinculación

- Un solo bot. El token se captura en Configuración → Notificaciones (solo
  `admin`), se guarda cifrado con AES-256-GCM
  (`CONNECTION_CREDENTIALS_ENCRYPTION_KEY`) y la API nunca lo devuelve
  completo.
- El token va en la URL de la Bot API: se oculta de cualquier error antes de
  registrarlo o guardarlo.
- Un bot no puede escribir a un número de teléfono. Cada usuario se vincula
  desde Mi perfil con `https://t.me/<bot>?start=<código>`. El código es
  aleatorio, de un solo uso y con caducidad corta; solo se guarda su hash.
- Los `/start` se reciben con `getUpdates` (no requiere URL pública). Cada
  entorno usa su propio bot: dos procesos con el mismo token chocan. Al copiar
  la base de un entorno a otro, el token viaja con ella.
- Al cambiar de bot, los usuarios deben vincularse de nuevo.
- Solo se atienden chats privados.

### Preferencias y destinatarios

- Cada usuario administra solo su vínculo y sus avisos. Todo empieza apagado.
- No hay filtro por conexión: cada aviso abarca todas las que correspondan.
- Cada tipo exige un permiso (ADR 0009), que se comprueba al configurar y al
  encolar. Los avisos de procesos masivos llegan solo a quien los inició.
- La hora del resumen diario es única, en `America/Mexico_City` por defecto.
  El binario incluye la base de zonas horarias (`time/tzdata`).

### Envío

- Los módulos reportan hechos al servicio de notificaciones, que encola una
  fila por destinatario en `ecom_notification_deliveries`. Un proceso en
  segundo plano (`safe.Supervise`) envía y respeta `retry_after`.
- Un fallo de Telegram nunca interrumpe una sincronización.
- Si el usuario bloqueó el bot, el vínculo queda como `blocked`.
- Lo que no cabe en 4,096 caracteres se envía como archivo.
- Los textos de error se recortan y se limpian de credenciales y datos de
  clientes.

### Avisos

| Tipo | Cuándo | Sin repetidos |
|---|---|---|
| Falla del ERP | Al momento | Un incidente abierto por flujo; se avisa la recuperación |
| Conexión caída | Al momento | Solo al cambiar de correcta a fallida, y al recuperarse |
| Errores de publicación | Al terminar la tanda | Por producto; solo errores nuevos o distintos |
| Pausas por stock | Resumen diario | Una entrada por producto |
| Cierres definitivos | Tras cada revisión, agrupados | Una vez por publicación |
| Procesos masivos | Al terminar | Uno por lote terminado |

### Fallas del ERP

Se avisan tres casos, para los flujos de productos, stock y existencias
Nissan:

1. Llega un evento `*.sync.failed`.
2. La sincronización termina con registros fallidos (falla parcial).
3. No se completa ninguna en `erp_silence_hours` (36 por defecto; el ERP se
   sincroniza a diario, pero procesar todo puede demorar y un margen corto
   generaría alertas falsas). Cubre el caso en que `synapse-bridge` está caído
   y no publica nada.

### Conexiones

Comprobación periódica (`connection_check_minutes`, 60 por defecto) de todas
las conexiones activas de MercadoLibre y Odoo, además de reaccionar a los
fallos de autenticación que ocurran durante la operación normal. Un error
transitorio de red no debe abrir un incidente al primer intento.

### Revisión de cierres

- Programada a las **00:00 y 12:00** (zona configurada), sobre **todas** las
  conexiones activas de MercadoLibre. No depende de las conexiones fijas que
  hoy se refrescan tras la sincronización de stock.
- Usa una consulta propia de solo lectura (`ListingLiveStatuses`) y guarda
  únicamente el paso a `closed`. **No** reutiliza
  `RefreshChannelProductMapStatus[AndCategory]` porque además refleja pausas y
  revisiones y crea categorías y mapeos, algo que no se quiere extender a todas
  las conexiones.
- **Corregido el 2026-11-26** en esa sincronización existente (ver «Estado
  leído de MercadoLibre»).
- Un cierre se detecta como publicación en `closed` sin registro vigente en
  `ecom_channel_listing_closures`, sin importar qué proceso cambió el estado.
- El motivo se consulta en
  `GET /moderations/last_moderation/{ITEM_ID}-ITM` (y, como apoyo,
  `GET /moderations/infractions/{USER_ID}`). Si no hay moderación, se informa
  el `sub_status`.
- Solo se avisan los cierres definitivos atribuibles a MercadoLibre:
  - `definitive`: `sub_status` `forbidden`, `suspended`,
    `suspended_for_prevention`, `freezed`, `held` o `expired`; o, sin uno de
    esos, con moderación vigente.
  - `fixable`: `waiting_for_patch` o `pending_documentation`.
  - `system`: el resto (lo cerró este sistema o el vendedor).
- El historial de infracciones solo se consulta cuando el `sub_status` ya es
  concluyente, porque incluye infracciones antiguas ya resueltas.
- Si no se puede consultar el motivo, se reintenta en la siguiente revisión; al
  segundo reintento se avisa sin motivo. Un cierre sin avisar caduca a los 7
  días.
- Cada conexión se registra por separado: si una falla, solo ella se reintenta
  (cada 30 minutos, hasta el siguiente horario).
- Una publicación que MercadoLibre ya no devuelve solo cuenta si localmente ya
  estaba cerrada.
- **La primera corrida es línea base:** registra los cierres ya existentes
  como notificados, para no enviar todo el histórico.

### Pausas por stock

El resumen agrupa por producto, al momento de enviar, las pausas ocurridas
desde el resumen anterior. Solo incluye productos que siguen pausados con
`paused_reason = 'zero_stock'` y lista sus publicaciones en todas las
conexiones, indicando dónde sigue activo. No se repite en los días siguientes.
El primer resumen cubre 24 horas; tras estar apagado, como máximo 72.

El resumen incluye las pausas propias (`zero_stock`) y las que MercadoLibre
aplica solo (`out_of_stock`). Estas quedan sin `paused_reason` y se reconocen
por un evento `pause` con el mensaje `MarketplaceOutOfStockPauseMessage`. En
las vistas de publicaciones y el dashboard siguen apareciendo como las demás
pausas sin motivo.

### Estado leído de MercadoLibre

- Leer el estado no es enviar: se guarda con `ApplyMarketplaceState`, que no
  toca `last_synced_at` ni `last_error`. Antes, `Refresh` podía omitir un
  cambio de precio o stock pendiente. Aplica a la sincronización de estado y
  categoría, a la de solo estados y a la reconciliación tras un envío
  rechazado.
- Motivo de pausa (`pausedReasonFor`): una pausa propia `zero_stock` se
  conserva mientras siga pausada o en revisión (también en `under_review`; quien
  la usa exige además estado `paused`); `out_of_stock` queda sin motivo; el
  resto es `user`.
- A propósito, las pausas `out_of_stock` **no** se marcan `zero_stock`: ese
  motivo hace que este sistema active la publicación, y pisaría una pausa que
  el vendedor aplique después en MercadoLibre.
- Tras enviar stock 0 sin pausar por cuenta propia, se relee la publicación
  para registrar si MercadoLibre la pausó.
- No repara las filas ya afectadas.

### Tareas con horario

`ecom_scheduled_job_runs` impide ejecutar dos veces el mismo horario. Si el
servicio estaba apagado a la hora prevista, al arrancar ejecuta solo la
corrida pendiente más reciente.

## Permisos por tipo de aviso

Basta uno; `admin` siempre puede. Catálogo en
`internal/application/notifications/catalog.go`.

| Tipo | Permisos |
|---|---|
| `erp_sync_failure` | `dashboard.view` |
| `connection_down` | `connections.view` |
| `publication_errors` | `channel_listings.view`, `reports.view` |
| `stock_pause_summary` | `channel_listings.view`, `products_channels.view` |
| `meli_listing_closed` | `channel_listings.view`, `meli_listings.view` |
| `bulk_process_finished` | `products_import.view`, `products_bulk_update.view` |

## API

- Solo `admin`: `GET` y `PUT /api/notification-settings`.
- Cualquier sesión, siempre sobre el propio usuario:
  - `GET` y `PATCH /api/me/notifications`
  - `PUT /api/me/notifications/preferences`
  - `POST` y `DELETE /api/me/notifications/telegram/link`
  - `POST /api/me/notifications/telegram/test`

La prueba se envía de inmediato, sin pasar por la cola.

## Detalles de la fase 2

- **Incidentes:** solo se abre uno cuando hay alguien a quien avisar. Así,
  quien active un aviso después se entera en la siguiente detección de un
  problema que siga presente. Quien se suscriba con el incidente ya abierto no
  recibe el aviso inicial, pero sí la recuperación.
- Las fallas de un solo evento (`*.sync.failed`) que ocurran sin destinatarios
  no se recuperan después; el silencio del ERP cubre el caso persistente.
- **Falla parcial:** la firma del incidente son los códigos fallidos. Mientras
  sean los mismos no se repite. Con más de 3 se adjunta un CSV.
- Un evento `completed` que no se puede procesar (se reencola) cuenta como
  falla del flujo.
- **Conexiones:** el monitor comprueba con `GET /users/me` en MercadoLibre y
  una lectura de productos en Odoo. Un rechazo de credenciales avisa de
  inmediato; otro error necesita dos fallos seguidos, con la segunda
  comprobación a los 5 minutos. Además, el servicio de tokens de MercadoLibre
  reporta los rechazos que ocurran en la operación normal. Al desactivar una
  conexión, su incidente se cierra sin aviso.
- Las comprobaciones periódicas solo corren con las notificaciones activadas.
- **Procesos masivos:** el monitor lee los lotes terminados en las últimas 2
  horas, sin cambiar los workers. La clave contra duplicados incluye la hora de
  la última fila, así que un reintento vuelve a avisar.
- **Cola:** un envío por segundo; respeta `retry_after`; hasta 6 intentos; lo
  que no se pueda enviar en 24 horas se cancela. Si un envío sale pero no se
  puede marcar como enviado, puede repetirse.

## Detalles de la fase 3

- **Fuente única:** `ecom_channel_listing_events`, sin enganchar cada flujo. Se
  toma el último intento (`publish`, `refresh`, `resync`, `pause`,
  `reactivate`) de cada publicación en conexiones activas. `status_sync` no
  cuenta. Ahora también quedan en la bitácora las publicaciones que fallan por
  producto sin marca o al cargar compatibilidades.
- «Sin stock disponible» no es una falla: se ignora.
- **Tanda:** se avisa tras 3 minutos sin eventos nuevos, o cuando un error
  lleva 30 minutos esperando. La bitácora se revisa cada 2 minutos.
- **Un resumen por tanda:** productos y casos por conexión. Con más de 5 casos
  muestra las 3 causas principales y adjunta un CSV con una fila por producto y
  conexión.
- **Nuevo o distinto:** hay un incidente por producto y conexión que guarda sus
  causas normalizadas (sin identificadores). Solo se avisa si aparece una causa
  que no estaba; que se corrija una de varias no es novedad.
- Se cierra sin aviso cuando el último intento es correcto o no hay errores en
  72 horas. Un reintento correcto que no cambia nada no deja evento, así que
  ese caso se cierra por antigüedad.
- Al activar el aviso por primera vez, el primer resumen incluye los errores
  vigentes de las últimas 72 horas.
- No se cubren los fallos de la cola de publicación que ocurren antes de
  intentar publicar (p. ej. no poder leer el producto).

## Fases

1. Configuración del bot, vinculación, preferencias y mensaje de prueba.
2. Fallas del ERP, conexiones y procesos masivos.
3. Errores de publicación.
4. Resumen de pausas, revisión programada y cierres con motivo.

## Consecuencias

- Si `core-orchestrator` está caído no puede avisar; eso requiere un monitor
  externo.
- Telegram es un servicio externo: los mensajes salen de la infraestructura.
- La revisión de cierres agrega consultas a MercadoLibre dos veces al día.
- Por verificar con la cuenta de México: la documentación de moderaciones se
  consultó en el sitio de Argentina, y la clasificación de cada `sub_status`
  como definitivo o corregible debe confirmarse con respuestas reales.
