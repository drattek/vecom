# Etapa 1: Build usando Gradle con Java 17
FROM gradle:8.5-jdk17 AS builder
WORKDIR /app

# Copiamos el wrapper y el archivo de build
COPY build.gradle settings.gradle ./
COPY gradle gradle

# Descargar dependencias para caché
RUN gradle build --no-daemon --stacktrace -x test || return 0

# Copiar el código fuente
COPY . .

# Build del jar (ignora tests)
RUN gradle bootJar --no-daemon

# Etapa 2: Imagen liviana para ejecución
FROM eclipse-temurin:17-jdk-jammy
WORKDIR /app

# Variables de entorno (opcional)
ENV SPRING_PROFILES_ACTIVE=prod
ENV JAVA_OPTS=""

# Copiamos el .jar generado desde el builder
COPY --from=builder /app/build/libs/*.jar app.jar

# Expone el puerto por defecto de Spring Boot
EXPOSE 8080

# Comando de arranque
ENTRYPOINT ["sh", "-c", "java $JAVA_OPTS -jar app.jar"]
