# Events Guide

## Propósito

Definir reglas para eventos y mensajería en la arquitectura orientada a eventos.

## Flujo oficial

ERP -> Synapse Bridge -> Redis/RabbitMQ -> Core -> Marketplace

## Reglas de publicación y consumo

- `synapse-bridge` publica cambios provenientes del ERP.
- `core-orchestrator` consume eventos y ejecuta lógica de negocio.
- Consumidores deben ser idempotentes y tolerantes a duplicados.
- Todo evento debe ser trazable (timestamp, origen, entidad, tipo de cambio).
- **Paridad obligatoria publisher/consumer**: todo routing key que `synapse-bridge`
  declara y publica en `sync.exchange` (`RabbitConfig.java` + `EventPublisher.java`)
  debe tener un consumer registrado y arrancado en `core-orchestrator`
  (`cmd/consumers_init.go` + `internal/interfaces/consumers/`). Si se agrega, renombra
  o elimina un evento en un lado, el otro lado se actualiza en el mismo cambio — nunca
  queden desincronizados.
- Colas actuales (`sync.exchange`, topic exchange), fuente de verdad = `RabbitConfig.java`:

  | Routing key / queue | Consumer en core-orchestrator |
  |---|---|
  | `item.page.processed` | `ERPPageProcessedConsumer` |
  | `item.sync.completed` | `ERPCompletedConsumer` |
  | `item.sync.failed` | `ERPSyncFailedConsumer` |
  | `stock.page.processed` | `StockPageProcessedConsumer` |
  | `stock.sync.completed` | `StockSyncCompletedConsumer` |
  | `stock.sync.failed` | `StockSyncFailedConsumer` |
  | `nissan.existencias.page.processed` | `NissanExistenciasPageProcessedConsumer` |
  | `nissan.existencias.sync.completed` | `NissanExistenciasSyncCompletedConsumer` |
  | `nissan.existencias.sync.failed` | `NissanExistenciasSyncFailedConsumer` |

- **Paridad de payload**: los structs Go en `internal/domain/events.go`
  (`SyncCompletedEvent`, `SyncFailedEvent`, `PageProcessedEvent`) deben tener un tag
  `json` por cada componente de su record Java equivalente en `dto/` (mismo nombre
  exacto — `Jackson2JsonMessageConverter` serializa por nombre de componente, sin
  pluralizar ni transformar). Un campo con nombre distinto deserializa en cero/vacío
  sin error visible. Si se agrega o renombra un campo en un record Java, el tag `json`
  correspondiente en Go se actualiza en el mismo cambio.

## Stock: reconciliación por ausencia y limpieza de Redis

`synapse-bridge` lee las fuentes de stock filtrando `stock > 0`
(`RE_VEXISTENCIAS.RELA_EXISTENCIAACTUAL > 0`, `dyn.ItemInventLocation.Disponible > 0`)
para no arrastrar decenas de miles de filas en cero. Consecuencia: cuando un
almacén pasa de 1 a 0, su fila deja de venir en el feed — la ausencia no se
distingue de "no cambió".

Por eso los consumers `stock.sync.completed` y `nissan.existencias.sync.completed`,
después de escribir todo el feed en MySQL y **antes** del refresh de listings en
los marketplaces:

1. **Reconcilian por ausencia** (`reconcileZeroedStock`, `sync_stock_reconcile.go`):
   bajan a 0 las filas de `ecom_product_stock` de esa fuente (`ecom_products.source_id`)
   que quedaron en un valor > 0 y no vinieron en la corrida. Salvaguardas: solo se
   tocan filas cuyo almacén apareció al menos una vez en la corrida; y si en un
   almacén > 50% de sus posiciones con stock > 0 intentaran bajar a 0
   (`stockReconcileMaxZeroRatio`), ese almacén se omite entero (feed probablemente
   truncado). Cada baja deja su movimiento en `ecom_stock_movements`. Es
   idempotente (las filas ya en 0 no vuelven a salir).
2. **Borran de Redis las claves ya consumidas** (`DeleteKeys`): las que dejaron de
   venir no vuelven a escribirse y quedarían como "fantasmas" re-sincronizándose
   para siempre. Se borra la lista exacta de claves escaneadas, no un `SCAN`+`DEL`
   por patrón, para no pisar una corrida del bridge que esté en curso.

Ver ADR `infrastructure/decisions/0002-reconciliacion-stock-por-ausencia.md`.

## Naming recomendado

- `<entidad>.<accion>`
- Ejemplos: `product.updated`, `inventory.updated`, `price.updated`

## Contrato mínimo de evento

- `eventName`
- `eventId`
- `occurredAt`
- `source`
- `entityId`
- `payload`

## Checklist de eventos

1. ¿El evento respeta el naming?
2. ¿El payload evita campos ambiguos?
3. ¿El consumidor es idempotente?
4. ¿Existe manejo de reintentos?
5. ¿La lógica de negocio queda en core?
