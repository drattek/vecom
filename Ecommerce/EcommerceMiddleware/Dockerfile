# =====================================================================
# EcommerceMiddleware - Spring Boot 3.2.3 / Java 21
# Build multi-etapa. Preparado para Azure Container Apps (HTTP en 8080,
# TLS terminado en el ingress).
# =====================================================================

# ---------------------------------------------------------------------
# Etapa 1: build con el Gradle wrapper del repo (misma version que en local)
# ---------------------------------------------------------------------
FROM eclipse-temurin:21-jdk-jammy AS builder
WORKDIR /app

# Primero solo lo necesario para resolver dependencias -> capa cacheable.
COPY gradlew ./
COPY gradle gradle
COPY build.gradle settings.gradle ./
RUN chmod +x gradlew && ./gradlew dependencies --no-daemon || true

# Ahora el codigo fuente. Cambiar codigo no invalida la capa de dependencias.
COPY src src
RUN ./gradlew bootJar --no-daemon -x test

# Descomprimir el jar en capas de Spring Boot para mejor cacheo de imagen.
# NOTA: requiere BuildKit (docker buildx / GitHub Actions). El builder clasico
# de ACR Tasks ("az acr build") falla al exportar estas capas, por eso la
# imagen se construye en el runner de GitHub y se hace push al ACR.
RUN mkdir -p build/extracted && \
    java -Djarmode=layertools -jar build/libs/*.jar extract --destination build/extracted

# ---------------------------------------------------------------------
# Etapa 2: runtime minimo (JRE, sin compilador) y usuario sin privilegios
# ---------------------------------------------------------------------
FROM eclipse-temurin:21-jre-jammy
WORKDIR /app

# curl para el HEALTHCHECK
RUN apt-get update && apt-get install -y --no-install-recommends curl && \
    rm -rf /var/lib/apt/lists/*

# Usuario no-root
RUN groupadd --system spring && useradd --system --gid spring spring

# Capas de Spring Boot, de la que menos cambia a la que mas
COPY --from=builder --chown=spring:spring /app/build/extracted/dependencies/ ./
COPY --from=builder --chown=spring:spring /app/build/extracted/spring-boot-loader/ ./
COPY --from=builder --chown=spring:spring /app/build/extracted/snapshot-dependencies/ ./
COPY --from=builder --chown=spring:spring /app/build/extracted/application/ ./

USER spring

# Valores por defecto; se sobrescriben con env vars / secrets de Azure.
ENV SERVER_PORT=8080 \
    SERVER_SSL_ENABLED=false \
    JAVA_OPTS="-XX:MaxRAMPercentage=75 -Djava.security.egd=file:/dev/./urandom"

EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=5s --start-period=60s --retries=3 \
    CMD curl -fsS http://localhost:${SERVER_PORT}/actuator/health || exit 1

ENTRYPOINT ["sh", "-c", "exec java $JAVA_OPTS org.springframework.boot.loader.launch.JarLauncher"]
