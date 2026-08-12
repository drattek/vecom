# Domain Guide

## Propósito

Definir reglas funcionales y fronteras de dominio para mantener consistencia entre servicios.

## Fronteras de dominio

- `core-orchestrator` es dueño de la lógica de negocio.
- `synapse-bridge` solo integra y publica cambios del ERP.
- `admin-dashboard` solo consume APIs del core.

## Reglas críticas

- No eliminar productos físicamente: usar soft delete.
- Atributos personalizados existen solo en core.
- Precios para marketplaces siempre en MXN.
- Redis no es fuente de verdad.

## Entidades núcleo

- Product
- Category
- Brand
- Inventory
- Pricing
- ProductMedia
- Channel
- ChannelConnection
- Credential

## Criterios para agregar reglas nuevas

1. Deben vivir en dominio/aplicación de `core-orchestrator`.
2. Deben indicar impacto en pricing, inventario y canales.
3. Deben definir si requieren evento.
4. Deben mantener trazabilidad y auditabilidad.
