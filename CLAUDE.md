# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

Middleware ecommerce (Vegusa): synchronizes product/inventory data from Azure Synapse/ERP into an owned system and distributes it to marketplaces (MercadoLibre, Amazon, Odoo). Three independent services:

- **`services/synapse-bridge`** (Java 17 / Spring Boot, package `com.vegusa.ecommerce`) — the *only* service allowed to connect to Azure Synapse/ERP. Read-only against Synapse/ERP; publishes events to RabbitMQ and writes through to Redis. Never contains product business logic.
- **`services/core-orchestrator`** (Go) — the backend. Owns all business logic, the REST API, marketplace integrations, and MySQL persistence. Never queries Azure Synapse or the ERP directly; only consumes data via Redis/RabbitMQ.
- **`services/admin-dashboard`** (React 19 / TypeScript, Vite) — admin frontend. Only ever talks to `core-orchestrator`'s API; never touches the ERP directly.

Mandatory data flow (do not introduce shortcuts that skip stages without an explicit architectural decision):

```
ERP → Synapse Bridge → Redis / RabbitMQ → Core Orchestrator → Marketplace APIs (MercadoLibre, Amazon, Odoo)
```

### Documentation precedence

When docs conflict, this is the order of truth (highest first):

1. `infrastructure/decisions/*` (ADRs, if present)
2. `infrastructure/docs/*.md` (`domain.md`, `api.md`, `events.md`)
3. `infrastructure/mysql/*.md` (`database.md`, `relationships.md`)
4. `README.md`, `SYSTEM_OVERVIEW.md`, `business-rules.md`, `conventions.md`, `coding-standards.md`, `product-flow.md` (root, complementary context)
5. `.github/prompts/{backend,frontend,bridge}.prompt.md` — per-service canonical checklists; read the one matching the service you're changing before proposing changes.

`AI_CONTENT_WORKFLOW.md` at the repo root defines the full request-classification workflow (frontend/backend/bridge/cross-service) — consult it for anything non-trivial or cross-service.

## Cross-cutting business rules

- **Soft delete only.** No physical deletes; every domain table has `deleted_at`, filtered out on normal reads (`deleted_at IS NULL`).
- **Currency:** prices are stored in USD and MXN via a `currency` relation; every marketplace channel receives prices in **MXN**.
- **Custom attributes** exist only in `core-orchestrator` — the ERP never stores them.
- **Redis is cache only**, never a source of truth. Azure Synapse is read-only.
- `synapse-bridge` connects to multiple ERPs (general/machinery ERP via `ErpDataSourceConfig`, Nissan vehicles ERP via `NisanDataSourceConfig`).
- Audit columns (`created_by`, `updated_by`, `created_at`, `updated_at`) are required on business writes.

## Database (MySQL, `core-orchestrator`)

Schema detail lives in `infrastructure/mysql/database.md` and `relationships.md` — read those before touching persistence. Key points:
- Domain tables use prefix `ecom_`; infra/API-access tables use `vecom_`.
- `ecom_products` is the central catalog table (join point for brand/category/stock/price/media).
- Current-state vs. history split: `ecom_product_stock` (current) vs `ecom_stock_movements` (history); `ecom_product_prices` (current) vs `ecom_price_history` (history).
- Channel integrations: `ecom_channels` → `ecom_channel_connections` → `ecom_connection_credentials` / `ecom_connection_settings` / `ecom_connection_status`. Credentials/settings are sensitive — see `ConnectionCredentialsRepository`'s encrypt/decrypt handling.
- Prefer extending an existing table over creating a parallel one for the same concept; never insert dependents before validating the parent exists.

## Architecture: core-orchestrator (Go)

Layout under `services/core-orchestrator/`:

```
cmd/                                    entrypoints: main.go, mysql_init.go, consumers_init.go
internal/api/                           router.go — chi router wiring
internal/interfaces/http/               HTTP handlers (thin — parse/validate, delegate to application layer)
internal/interfaces/consumers/          RabbitMQ consumers
internal/interfaces/schedulers/         cron-driven jobs
internal/application/<module>/          use cases (brands, categories, channels, channel_connections,
                                         channel_config, credentials, currencies, files, inventory,
                                         pricing, product_media, products, stock_movements,
                                         storage_disks, sync)
internal/domain/                        entities and business rules
internal/infrastructure/mysql/          repositories (generic Create/Update/FindByID/FindPaginated/SoftDelete pattern)
internal/infrastructure/redis/          Redis client + product repo (cache)
internal/infrastructure/rabbitmq/       broker client
internal/infrastructure/marketplace/mercadolibre/  MercadoLibre API client, split per topic (handler_auth.go,
                                         handler_categories.go, client.go — add handler_items.go / handler_orders.go
                                         etc. the same way; keeps transport separate from business logic)
internal/infrastructure/synapse/        (consumption of data arriving via the event flow, not direct Synapse access)
internal/infrastructure/servertls/      TLS keystore loading (PKCS12)
internal/config/                        env-based config loader (config.Load())
```

