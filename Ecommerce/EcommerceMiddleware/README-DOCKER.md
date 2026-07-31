# EcommerceMiddleware — Docker y despliegue en Azure

Middleware Spring Boot 3.2.3 / Java 21 que sincroniza el ERP (Dynamics 365 vía Azure
Synapse) con los marketplaces **Camso, Jumpseller, MercadoLibre y Multivende**.

---

## 1. Arranque local

```bash
cp .env.example .env     # rellena las credenciales reales
docker compose up --build
```

La app queda en <http://localhost:8080>, health en <http://localhost:8080/actuator/health>.

Sin compose:

```bash
docker build -t ecommerce-middleware:local .
docker run --rm -p 8080:8080 --env-file .env ecommerce-middleware:local
```

> No necesitas Java instalado: el build ocurre dentro del contenedor (tu JDK local es 8,
> el proyecto requiere 21).

---

## 2. Configuración

Toda la configuración sensible se lee de **variables de entorno**;
`application.properties` solo contiene referencias `${VAR:default}`.
La lista completa está en `.env.example`.

Las más importantes:

| Variable | Para qué sirve |
|---|---|
| `MW_DB_URL` / `MW_DB_USERNAME` / `MW_DB_PASSWORD` | Azure MySQL (base del middleware) |
| `MSB_DB_URL` / `MSB_DB_USERNAME` / `MSB_DB_PASSWORD` | Azure Synapse (ERP). Con `ActiveDirectoryServicePrincipal`, el username es el *client id* y el password el *client secret* |
| `SERVER_PORT` | Puerto HTTP. **8080** en contenedor |
| `SERVER_SSL_ENABLED` | **false** en Azure: el TLS lo termina el ingress |
| `CORS_ALLOWED_ORIGINS` | Orígenes permitidos, separados por comas |
| `WEBSCRAPER_API_BASE_URL` | URL del servicio `veg-web-scraper` (ver limitaciones) |
| `JAVA_OPTS` | Flags de JVM. Por defecto `-XX:MaxRAMPercentage=75` |

---

## 3. Despliegue en Azure Container Apps

El despliegue es **automático desde GitHub**. Solo hay que hacer el bootstrap una vez.

### Bootstrap (una sola vez)

```bash
az login --tenant e60ad600-4568-4f55-bdad-195ca7bc2861   # la suscripción exige MFA
gh auth login
cd Ecommerce/EcommerceMiddleware
./setup-cicd.sh
```

Crea, de forma idempotente:

1. **ACR** `acrvecom` y el **Container Apps environment** `vecom-env` en el RG `VECOM`
   (`southcentralus`, misma región que el MySQL `vecomdb`)
2. Una **app registration** `gh-actions-vecom` con **federated credentials (OIDC)** para las
   ramas `dev` y `prod` — GitHub obtiene un token temporal en cada run, **no se almacena
   ninguna contraseña**
3. Las Container Apps **`ecommerce-middleware-dev`** y **`ecommerce-middleware-prod`**, cada
   variable del `.env` cargada como *secret* de Azure y referenciada vía `secretref:`
4. Los secrets `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_SUBSCRIPTION_ID` en el repo

Las apps hacen pull del ACR con su **managed identity** (el admin del registry queda
deshabilitado). Los secretos de BD viven **solo en Azure**, nunca en GitHub.

### Uso diario

| Acción | Resultado |
|---|---|
| Push a `dev` | Build en ACR → despliega a `ecommerce-middleware-dev` |
| Push a `prod` | Build en ACR → despliega a `ecommerce-middleware-prod` |
| Abrir un PR | Solo compila la imagen para validar; no toca Azure |
| Actions → *Run workflow* | Despliegue manual de la rama que elijas |

Cada despliegue etiqueta la imagen con el SHA corto del commit y espera a que
`/actuator/health` responda `UP`; si no lo hace en 5 minutos, **el job falla y vuelca los
logs** del contenedor.

Rollback a una versión anterior:

```bash
az containerapp update -n ecommerce-middleware-prod -g VECOM \
  --image acrvecom.azurecr.io/ecommerce-middleware:<sha-anterior>
```

Ver logs en vivo:

```bash
az containerapp logs show -n ecommerce-middleware-prod -g VECOM --follow
```

### Cambiar una credencial

No se toca ni el código ni GitHub — se actualiza el secret en Azure y se reinicia:

```bash
az containerapp secret set -n ecommerce-middleware-prod -g VECOM --secrets mw-db-password=<nuevo>
az containerapp revision restart -n ecommerce-middleware-prod -g VECOM \
  --revision "$(az containerapp revision list -n ecommerce-middleware-prod -g VECOM --query '[0].name' -o tsv)"
```

---

## 4. Decisiones de diseño

**TLS.** La app venía configurada en el puerto 443 con un `keystore.p12` autofirmado.
Container Apps termina el TLS en el ingress y envía HTTP plano al contenedor, así que
ahora el contenedor escucha **HTTP en 8080** y el certificado lo gestiona Azure. El
soporte SSL sigue disponible (`SERVER_SSL_ENABLED=true`) para correr con HTTPS local.
`server.forward-headers-strategy=FRAMEWORK` hace que la app respete los headers
`X-Forwarded-*` del ingress.

**Health probe.** Se añadió `spring-boot-starter-actuator`. El indicador de base de datos
está **desactivado** (`MANAGEMENT_HEALTH_DB_ENABLED=false`): si estuviera activo, una caída
temporal de Synapse o MySQL marcaría el contenedor como no sano y Container Apps lo
reciclaría en bucle.

**Imagen.** Build multi-etapa: compila con el JDK 21 y el wrapper de Gradle del repo, y
ejecuta sobre **JRE** (sin compilador) como **usuario no-root**. El jar se descomprime en
capas de Spring Boot, de modo que un cambio de código solo reconstruye la capa de
aplicación y no las ~60 MB de dependencias.

---

## 5. Pendientes conocidos

**Rotar las credenciales.** Las contraseñas de MySQL y el client secret del service
principal estuvieron commiteadas en `application.properties` y **siguen en el historial de
git**. Sacarlas del archivo no las borra del historial: hay que rotarlas en Azure.

**Servicio `veg-web-scraper`.** `WebScraperService` llamaba a
`http://localhost:8080/veg-web-scraper`. Ahora es configurable vía
`WEBSCRAPER_API_BASE_URL`, pero ese servicio vive en otro repo y **no está desplegado**:
esos tres endpoints fallarán hasta que lo publiques y apuntes la variable a su URL.

**Tareas programadas.** Todos los `@Scheduled` del proyecto están comentados; la
sincronización solo se dispara por endpoints REST. `min-replicas=1` en el script evita que
la app escale a cero, para cuando los reactives.

**Réplicas.** Si subes `max-replicas` por encima de 1 y reactivas los `@Scheduled`, cada
réplica ejecutará los jobs por su cuenta. Haría falta ShedLock o similar.
