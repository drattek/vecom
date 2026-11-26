# ADR 0010 — Sincronización de ventas de MercadoLibre y Odoo (solo lectura)

- Estado: IMPLEMENTADO (2026-11-25); **pendiente de verificar contra las cuentas reales** (ver la última sección).
- Alcance: `core-orchestrator` (job, persistencia, API) y `admin-dashboard` (vista "Ventas").
- DDL: `infrastructure/mysql/2026-11-25_sales_sync.sql` (se ejecuta a mano).

## Contexto

La plataforma publica en MercadoLibre y Odoo, pero no muestra las ventas que
generan esas conexiones. Hoy ningún proceso ni conector las copia.

El webhook `/meli_notifications` **no se usa**: la URL de callback configurada en
MercadoLibre no es la de este servidor. Su procesador y las tablas
`ecom_meli_*` quedan fuera de este diseño y no se modifican ni se borran.

## Alcance acordado

- Consulta periódica cada 4 horas (configurable).
- Carga inicial de 30 días.
- Solo lectura: no mueve stock, no crea ni modifica pedidos, no responde al comprador.
- De Odoo, solo ventas del ecommerce. Las órdenes de CRM y mostrador se excluyen.
- Quien tenga acceso a la vista de ventas ve también el detalle con los datos del cliente.
- Acceso controlado por RBAC (ADR 0009).

Fuera de alcance: Amazon, mensajes con el comprador, reclamos, conciliación con
Mercado Pago, exportación a Excel e indicadores.

## Decisión

### Flujo

La lectura de ventas va de la API del marketplace a `core-orchestrator`, que la
guarda en MySQL. Usa las mismas conexiones y credenciales que la publicación. No
pasa por `synapse-bridge`, Redis ni RabbitMQ.

### Job

- Nuevo módulo `internal/application/sales`, con un lector por canal detrás de una
  interfaz común, y un scheduler bajo `safe.Supervise`.
- Procesa las conexiones activas de los canales MercadoLibre y Odoo. El error de
  una conexión no detiene a las demás.
- Al arrancar, ejecuta las conexiones cuya última corrida correcta sea más
  antigua que el intervalo.
- Usa los limitadores de peticiones existentes de cada canal.
- Variables: `SALES_SYNC_ENABLED` (por defecto `false`),
  `SALES_SYNC_INTERVAL_HOURS` (4), `SALES_SYNC_BACKFILL_DAYS` (30),
  `SALES_SYNC_OVERLAP_MINUTES` (60).
- `SALES_SYNC_ENABLED` solo gobierna las corridas periódicas. "Sincronizar
  ahora" funciona aunque esté apagado, para poder probar una conexión a la vez.
- Una conexión que falla se reintenta a los 30 minutos. Diez fallos seguidos de
  ventas detienen la corrida.
- En la búsqueda no se vuelve a pedir el detalle de una venta completa cuya
  fecha de modificación no cambió: repetir una carga interrumpida cuesta pocas
  llamadas.

### Ventana de consulta

- La primera corrida de cada conexión fija un **piso**: su inicio menos 30 días.
  Las ventas creadas antes del piso nunca se importan, aunque se modifiquen después.
- La carga inicial filtra por fecha de creación.
- Las corridas siguientes filtran por fecha de modificación, desde el cursor menos
  el traslape.
- El cursor es la hora de **inicio** de la corrida y solo avanza si se
  recorrieron todas las páginas y se guardaron todas las ventas.
- Guardar es idempotente: clave única `(connection_id, external_id)`.
- Además, cada corrida vuelve a consultar por ID las ventas guardadas que siguen
  abiertas (pago pendiente, envío sin entregar ni cancelar, o datos
  incompletos) con hasta 90 días de antigüedad. No se asume que un cambio de
  envío o de entrega modifique la fecha de la orden.
- Un bloqueo con caducidad en la tabla de control impide dos corridas simultáneas
  de la misma conexión (programada y manual, o dos réplicas).

### MercadoLibre

- `GET /users/me` para el ID del vendedor y
  `GET /orders/search?seller=…` con `order.date_created.from` (carga inicial) u
  `order.date_last_updated.from` (incremental), paginado.
- Por venta: detalle de la orden, envío (estado, dirección de destino, costos y
  fecha límite de despacho) y datos de facturación
  (`/orders/billing-info/{site_id}/{billing_info_id}`).
- La fecha límite no se consulta en envíos cancelados ni de Fulfillment.
- Una respuesta parcial (HTTP 206) se acepta. Un dato ausente **no sobrescribe**
  un dato ya guardado.