`main.go` wires everything by hand (no DI framework): build MySQL repos → application services → HTTP handlers → start RabbitMQ consumers in a goroutine → build chi router (`api.NewRouter(...)`) → serve HTTPS (falls back to plain HTTP if no TLS config) using `crypto/tls` and a PKCS12 keystore (`SERVER_TLS_KEYSTORE_PATH`).

Notable hotspots (high fan-in, touched by most modules): `writeJSONError`, `UserFromContext` (both in `internal/interfaces/http`), and the generic MySQL repo methods `Create`/`Update`/`FindByID`/`FindPaginated`/`SoftDelete`.

**Rules for this service:**
- Every I/O method takes `context.Context`.
- Handlers stay thin — no business logic in router/handlers/repositories.
- Wrap errors with `fmt.Errorf("...: %w", err)`.
- No package-level mutable global state.
- New modules follow the existing `internal/application/<module>` + handler pattern rather than introducing a parallel structure.

## Architecture: synapse-bridge (Java / Spring Boot)

Layout under `services/synapse-bridge/src/main/java/com/vegusa/ecommerce/`: `config/` (datasources for ERP + Nissan, RabbitMQ, Redis), `controller/`, `service/` (`EcomProductService`, `ProductService`, `ItemStockService`, `ItemLocationService`, `RedisProductService`, `EventPublisher`), `repository/`, `dto/` (includes event DTOs `PageProcessedEvent`, `SyncCompletedEvent`, `SyncFailedEvent`), `exception/`.

- Constructor injection only, never field injection.
- Controllers stay thin; orchestration lives in `service/`.
- Bean Validation (`@Valid`, constraints) at the input boundary.
- Read-only against ERP/Synapse; writes go through Redis/RabbitMQ into the official flow, never back to the ERP without explicit approval.

## Architecture: admin-dashboard (React + TypeScript + Vite)

Stack: React 19, TypeScript strict, TanStack Query (server state), React Hook Form + Zod (forms/validation), Zustand (local/shared state), Tailwind CSS + shadcn/ui, react-router-dom, axios.

- Feature-folder organization by domain.
- All HTTP calls centralized through the API client layer (`src/lib/api.ts`), including `registerUnauthorizedHandler`.
- Auth/session: `src/auth/AuthContext.tsx`, `src/auth/ProtectedRoute.tsx`, `src/auth/sessionStorage.ts` — route protection follows the existing `ProtectedRoute` / `ProtectedLayout` pattern.
- Functional components + hooks only; never duplicate core-orchestrator business rules in the UI.

## Commands

### core-orchestrator (Go, from `services/core-orchestrator/`)
```
go run ./cmd                 # run the server locally (loads ../../.env)
go build ./cmd                # build
go vet ./...                  # vet
go test ./...                  # run all tests
go test ./internal/application/products/... -run TestName -v   # single package / single test
```
There are currently no `*_test.go` files in this service.

### synapse-bridge (Java/Spring, from `services/synapse-bridge/`)
```
./gradlew bootRun                                   # run locally
./gradlew build                                     # build (compiles + runs tests)
./gradlew test                                      # run all tests
./gradlew test --tests EcommerceApplicationTests     # single test class
```

### admin-dashboard (React, from `services/admin-dashboard/`)
```
npm run dev        # Vite dev server
npm run build       # tsc -b && vite build
npm run lint        # eslint .
npm run preview     # preview production build
```

### Whole stack
```
docker-compose up     # rabbitmq, redis, synapse-bridge, core-orchestrator (HTTPS on 443), admin-dashboard (5173)
```
core-orchestrator serves HTTPS using the PKCS12 keystore at `keystore.p12` (`SERVER_TLS_KEYSTORE_*` env vars).

## Security note

`.env.example` currently contains what appear to be real credentials (ERP/Nissan DB passwords) rather than placeholders — treat as sensitive, do not commit further copies, and rotate/redact if this is unintentional.
