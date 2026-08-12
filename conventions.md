# Convenciones Unificadas del Proyecto

## Objetivo

Definir un marco único para generar módulos y código en todo el ecosistema del proyecto, de modo que cualquier implementación nueva sea consistente con:

- reglas de negocio,
- flujo de producto,
- estándares técnicos por lenguaje,
- estructura actual de servicios.

Estas convenciones aplican a `core-orchestrator` (Go), `synapse-bridge` (Java/Spring Boot) y `admin-dashboard` (React/TypeScript).

## Principios transversales

1. **Una sola fuente de verdad por tipo de dato**
- Redis es caché, nunca fuente de verdad.
- Azure Synapse es solo lectura.
- El estado de negocio persistente vive en MariaDB/Core.

2. **Consistencia de dominio antes que conveniencia técnica**
- El ERP no almacena atributos personalizados.
- Los atributos personalizados pertenecen exclusivamente al Core.
- Todo módulo nuevo debe respetar esta frontera.

3. **Estandarización del flujo de información**
- Flujo oficial: `ERP -> Synapse Bridge -> Redis/RabbitMQ -> Core -> Marketplaces`.
- No crear atajos que salten etapas del flujo sin justificación arquitectónica explícita.

4. **Trazabilidad y auditabilidad**
- Toda operación de escritura debe poder rastrearse.
- En base de datos, respetar columnas de auditoría y soft delete (`deleted_at`).

## Convenciones funcionales (negocio)

- Todas las bases de datos usan **Soft Delete**.
- Los precios se manejan en **USD y MXN**, usando relación con `currency` según aplique.
- Todos los marketplaces reciben precios en **MXN**.
- Las transformaciones de datos de ERP hacia el modelo Core deben preservar semántica de negocio y evitar sobre-escrituras destructivas sin control.

## Convenciones de arquitectura por servicio

### 1) Core Orchestrator (Go)

#### Organización obligatoria
- `cmd/`: entrypoints y bootstrap.
- `internal/api/`: router y configuración HTTP.
- `internal/interfaces/http/`: handlers y contrato web.
- `internal/application/<modulo>/`: casos de uso.
- `internal/domain/`: entidades y reglas del negocio.
- `internal/infrastructure/`: adaptadores (DB, Redis, RabbitMQ, marketplaces, servicios externos).

#### Reglas de implementación
- Siempre usar `context.Context` en operaciones con I/O.
- Handlers delgados: validan/parsean y delegan a application layer.
- No colocar lógica de negocio en router ni en infraestructura.
- Envolver errores con contexto (`%w`).
- Evitar estado global mutable.
- Toda función pública relevante debe estar documentada.

### 2) Synapse Bridge (Java / Spring Boot)

#### Organización obligatoria
- `controller`: entrada HTTP o jobs expuestos.
- `service`: casos de uso y orquestación.
- `repository` o adaptadores de acceso.
- `dto`/`model`/`mapper` para separación de contratos.

#### Reglas de implementación
- Constructor Injection siempre.
- Nunca Field Injection.
- Validación de entrada con Bean Validation (`@Valid`, constraints).
- Controladores sin lógica de negocio compleja.
- Manejo consistente de errores y respuestas.
- Integración con Synapse en modo lectura, sin asumir escritura en origen.

### 3) Admin Dashboard (React + TypeScript)

#### Organización obligatoria
- Feature Folder por dominio funcional.
- Separar páginas, componentes, hooks, stores y servicios API.

#### Reglas de implementación
- Solo componentes funcionales con Hooks.
- Estado servidor con TanStack Query.
- Formularios con React Hook Form + Zod.
- UI con Tailwind + Shadcn, manteniendo coherencia visual existente.
- Tipado estricto en requests/responses.
- Centralizar llamadas HTTP en capa de API (`lib/api.ts` o equivalente).
- Proteger rutas y sesión según el patrón actual (`auth/`, `ProtectedRoute`).

## Convención para crear un módulo nuevo

Cada módulo nuevo debe seguir este procedimiento base:

1. **Definición de dominio**
- Identificar entidad raíz, reglas de negocio y relaciones.
- Confirmar impacto en monedas, inventario, pricing o canales.

2. **Diseño de contrato**
- Definir entradas/salidas del caso de uso.
- Definir eventos o mensajes si participan RabbitMQ/Redis.

3. **Implementación por capas**
- Dominio: tipos y reglas.
- Aplicación/servicio: casos de uso.
- Infraestructura/repositorio: persistencia o cliente externo.
- Interfaces (HTTP/UI): transporte.

4. **Persistencia y datos**
- Aplicar soft delete cuando corresponda.
- Respetar FK, auditoría y consistencia con tablas existentes.

5. **Integración con flujo oficial**
- Verificar que la data fluye por `Synapse Bridge -> Core -> Marketplace` cuando aplique.
- Evitar acoplamientos directos entre UI y orígenes externos sin pasar por Core.

6. **Calidad mínima**
- Manejo de errores consistente.
- Validaciones de entrada completas.
- Pruebas en la capa adecuada (unitarias/integración según riesgo).

## Convenciones de nomenclatura

- Nombres de dominio claros y consistentes en inglés técnico para código.
- Evitar abreviaturas ambiguas.
- Mantener simetría entre nombre de módulo backend y recursos API/UI relacionados.
- Endpoints REST en plural para colecciones (ej. `/api/products`, `/api/price-lists`).

## Convenciones de base de datos

- Preferir extensión de tablas existentes del dominio antes de crear estructuras paralelas.
- No crear datos huérfanos: validar entidades padre antes de insertar dependientes.
- En lecturas de negocio, considerar `deleted_at IS NULL` cuando aplique.
- Para pricing:
	- estado actual en tablas operativas de precio,
	- trazabilidad en historial.

## Convenciones de integración y mensajería

- Redis para caché y optimización de lectura, no para decisiones definitivas de negocio.
- RabbitMQ para desacoplar procesos asíncronos de sincronización.
- Los conectores a marketplaces deben depender de modelos normalizados del Core, no del formato crudo del ERP.

## Checklist obligatorio antes de dar por terminado un cambio

- ¿El cambio respeta el flujo oficial del producto?
- ¿La lógica está en la capa correcta?
- ¿Se respetan las reglas de negocio de moneda y atributos personalizados?
- ¿Se evita usar Redis como fuente de verdad?
- ¿Se mantiene soft delete y auditoría en persistencia?
- ¿El módulo mantiene estilo y estructura del servicio donde vive?

Si cualquier respuesta es "no", el cambio debe ajustarse antes de considerarse completo.

