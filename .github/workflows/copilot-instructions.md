# Arquitectura

Este proyecto sigue una arquitectura orientada a eventos.

No generar código que consulte directamente Azure Synapse desde servicios distintos a synapse-bridge.

## Servicios

admin-dashboard
Frontend React.

core-orchestrator
Backend principal.

synapse-bridge
Integración con ERP.

## Reglas

Nunca acceder directamente al ERP desde core-orchestrator.

Los cambios del ERP llegan mediante RabbitMQ.

Redis únicamente actúa como caché.

Los atributos personalizados existen únicamente en core-orchestrator.

Los productos nunca se eliminan físicamente.

Todos los marketplaces utilizan precios en MXN.

Toda lógica de negocio pertenece al core-orchestrator.

Mantener separación estricta entre infraestructura, dominio y adaptadores.

Evitar duplicar lógica entre servicios.

La documentación oficial del proyecto vive en `infrastructure/`.

Consultar primero `infrastructure/docs` y `infrastructure/decisions` antes de proponer nuevas estructuras.

## Flujo de trabajo documental obligatorio

Antes de generar contenido o código, seguir el flujo definido en `AI_CONTENT_WORKFLOW.md`.

Usar prompts canónicos por servicio en:

- `.github/prompts/backend.prompt.md`
- `.github/prompts/frontend.prompt.md`
- `.github/prompts/bridge.prompt.md`