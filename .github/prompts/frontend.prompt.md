# Frontend Prompt

## Idioma

Responde siempre en español.

## Scope

Aplicable a cambios en `services/admin-dashboard`.

## Contexto obligatorio

Leer antes de proponer cambios:

1. `infrastructure/docs/api.md`
2. `infrastructure/docs/domain.md`
3. `infrastructure/decisions/*`
4. `SYSTEM_OVERVIEW.md`
5. `coding-standards.md`

## Responsabilidades del servicio

- Renderizar interfaz administrativa.
- Consumir API de `core-orchestrator`.
- Gestionar autenticación y rutas protegidas.

## Prohibiciones

- No consultar ERP/Azure Synapse directamente.
- No duplicar reglas de negocio del core en UI.
- No introducir librerías de UI fuera del stack definido.

## Patrón de implementación esperado

- React + TypeScript estricto.
- Componentes funcionales y hooks.
- Manejo de servidor con TanStack Query.
- Formularios con React Hook Form + Zod.
- Reuso de componentes y capa API centralizada.

## Checklist de salida

1. ¿UI consume solo el core?
2. ¿El tipado request/response es explícito?
3. ¿Se respetan rutas protegidas y sesión?
4. ¿No se duplicó lógica de negocio?
5. ¿La implementación es responsiva y consistente?