- Si falla un dato secundario (facturación, costos, fecha límite), la venta se
  guarda marcada como incompleta y se reintenta en la siguiente corrida. Esa
  marca no frena el cursor.
- Los tipos que decodifican las órdenes de ventas son propios de este módulo. Los
  de `handler_orders.go` declaran `pack_id` como objeto, pero la documentación lo
  muestra como número; `variation_id` está declarado como texto y se espera que
  llegue como número. El lector nuevo debe aceptar ambos, con pruebas sobre
  respuestas reales.

### Odoo

- API JSON-2 con el cliente actual y `LoadOdooConnectionValues`.
- Filtro sobre `sale.order`: `website_id != false` y
  `state in ('sent', 'sale', 'cancel')`, más la fecha (`date_order` en la carga
  inicial, `write_date` después).
- Se excluye `draft` porque en el ecommerce de Odoo los carritos sin terminar son
  órdenes en ese estado.
- Ajuste opcional por conexión `sales_website_id` para limitar a un sitio web. Es
  obligatorio en la práctica si dos conexiones apuntan a la misma instancia: sin
  él, cada una importaría las mismas ventas.
- Datos relacionados: líneas, cliente y direcciones (`res.partner`), entregas
  (`stock.picking`), facturas y transacciones de pago.
- La línea de envío (`is_delivery`) se registra como cargo, no como producto.
- Una venta ya guardada que vuelva a `draft` se detecta al consultar las abiertas.

### Modelo de datos

Todas las tablas llevan `created_at`, `updated_at`, `created_by`, `updated_by`
(nulos cuando escribe el job) y `deleted_at`.

| Tabla | Contenido |
|---|---|
| `ecom_sales` | Conexión, ID externo, folio, `pack_id`, estados, fechas, datos del cliente y fiscales al momento de la venta, importes, moneda, marca de incompleta, respuesta original, última sincronización |
| `ecom_sale_items` | ID externo de línea, publicación y variación, SKU, título, cantidad, precios, descuento, impuesto, comisión, `product_id` (nulo si no se vincula) y método de vinculación |
| `ecom_sale_charges` | Desglose: tipo (descuento, envío, impuesto, comisión), quién lo paga (comprador, vendedor, plataforma), importe y descripción |
| `ecom_sale_payments` | ID externo, método, mensualidades, estado, importe, reembolsado, fechas |
| `ecom_sale_shipments` | ID externo, estado y subestado, tipo logístico, transportista, guía, destinatario y dirección, fecha límite de despacho, fechas de envío y entrega, costos |
| `ecom_sale_sync_state` | Por conexión: piso, cursor, bloqueo, última corrida, último éxito, último error y conteos |

- El canal se obtiene por la conexión (`ecom_channel_connections` →
  `ecom_channels`); no se duplica en la venta.
- Claves únicas: `(connection_id, external_id)` en ventas y el ID externo dentro
  de cada venta en líneas, pagos y envíos. Los hijos se actualizan por esa clave.
  Los que desaparezcan se marcan con `deleted_at`; no hay borrado físico.
- Venta e hijos se guardan juntos con `mysql.WithinTx`, y las llamadas HTTP
  quedan fuera de la transacción.
- Los importes se guardan en la moneda original, con su código. **Un importe
  desconocido es `NULL`, no cero.**
- El neto del vendedor es un **estimado** y así se muestra.
- Vinculación con el catálogo: primero por la publicación en
  `ecom_channel_product_map` (conexión + ID externo), si identifica un solo
  producto; después por SKU. Sin coincidencia, `product_id` queda nulo.
- MercadoLibre puede repartir una compra en varias órdenes con el mismo
  `pack_id` y un envío común. Se guarda una venta por orden, la pantalla muestra
  las del mismo paquete y el costo de ese envío no se suma más de una vez.

### Estados

Cada valor normalizado se guarda junto al estado original de la plataforma. Un
valor externo no reconocido se guarda como `unknown` y no detiene la corrida.

| Campo | Valores |
|---|---|
| `order_status` | `pending`, `confirmed`, `cancelled`, `unknown` |
| `payment_status` | `pending`, `paid`, `partially_refunded`, `refunded`, `rejected`, `unknown` |
| `shipping_status` | `pending`, `ready_to_ship`, `shipped`, `delivered`, `not_delivered`, `cancelled`, `not_applicable`, `unknown` |

La tabla de equivalencias por canal se cierra en cada fase con datos reales.

### API y permisos

