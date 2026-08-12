# AI Content Workflow

## Objetivo

Definir un flujo único y repetible para que cualquier generación de contenido o código quede alineada con la arquitectura, reglas de negocio y estándares del repositorio.

## Fuente documental oficial

La documentación oficial del proyecto vive en `infrastructure/`.

En caso de conflicto, el contenido de `infrastructure/docs/` y `infrastructure/decisions/` tiene prioridad sobre documentos resumen ubicados en la raíz.

## Orden de precedencia (fuentes de verdad)

Cuando exista conflicto entre documentos, aplicar este orden:

1. `infrastructure/decisions/*`
2. `infrastructure/docs/*.md`
3. `infrastructure/mysql/*.md`
4. `README.md`
5. `SYSTEM_OVERVIEW.md`
6. `business-rules.md`
7. `conventions.md`
8. `coding-standards.md`
9. `product-flow.md`
10. `.github/prompts/*.prompt.md`

## Flujo obligatorio antes de generar contenido

1. Clasificar solicitud
- `frontend`: cambios en `services/admin-dashboard`
- `backend`: cambios en `services/core-orchestrator`
- `bridge`: cambios en `services/synapse-bridge`
- `cross-service`: impacto entre dos o más servicios

2. Cargar contexto base
- Leer siempre `infrastructure/docs/domain.md`, `infrastructure/docs/api.md` y `infrastructure/docs/events.md`.
- Leer `infrastructure/decisions/*` cuando exista decisión aplicable.
- Usar documentos de raíz solo como contexto complementario.

3. Cargar contexto específico
- Para `frontend`: usar `.github/prompts/frontend.prompt.md`.
- Para `backend`: usar `.github/prompts/backend.prompt.md`.
- Para `bridge`: usar `.github/prompts/bridge.prompt.md`.
- Para `cross-service`: combinar prompts involucrados y validar `infrastructure/docs/events.md`.

4. Verificación de guardrails arquitectónicos
- `core-orchestrator` no consulta Azure Synapse.
- Solo `synapse-bridge` puede conectarse a ERP/Synapse.
- Redis es caché, no fuente de verdad.
- Soft delete obligatorio (`deleted_at`) cuando aplique.
- La lógica de negocio vive en `core-orchestrator`.

5. Plan de salida
- Definir archivos afectados.
- Definir riesgos de arquitectura y mitigaciones.
- Definir pruebas mínimas.

6. Generación
- Producir cambios por capas (dominio, aplicación, infraestructura, interfaces) en backend.
- Mantener consistencia de tipado y patrones en frontend.
- No duplicar lógica entre servicios.

7. Autochequeo final
- ¿Respeta flujo oficial ERP -> Bridge -> Redis/RabbitMQ -> Core -> Marketplace?
- ¿Respeta responsabilidades por servicio?
- ¿Respeta estándares del lenguaje?
- ¿Evita dependencias directas prohibidas?

## Matriz de documentos por tipo de tarea

### Tareas de negocio
- `infrastructure/docs/domain.md`
- `business-rules.md`
- `product-flow.md`

### Tareas de contratos/API
- `infrastructure/docs/api.md`
- `coding-standards.md`

### Tareas de integración/eventos
- `infrastructure/docs/events.md`
- `infrastructure/decisions/*`

## Convención para prompts

Cada archivo en `.github/prompts` debe incluir:

- Scope del servicio
- Responsabilidades
- Prohibiciones explícitas
- Checklist de arquitectura
- Estándares de implementación
- Criterios de salida

## Mantenimiento

Actualizar este flujo cuando:

- cambie la arquitectura,
- cambien reglas de negocio,
- se agregue un nuevo servicio,
- se detecten inconsistencias repetitivas en generación.
