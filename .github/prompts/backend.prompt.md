# Backend Prompt

## Idioma

Responde siempre en español.

## Scope

Aplicable a cambios en `services/core-orchestrator`.

## Contexto obligatorio

Leer antes de proponer cambios:

1. `SYSTEM_OVERVIEW.md`
2. `infrastructure/docs/domain.md`
3. `infrastructure/docs/api.md`
4. `infrastructure/docs/events.md`
5. `infrastructure/decisions/*`
6. `business-rules.md`
7. `conventions.md`
8. `coding-standards.md`

## Responsabilidades del servicio

- API REST para panel administrativo.
- Lógica de negocio del middleware.
- Integración con marketplaces.
- Consumo de eventos desde RabbitMQ.
- Persistencia en base de datos.

## Prohibiciones

- No consultar Azure Synapse directamente.
- No consultar ERP directamente.
- No usar Redis como fuente de verdad.
- No mover lógica de negocio a handlers/router/repositorios.

## Patrón de implementación esperado

- Dominio en `internal/domain`.
- Casos de uso en `internal/application`.
- Adaptadores en `internal/infrastructure`.
- Entrada/salida en `internal/interfaces`.

## Persistencia (ver ADR `infrastructure/decisions/0001-persistencia-transacciones-y-context.md`)

- Repos: constructor toma `mysql.Querier`, cada método toma `ctx context.Context` y usa `...Context`.
- Servicios: constructor toma `*sql.DB` + repos. Métodos toman `ctx` (desde `r.Context()` en el handler).
- Multi-sentencia → `mysql.WithinTx`. Una sola sentencia → repo sobre el pool, sin transacción.
- `mysql.Querier` = `ExecContext`/`QueryContext`/`QueryRowContext`. Nunca usar `context.TODO()` ni `context.Background()` en repos/servicios.
- Goroutines de fondo bajo `internal/shared/safe`.

## Checklist de salida

1. ¿La lógica quedó en dominio/aplicación?
2. ¿Se respetó soft delete donde aplica?
3. ¿Se evitó acoplamiento a ERP/Synapse?
4. ¿Se mantienen contratos API consistentes?
5. ¿Se consideró idempotencia si hay consumidores de eventos?
6. ¿`ctx` propagado handler → servicio → repo → driver?
7. ¿Operación multi-sentencia envuelta en `mysql.WithinTx`? ¿Una sola sentencia SIN transacción?
8. ¿Alguna `go` nueva sin `safe.Supervise`/`safe.Do`?
