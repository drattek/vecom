# Middleware Ecommerce (Vegusa)

Middleware que sincroniza catálogo, inventario y precios desde Azure Synapse/ERP hacia un
sistema propio, y distribuye esa información hacia marketplaces (MercadoLibre, Amazon) y
Odoo.

## Qué resuelve

La empresa mantiene su información de productos en distintos ERP (uno general/de
maquinaria y otro de vehículos Nissan, ambos sobre Azure Synapse). Este proyecto:

1. Lee esos ERP de forma **read-only**, sin tocarlos nunca.
2. Centraliza la información en una base de datos propia (`ecom_*`), agregando lo que el
   ERP no tiene (atributos personalizados, medios, curación manual de marca/categoría).
3. Publica ese catálogo hacia marketplaces (MercadoLibre, Amazon) y Odoo, siempre en MXN.
4. Expone un panel de administración para operar y curar el catálogo sin depender del ERP.

## Arquitectura: tres servicios independientes

```
                 (solo lectura)
   ERP / Azure Synapse
          │
          ▼
   synapse-bridge  ──publica eventos──▶  RabbitMQ
   (Java / Spring)  ──escribe cache───▶  Redis
                                            │
                                            ▼
                                   core-orchestrator (Go)
                                   API REST + lógica de negocio
                                   MySQL (ecom_*)
                                            │
                                            ▼
                              MercadoLibre · Amazon · Odoo
                                            ▲
                                            │
                                    admin-dashboard (React)
```

| Servicio | Stack | Responsabilidad | Nunca hace |
|---|---|---|---|
| [`services/synapse-bridge`](services/synapse-bridge) | Java 17 / Spring Boot | Única aplicación autorizada a conectarse a Azure Synapse/ERP. Lee productos, stock y existencias por página, los escribe en Redis (cache) y publica eventos de progreso/finalización en RabbitMQ. | No contiene lógica de negocio de producto. No escribe de vuelta al ERP sin decisión explícita. |
| [`services/core-orchestrator`](services/core-orchestrator) | Go | Backend: dueño de toda la lógica de negocio, la API REST, integraciones de marketplace y persistencia en MySQL. Consume los eventos de `synapse-bridge` vía Redis/RabbitMQ para materializar el catálogo. | No consulta Azure Synapse ni el ERP directamente. |
| [`services/admin-dashboard`](services/admin-dashboard) | React 19 / TypeScript / Vite | Frontend de administración: gestión de catálogo, canales, credenciales, medios, compatibilidades. | No accede al ERP directamente; todo pasa por la API de `core-orchestrator`. |

El flujo de datos oficial es siempre `ERP → Synapse Bridge → Redis/RabbitMQ → Core
Orchestrator → Marketplaces`. No se introducen atajos que se lo salten sin una decisión de
arquitectura explícita (ver `infrastructure/decisions/`).

## Cómo viajan los eventos

`synapse-bridge` pagina cada fuente (ítems del ERP, stock del ERP, existencias de Nissan),
guarda cada página en Redis y publica un evento de progreso por página más un evento de
finalización cuando termina toda la sincronización. `core-orchestrator` solo materializa el
catálogo en MySQL al recibir el evento de finalización (lee todo lo acumulado en Redis en
ese momento), no en cada evento de página.

| Origen | Prefijo en Redis | Routing keys (`sync.exchange`) |
|---|---|---|
| Productos del ERP (`EcomProductDTO`) | `product:<code>` | `item.page.processed`, `item.sync.completed`, `item.sync.failed` |
| Stock del ERP (`ItemInventLocationDTO`) | `stock:<articulo>-<almacen>` | `stock.page.processed`, `stock.sync.completed`, `stock.sync.failed` |
| Existencias Nissan (`NissanExistenciasDTO`) | `nissan:existencia:<code>-<agencia>` | `nissan.existencias.page.processed`, `nissan.existencias.sync.completed`, `nissan.existencias.sync.failed` |

