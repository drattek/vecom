# ADR 0009 — RBAC del admin-dashboard: roles, permisos y excepciones por usuario

- Estado: IMPLEMENTADO (2026-11-24)
- Alcance: `core-orchestrator` (API) y `admin-dashboard`.
- DDL: `infrastructure/mysql/2026-11-18_rbac_roles_permissions.sql` (tablas) y
  `2026-11-18_rbac_permissions_seed.sql` (catálogo, idempotente).

## Contexto

Todas las rutas protegidas solo exigían un JWT válido (`RequireAuth`) y todas las
cuentas creadas desde "Usuarios" nacían como `admin`. Hacía falta restringir qué
secciones y acciones ve cada persona sin tocar el flujo de datos ERP → marketplaces.

## Decisión

### Modelo

- `ecom_permissions`: catálogo `recurso.accion` (p. ej. `brands.edit`).
- `ecom_roles` + `ecom_role_permissions`: un rol agrupa permisos. Baja lógica (`deleted_at`).
- `ecom_user_permissions`: excepciones por usuario, `grant` o `deny`.
- `ecom_api_user.role_id` (FK a `ecom_roles`) y la columna existente `role`.
- **Permisos efectivos = (permisos del rol ∪ grants del usuario) − denies del usuario.**

### Administrador

`ecom_api_user.role = 'admin'` tiene acceso total y **no pasa por estas tablas**
(cortocircuito en backend y frontend). Las cuentas no admin guardan `role = 'user'`
más un `role_id` opcional; las cuentas técnicas (p. ej. `supabase-bridge`,
`api_client`) no son admin y quedan sin permisos de dashboard.

### Backend

- `RequireAuth` carga los permisos efectivos **en cada request** (no viajan en el
  JWT): un cambio de rol/permisos aplica de inmediato, sin re-login. Costo: una
  consulta extra por request de usuarios no admin.
- `RequireRoutePermission(routePermissions)` (`internal/api/route_permissions.go`)
  resuelve `"MÉTODO /patrón-chi"` → lista de permisos (basta uno). **Default-deny:**
  una ruta protegida sin entrada es solo para admin. Así `/api/sources`,
  `/api/attributes*`, `/api/channel-attributes*`, `/api/channel-parameters*`,
  `/api/migration/*`, los `temp-*`/debug de MercadoLibre, `item-lookup` y el borrado
  de movimientos de stock quedan solo-admin sin catalogarlos.
- Las lecturas de catálogos (marcas, categorías, divisas, almacenes…) admiten
  también los permisos de las pantallas que las usan en formularios.
- `GET /api/me` devuelve el usuario con sus permisos (el frontend lo refresca al cargar).
- `GET/POST/PUT/DELETE /api/roles`, `GET /api/permissions`, `GET/PUT /api/users/{id}`.
- Protecciones: no se puede quitar el último admin activo ni auto-bloquearse
  (quitarse admin o desactivarse); un rol con usuarios asignados no se elimina.
- `TestRoutePermissionsMatchRouter` falla si una clave de la política no existe en
  el router (typos que dejarían la ruta como solo-admin sin aviso).

### Frontend

- `useAuth()` expone `isAdmin` y `can(...permisos)` (admin siempre `true`).
- Menú lateral, menú de Configuración y pestañas del detalle de producto se filtran
  por permiso; las rutas se protegen con `RequirePermission`; `/dashboard` redirige
  a la primera sección permitida si falta `dashboard.view`.
- Configuración → Accesos → **Roles** (CRUD con matriz de permisos).
- Usuarios: alta con tipo de cuenta (admin / con rol), rol y permisos extra o
  denegados; edición de acceso y estado.

## Consecuencias / pendientes

- Ocultar botones de crear/editar/eliminar en cada pantalla de Configuración según
  permiso es incremental: el backend ya responde 403 aunque el botón se vea.
- Nuevos endpoints requieren una entrada en `routePermissions` o serán solo-admin.
- Nuevos permisos: agregarlos al seed (idempotente) y a `RESOURCE_LABELS` en
  `admin-dashboard/src/lib/rbac.ts` para tener etiqueta en español.
