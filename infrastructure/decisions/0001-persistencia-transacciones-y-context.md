# ADR 0001 — Persistencia: `Querier`, transacciones y propagación de `context`

- **Estado:** aceptado — implementación COMPLETA (2026-09-03)
- **Fecha:** 2026-09-02
- **Servicio:** `services/core-orchestrator`
- **Origen:** auditoría interna (puntos 6, 7, 8, 9, 10) + endurecimiento de robustez del proceso

## Contexto

La auditoría de `core-orchestrator` encontró varios problemas en la capa de
persistencia y en el arranque/parada del proceso:

- **Pool MySQL sin límites** — sin `SetMaxOpenConns` / `SetConnMaxLifetime`.
- **Repos inconsistentes** — unos tomaban `Querier` (re-escopeables a `*sql.Tx`),
  otros `*sql.DB` directo (no transaccionables).
- **`Querier` sin `context`** — solo `Exec/Query/QueryRow`; contradice la regla
  de `coding-standards.md` ("`context.Context` en todos los métodos de I/O") y
  una query lenta no se cancela cuando el cliente HTTP corta.
- **`config.Load()` llamado varias veces** — en `main` y dentro de cada
  constructor de infraestructura.
- **Operaciones multi-sentencia sin transacción** — si la 2ª escritura falla, la
  1ª queda aplicada (posible desincronización, p. ej. `ecom_channel_product_map`
  vs. marketplace).
- **Goroutines de fondo sin `recover`** — un panic tumbaba todo el proceso.

## Decisión

### 1. Un solo `config.Load()`

Se llama **una vez en `main`** y se inyecta `config.Config` (por valor) a cada
constructor de infraestructura (`mysql.NewConnection`, `redis.NewClient`,
`rabbitmq.StartConsumer` → `newConnection`). Ningún otro paquete llama a
`config.Load()`.

### 2. Pool MySQL acotado

`mysql.NewConnection` aplica `SetMaxOpenConns` / `SetMaxIdleConns` /
`SetConnMaxLifetime` / `SetConnMaxIdleTime`, configurables vía `MYSQL_*`
(defaults 25 / 25 / 5m / 5m). `instancias × MYSQL_MAX_OPEN_CONNS` debe quedar
por debajo de `max_connections` menos margen para admin/migraciones.

### 3. Todos los repos toman `Querier`

Cada `NewXxxRepository` recibe `mysql.Querier`, no `*sql.DB`. Como `*sql.DB` y
`*sql.Tx` satisfacen la interfaz, un repo puede correr contra el pool o
escoparse a una transacción sin duplicar código.

### 4. `Querier` propaga `context` (migración por fases)

`Querier` pasa a exigir los métodos `...Context`
(`ExecContext`/`QueryContext`/`QueryRowContext`). Cada método de repo recibe
`ctx context.Context` como primer parámetro y lo propaga al driver.

Durante la migración la interfaz exige **ambos** juegos de métodos (con y sin
`context`) para permitir builds verdes entre fases. Los call sites de dominios
todavía no migrados pasan `context.TODO()` como marcador temporal. Cuando ningún
repo use los métodos sin `context` se eliminan de `Querier` y se verifica que no
queden `context.TODO()` en `internal/application` ni `internal/interfaces`.

### 5. Transacciones solo en casos multi-sentencia

Helper `mysql.WithinTx(ctx, db, func(tx *sql.Tx) error { ... })`: rollback ante
error o panic, commit si no.

- **Una sola sentencia** (`Create`, `Update`, `SoftDelete` simples): se llama al
  repo sobre el pool directamente. NO se abre transacción — un `BEGIN`/`COMMIT`
  por operación agrega un fsync sin ganancia.
- **Multi-sentencia** (crear producto + dimensiones/precios/stock/media; precio
  actual + histórico; asignación masiva; etc.): se corre dentro de `WithinTx`,
  construyendo los repos necesarios sobre el `tx`.

El borde de la transacción vive en la **capa de servicio** (el servicio sabe qué
es una operación lógica). Por eso cada servicio recibe `*sql.DB` en su
constructor además de los repos ya construidos sobre el pool.

### 6. Barrera de panic en goroutines de fondo

Paquete `internal/shared/safe`: `Do` (una unidad de trabajo), `Supervise` (un
loop de fondo, lo reinicia tras panic), `Guard` (convierte panic en `error`).
Aplicado a consumers, workers y schedulers. En HTTP, middleware `recoverJSON`.