| Ruta | Permiso |
|---|---|
| `GET /api/sales` (paginada; filtros por canal, conexión, estados, fechas y búsqueda) | `sales.view` |
| `GET /api/sales/{id}` | `sales.view` |
| `GET /api/sales/sync-status` (también alimenta el filtro de conexiones) | `sales.view` |
| `POST /api/sales/sync` (una conexión o todas; responde 202) | `sales.sync` |

- `sales.view` da acceso a la lista y al detalle, incluidos los datos del cliente.
- `sales.sync` permite "Sincronizar ahora". No sustituye a `sales.view`.
- Se agregan al seed `2026-11-18_rbac_permissions_seed.sql` (idempotente) y a
  `routePermissions`. Ningún rol existente los recibe solo; se asignan en
  Configuración → Roles. Admin tiene acceso por defecto.
- En el frontend: `sales: "Ventas"` en `RESOURCE_LABELS`, entrada "Ventas" en el
  menú lateral filtrada por `sales.view`, rutas con `RequirePermission` y botón de
  sincronizar con `<Can any={["sales.sync"]}>`.
- `sales.view` no concede acceso a las rutas de conexiones ni a sus credenciales.

### Datos personales

- No se escriben datos del cliente ni respuestas completas en los logs, tampoco
  dentro de los mensajes de error.
- La respuesta original se guarda para auditoría y **no** se expone por la API.

### Vista "Ventas"

- Lista: fecha, folio, canal, conexión, cliente, número de productos, total,
  estados de venta, pago y envío, y fecha límite de despacho.
- Detalle: cliente y facturación, productos (con enlace al catálogo o aviso "sin
  vincular"), desglose del precio, pagos y envío.
- Estado de sincronización por conexión: último éxito y último error. La pantalla
  indica que los datos pueden tener hasta un intervalo de retraso.

## Implementación

- Job, lectores y consultas: `internal/application/sales/`.
- Persistencia: `sales_repository.go`, `sales_view_repository.go` y
  `sale_sync_state_repository.go`.
- Transporte: `handler_sales.go` en los paquetes de MercadoLibre y Odoo.
- Frontend: `pages/sales/`, `components/sales/`, `lib/sales.ts`.
- Pruebas: reglas y lectores con servidores simulados (la orden de ejemplo es un
  recorte de la documentación oficial). `integration_test.go` cubre el guardado
  contra un MySQL desechable y se omite sin `SALES_TEST_MYSQL_DSN`; pasó con
  MySQL 8.4.
- El lector de Odoo pide solo los campos que `fields_get` reporta. Sin permiso
  sobre un modelo relacionado, importa sin esos datos y lo muestra como aviso de
  la conexión. Si no hay entregas, guarda el destino como una fila de envío.
- El neto estimado se calcula al consultar; no se guarda.
- `sale_fee` se trata como comisión **por unidad**: en el ejemplo oficial, con 2
  unidades y `sale_fee` 11.07, `marketplace_fee` es 22.14.

## Puesta en marcha

1. Ejecutar `2026-11-25_sales_sync.sql`.
2. Desplegar con `SALES_SYNC_ENABLED` apagado.
3. Asignar `sales.view` y `sales.sync` en Configuración → Roles.
4. Usar "Sincronizar ahora" en una conexión de MercadoLibre y comparar varias
   ventas con el portal; repetir con Odoo.
5. Activar `SALES_SYNC_ENABLED=true`.

## Por verificar contra las cuentas reales

- MercadoLibre: nombre y comportamiento del filtro por fecha de modificación (la
  página de Global Selling usa `last_updated.from`); si un cambio de envío mueve
  esa fecha; límite de paginación; forma real de `/shipments/{id}` con
  `x-format-new` y `views=destination` (el lector acepta las dos documentadas);
  que `sale_fee` sea por unidad; qué datos del comprador y de facturación
  entrega a esta aplicación.
- Odoo: versión; campos disponibles (envían los módulos de entregas, facturación,
  pagos y localización mexicana); si hay órdenes `cancel` que en realidad sean
  carritos abandonados; si los pagos por transferencia quedan en `sent`.

## Consecuencias

- Una venta o un cambio puede tardar hasta un intervalo en verse. "Sincronizar
  ahora" lo reduce cuando haga falta.
- Las llamadas crecen con las ventas abiertas: cada una se vuelve a consultar en
  cada corrida. Si resulta costoso, habrá que limitar su antigüedad.
- La base pasa a guardar datos personales y fiscales de compradores. Queda
  pendiente definir cuánto tiempo se conservan.
- Descontar stock o crear pedidos a partir de estas ventas requiere otra ADR.
