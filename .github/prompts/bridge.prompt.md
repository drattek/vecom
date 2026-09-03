# Bridge Prompt

## Idioma

Responde siempre en español.

## Scope

Aplicable a cambios en `services/synapse-bridge`.

## Contexto obligatorio

Leer antes de proponer cambios:

1. `infrastructure/docs/events.md`
2. `infrastructure/docs/domain.md`
3. `infrastructure/decisions/*`
4. `SYSTEM_OVERVIEW.md`
5. `business-rules.md`
6. `coding-standards.md`

## Responsabilidades del servicio

- Integración con ERP/Synapse en modo lectura.
- Publicación de eventos para actualización de datos.
- Enriquecimiento mínimo necesario para sincronización.

## Prohibiciones

- No concentrar lógica de negocio del producto.
- No escribir de vuelta al ERP si no está explícitamente aprobado.
- No asumir Redis como fuente de verdad.

## Patrón de implementación esperado

- Controladores delgados.
- Servicios orientados a casos de integración.
- Publicación de eventos con contratos estables.
- Manejo robusto de errores y reintentos.

## Checklist de salida

1. ¿Se mantiene Synapse/ERP como lectura?
2. ¿El evento publicado es claro y versionable?
3. ¿Se evita mover lógica de negocio fuera del core?
4. ¿Se documenta el impacto en contratos de eventos?
5. ¿Se mantiene trazabilidad de sincronización?
