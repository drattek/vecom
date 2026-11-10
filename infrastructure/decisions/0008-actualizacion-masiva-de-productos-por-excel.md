# ADR 0008 — Actualización masiva de productos por Excel (y videos de YouTube)

- **Estado:** aceptado — implementado en código (2026-11-10); falta ejecutar el DDL a mano
  (`infrastructure/mysql/2026-11-10_product_bulk_update.sql`)
- **Fecha:** 2026-11-10
- **Servicios:** `services/core-orchestrator`, `services/admin-dashboard`

## Contexto

Hay que actualizar en bloque nombre, marca, descripciones, SEO, fotos, video y medidas de
muchos productos a partir de un Excel (`sku, marca, titulo_optimizado, descripcion_corta,
descripcion_larga, foto_1…foto_7, foto_aplicacion, video, ancho_cm, alto_cm, largo_cm,
diametro, peso_kg`). Una fila toca varias tablas y valida hasta 9 URLs con HEAD, así que no
cabe en una petición HTTP síncrona.

## Decisión

### 1. Ejecución asíncrona, igual que la importación masiva de MercadoLibre

Lote → ítems → worker → reintento → reporte, calcado de `meli_bulk_import`:

- `ecom_product_bulk_update_batches` (un Excel; guarda `source_id` y `storage_disk_id` elegidos)
  y `ecom_product_bulk_update_items` (una fila del Excel; `payload` JSON con lo que pide,
  `steps` JSON con el resultado de cada paso).
- El worker (`product_bulk_update.Service.Start`, 4 en paralelo, `FOR UPDATE … SKIP LOCKED`)
  corre bajo `safe.Supervise`; cada fila bajo `safe.Do`. Filas `running` huérfanas se
  reencolan tras 15 min.
- Estados: `pending`, `running`, `done`, `failed`, `missing` (SKU inexistente). Las filas
  inválidas del Excel nacen `failed` sin `payload` (no reintentables: se corrige el archivo
  y se vuelve a subir).
- UI: `/products/bulk-update` (botón "Actualización masiva" en Productos), misma vista que
  Importar: contadores por estado, polling mientras haya filas en cola, historial, reintento
  por fila/en bloque y reporte `.xlsx` por lote. `GET /api/product-bulk-update/template`
  descarga la plantilla.

### 2. Reglas de actualización

- `sku` debe coincidir exactamente con `ecom_products.sku` (que es `UNIQUE` global). La
  **source** elegida al subir el archivo es una *guarda*, no un desambiguador: si el producto
  pertenece a otra source, la fila falla y no se toca.
- **Celda vacía = no tocar** ese campo. Nunca borra información.
- `titulo_optimizado` → `ecom_products.name` y `ecom_product_seo.meta_title`;
  `descripcion_corta` → `ecom_products.short_description` y `ecom_product_seo.meta_description`;
  `descripcion_larga` → solo `ecom_products.description`. El registro SEO se crea si no existe
  o se actualiza (conservando `keywords`).
- `marca` → se busca por nombre exacto (sin distinguir mayúsculas) en `ecom_brands`; si no
  existe la fila falla **sin modificar nada** (no se crean marcas).
- Medidas: solo se actualizan las presentes; las demás conservan su valor. Si el producto no
  tenía medidas, las faltantes toman el default del alta masiva de dimensiones (1; diámetro =
  ancho). El volumen lo recalcula el repositorio.
- **Fotos reemplazan todas** las imágenes del producto (soft delete + alta en orden). La
  galería es `foto_1…foto_7` (las que existan, en orden) y **`foto_aplicacion` siempre al
  final**, así que la cantidad de fotos es variable. La primera es la portada (`is_first`).
  Todas las URLs se validan antes de tocar nada: si una falla, el producto conserva sus
  imágenes. Si no hay ninguna foto en la fila, no se tocan las existentes.
- **El video se agrega** a los existentes (no reemplaza); el mismo enlace no se duplica.
- Una fila se ejecuta en 3 etapas independientes e idempotentes: (a) datos generales + SEO +
  medidas en **una** transacción (`mysql.WithinTx`, ADR 0001); (b) imágenes (su propia
  transacción); (c) video. Reintentar la fila completa es seguro.

### 2b. Resincronización opcional hacia las conexiones

Al subir el archivo se pueden marcar conexiones (checkboxes; hoy las de MercadoLibre y Odoo,
las que tienen `FullRefresher`). Se guardan en `ecom_product_bulk_update_batches.sync_connection_ids`
(DDL `2026-11-10_product_bulk_update_connections.sql`) y el servidor rechaza una conexión
inexistente o de un canal sin resincronización (`ValidateResyncConnection`).

- Tras guardar la fila en la base local, el worker corre el paso `resync`: llama a
  `channel_listings.ResyncProductListings` por conexión, el mismo flujo del botón
  "Resincronizar" del detalle (envía todos los campos que el canal permite cambiar, sin
  comparar contra lo anterior).
- **Solo actualiza publicaciones existentes**; un producto sin publicación en esa conexión no
  es error (se anota "sin publicación"). No crea publicaciones nuevas.
- Solo corre si todos los pasos anteriores salieron bien; si no, queda omitido para no publicar
  un estado a medias. Si la resincronización falla, la fila queda `failed` y al reintentarla se
  repite todo (idempotente).
- Las resincronizaciones se serializan con un mutex del servicio, porque las APIs de los
  marketplaces comparten presupuesto de peticiones; el resto de las etapas siguen en paralelo.

### 3. Videos de YouTube sin cambiar el esquema

`ecom_product_videos` solo apunta a `ecom_files`, y un video de YouTube no es un archivo. Se
guarda como una fila más de `ecom_files`:

| columna | valor |
|---|---|
| `path` | URL canónica `https://www.youtube.com/watch?v=ID` |
| `file_type` | `video` |
| `mime_type` / `extension` | `video/youtube` / `youtube` |
| `size` | `0` (no hay archivo) |
| `disk_id` | el disco elegido al subir (no significa nada para estos registros) |

- `product_image_import.ParseYouTubeURL` reconoce `watch?v=`, `youtu.be/`, `shorts/`,
  `embed/`, `live/`, normaliza y valida el ID de 11 caracteres; un enlace de YouTube sin ID
  válido es un error (no se trata como archivo). No se hace HEAD a YouTube (devolvería HTML).
- `ImportVideosForProduct` y `ReplaceProductVideo` lo usan, así que el alta manual de videos
  del detalle del producto también acepta YouTube.
- El dashboard distingue por `mime_type = 'video/youtube'`: muestra "YouTube" como formato,
  sin tamaño, y el botón "Ver" abre el reproductor incrustado; para archivos abre `<video>`.
- **Consecuencia:** cualquier consumidor nuevo de `ecom_product_videos` (marketplaces, Odoo)
  debe tener en cuenta que `path` puede ser un enlace de YouTube y no un archivo.

## Alternativa descartada

Agregar `provider`/`external_url` a `ecom_product_videos` y volver `file_id` nullable: modelo
más limpio, pero exige DDL manual y tocar repos, DTOs y UI. Se puede migrar después sin
perder datos (los registros con `mime_type = 'video/youtube'` son fáciles de identificar).

## Limitaciones conocidas

- Un registro de video de YouTube guarda un `disk_id` y un `size` sin significado real.
- No hay columna de posición en `ecom_product_images`: el orden de la galería es el `id`, que
  aquí se inserta en el orden del Excel.
- Este módulo no dispara explícitamente ningún refresco hacia los marketplaces (ML, Odoo); lo
  que se propague depende de los flujos de sincronización existentes.