## Estructura común de un servicio

```go
type XService struct {
    db         *sql.DB                 // para WithinTx (multi-sentencia)
    repository *mysqlInfra.XRepository // para lecturas / una sola sentencia
    // ...otros repos o servicios
}

func NewXService(db *sql.DB, repository *mysqlInfra.XRepository) *XService { ... }

// lectura / una sola sentencia
func (s *XService) GetX(ctx context.Context, id int64) (*X, error) {
    return s.repository.FindByID(ctx, id)
}

// multi-sentencia
func (s *XService) DoComplexThing(ctx context.Context, in Input) error {
    return mysqlInfra.WithinTx(ctx, s.db, func(tx *sql.Tx) error {
        x  := mysqlInfra.NewXRepository(tx)
        y  := mysqlInfra.NewYRepository(tx)
        // ...varias escrituras; si una falla, ninguna queda aplicada
        return nil
    })
}
```

Los handlers pasan `r.Context()` a cada método de servicio.

## Estado de la migración por fases

Cada fase = un dominio (repos + servicio + handlers), build + `go vet` en verde.

- [x] Fundación: `Querier` transicional (6 métodos), `WithinTx`, los 43 repos sobre `Querier`
- [x] **brands** (repo + servicio + handler + `WithinTx` en `BulkAssignBrands`)
- [x] **categories** (repo + servicio + handler; sin ops multi-sentencia hoy → sin `WithinTx`)
- [x] **products** (`product_repository` + `product_dimensions`/`images`/`videos`/`part_numbers`/`seo`; servicios `products`, `product_media` (×5), `product_image_import`; handlers). `product_attributes` queda en la fase **attributes**. Sin `WithinTx` nuevo (las ops de producto son de una sola escritura efectiva; `CreateProduct` mantiene el best-effort de resolución pendiente por diseño). `product_image_import.Import` tiene un `TODO(persistence)` para envolver `ecom_files`+`ecom_product_images` cuando se migre files/storage_disks.
- [x] **pricing** (`price_list`, `pricing_formula`, `product_prices`, `exchange_rates`, `price_history`; servicios `PriceListService`/`ProductPricesService`/`ExchangeRatesService`/`PricingFormulaService` + `EffectivePriceResolver.ResolveInCurrency` + `PricingFormulaCalculator.CalculatePrice` + `SIEExchangeRateUpdater`; handler). Sin `WithinTx` nuevo: el par "precio actual + histórico" se escribe en los tx helpers de sync (`sync_nissan`/`sync_erp_stock`), que migran a `WithinTx` en la fase **sync**. `BulkUpsertPrices` = una escritura por ítem.
- [x] **inventory** (`branches`, `warehouses`, `product_stock`, `stock_movements`; servicios `BranchService`/`WarehouseService`/`ProductStockService`/`StockMovementsService`; handlers). Sin `WithinTx` nuevo: el par "stock actual + movimiento" (`ecom_product_stock` + `ecom_stock_movements`) se escribe en los tx helpers de sync — migran a `WithinTx` en la fase **sync**.
- [x] **compatibility** (`vehicle_fitments`, `equipment_types`, `equipment_fitments`, `product_vehicle_compatibility`, `product_equipment_compatibility`, `pending_product_vehicle_fitments`, `part_number_supersessions`; 5 servicios; handler). `ResolvePendingFitments` usa `WithinTx` por fila (crear compat + `MarkResolved` juntos). `BulkImportFitments` tiene `TODO(persistence)` (escribe en 3 tablas; hoy recuperable por el manejo idempotente de `*AlreadyExists`). También se completó el threading de `ctx` en los helpers `resolvePendingVehicleFitments`/`resolvePartNumberSupersessions` de la fase products.
- [x] **channels / channel_*** (repos: `channel`, `channel_connection`, `channel_parameters`, `channel_product_map`, `channel_category_map`, `channel_attributes`, `channel_attribute_map`, `channel_sync_queue`, `connection_credentials`/`settings`/`status`; servicios: `channels`, `channel_connections`, `channel_config` ×4, `channel_attributes` ×2, `channel_sync_queue`, `channel_listings`, `channel_attribute_values`; 7 handlers; `marketplace_worker`, schedulers). Sin `WithinTx` nuevo (ops de una escritura efectiva; `channel_attribute_values` provisioning es resolve-or-create idempotente — TODO comentado para envolver si hace falta). Los helpers crypto de `connection_credentials`/`settings` (`encryptIfNeeded`/`decryptIfNeeded`/`ensureEncryptionReady`) NO reciben `ctx` (no hacen I/O).
- [x] **attributes** (`attributes`, `attribute_options`, `product_attributes`; servicios `AttributeService`/`AttributeOptionService`/`ProductAttributeService`; handler). Sin `WithinTx` (ops de una escritura: `SetValue` = `Upsert`). Con esto `channel_attribute_values` quedó 100% con `ctx` real (ya no dependía de repos sin migrar).
- [x] **currencies, sources** (repos + servicios `CurrencyService`/`SourceService` + handlers; CRUD simple, sin `WithinTx`). `findOrCreateDynamicsSource`/`findOrCreateNissanSource` (funcs libres en sync) reciben `ctx`.
- [x] **files, storage_disks** (repos + servicios `FileService`/`StorageDiskService` + handlers; CRUD simple, sin `WithinTx`). `product_image_import.Import` ahora pasa `ctx` a los repos de files/storage; su `TODO(persistence)` sigue abierto (falta reestructurar el loop con lecturas/HTTP intercaladas para envolver los inserts en `WithinTx`; hoy recuperable por dedup de `FindByPath`).
- [x] **credentials / auth** (`auth_repository`; `AuthService` (`Login`/`Logout`/`ValidateToken`) recibe `*sql.DB` + `ctx`; `auth_handler` y `auth_middleware.RequireAuth` pasan `r.Context()`). Sin `WithinTx` (`Login` = 1 escritura). Ahora una request cancelada corta también la validación de token.
- [x] **meli_notifications** (`meli_notification_repository`; `MeliNotificationService.Receive` recibe `*sql.DB` + `ctx`; handler `/meli_notifications` pasa `r.Context()`). Sin `WithinTx` (`Receive` = find-or-create, 1 escritura).
- [x] **migration** (`odoo_categories`, `vecom_sync_products`). `ctx` propagado desde `MigrateCategories`/`Migrate` (que ya lo tenían) por todos los helpers privados (`upsertHierarchy`, `resolveOrCreateProduct`, `resolveBrandID`, `writeMeliDimensions`, `recordChannelCategory`, `vecomOdooCategoryResolver.resolveNode`). Los 17 `context.TODO()` que arrastraba se eliminaron. Sin `WithinTx`: es un endpoint temporal one-off (marcado para borrar).
- [x] **sync** — capa de consumers RabbitMQ (`handler` pasa a `func(context.Context, []byte) error`, `runConsumer` propaga su ctx de vida), `SyncService.Process*` (ya no hacen `context.Background()` interno), `prepare*SyncCache`, y los tx-helpers `nissanSyncTx`/`erpStockSyncTx` (todos sus métodos toman `ctx`). `ProcessNissanExistenciaBatch` y `ProcessERPStock` migrados de `BeginTx`+Commit/Rollback a mano → `mysqlRepo.WithinTx`. Servicios `MercadoLibre*`/`Odoo*`: `ctx` enhebrado por todos los helpers (`resolveOrCreateProduct`, `sumAvailableStock`, `resolveDimensions`, `upsertSetting`, `recordConnectionStatus`, etc.).
- [x] **cierre** — `Querier` reducido a 3 métodos (`ExecContext`/`QueryContext`/`QueryRowContext`). `context.TODO()` == 0 en todo `internal/`. `go build ./... && go vet ./...` en verde.
- [ ] cierre: quitar métodos sin `context` de `Querier`; grep `context.TODO()` == 0 en application/interfaces

## Consecuencias

- **+** Consistencia: toda operación de I/O toma `ctx`; toda operación
  multi-sentencia es atómica.
- **+** Cancelación de punta a punta cuando el cliente HTTP corta.
- **+** Los repos siguen sin lógica de negocio; la transacción se orquesta en el
  servicio.
- **−** Migración amplia (≈267 métodos de repo, ≈24 servicios, ≈36 handlers);
  se hace por fases con `context.TODO()` como puente.
- **−** `Querier` queda temporalmente "gorda" (6 métodos) hasta el cierre.
