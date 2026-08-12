# Backend Prompt

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

## Checklist de salida

1. ¿La lógica quedó en dominio/aplicación?
2. ¿Se respetó soft delete donde aplica?
3. ¿Se evitó acoplamiento a ERP/Synapse?
4. ¿Se mantienen contratos API consistentes?
5. ¿Se consideró idempotencia si hay consumidores de eventos?