Todo routing key que se agregue, renombre o elimine debe actualizarse en el mismo cambio
en ambos lados (`RabbitConfig.java`/`EventPublisher.java` en `synapse-bridge` y
`cmd/consumers_init.go`/`internal/interfaces/consumers/` en `core-orchestrator`). El detalle
completo vive en [`infrastructure/docs/events.md`](infrastructure/docs/events.md).

## Reglas de negocio transversales

- **Soft delete siempre.** Ninguna tabla de dominio borra físicamente; se filtra por
  `deleted_at IS NULL`.
- **Precios en USD y MXN**, vía relación con `currency`; todo marketplace/canal recibe
  precios en **MXN**.
- **Atributos personalizados** existen solo en `core-orchestrator`; el ERP nunca los
  almacena.
- **Redis es cache**, nunca fuente de verdad. Azure Synapse es solo lectura.
- **Multi-ERP**: `synapse-bridge` se conecta al ERP general/de maquinaria
  (`ErpDataSourceConfig`) y al ERP de vehículos Nissan (`NisanDataSourceConfig`) por
  separado.
- Las escrituras de negocio llevan columnas de auditoría (`created_by`, `updated_by`,
  `created_at`, `updated_at`).

## Documentación del proyecto

Cuando la documentación entra en conflicto, este es el orden de verdad (más alto primero);
ver el flujo completo en [`AI_CONTENT_WORKFLOW.md`](AI_CONTENT_WORKFLOW.md):

1. `infrastructure/decisions/*` (ADRs, si existen)
2. [`infrastructure/docs/domain.md`](infrastructure/docs/domain.md),
   [`api.md`](infrastructure/docs/api.md), [`events.md`](infrastructure/docs/events.md)
3. [`infrastructure/mysql/database.md`](infrastructure/mysql/database.md),
   [`relationships.md`](infrastructure/mysql/relationships.md)
4. Documentos de raíz complementarios: [`SYSTEM_OVERVIEW.md`](SYSTEM_OVERVIEW.md),
   [`business-rules.md`](business-rules.md), [`conventions.md`](conventions.md),
   [`coding-standards.md`](coding-standards.md), [`product-flow.md`](product-flow.md)
5. `.github/prompts/{backend,frontend,bridge}.prompt.md` — checklist canónico por
   servicio; leer el que corresponda antes de proponer cambios ahí.

`CLAUDE.md` resume estas reglas para asistentes de IA que trabajen en el repo.

## Cómo correr el proyecto

### Stack completo (Docker Compose)

```
docker-compose up
```

Levanta RabbitMQ, Redis, `synapse-bridge`, `core-orchestrator` y `admin-dashboard`. Requiere
un `.env` en la raíz (ver `.env.example`) y el keystore TLS de `core-orchestrator`
(`SERVER_TLS_KEYSTORE_PATH`, `keystore.p12`).

| Servicio | Puerto | Notas |
|---|---|---|
| RabbitMQ | 5672 (AMQP), 15672 (management UI) | |
| Redis | 6379 | |
| synapse-bridge | 8080 | HTTP plano |
| core-orchestrator | 443 | HTTPS (cae a HTTP si no hay config TLS) |
| admin-dashboard | 5173 | |

### Por servicio (desarrollo local)

**`core-orchestrator`** (desde `services/core-orchestrator/`):
```
go run ./cmd                 # corre el servidor (carga ../../.env)
go build ./cmd                # build
go vet ./...                  # vet
go test ./...                  # tests
```

**`synapse-bridge`** (desde `services/synapse-bridge/`):
```
./gradlew bootRun             # corre local
./gradlew build                # build + tests
./gradlew test                 # solo tests
```

**`admin-dashboard`** (desde `services/admin-dashboard/`):
```
npm run dev        # servidor de desarrollo (Vite)
npm run build        # tsc -b && vite build
npm run lint          # eslint .
```

## Seguridad

`.env.example` contiene actualmente lo que parecen ser credenciales reales (contraseñas de
BD del ERP/Nissan) en lugar de placeholders — tratar como sensible, no commitear copias
adicionales y rotar/redactar si esto no fue intencional.
