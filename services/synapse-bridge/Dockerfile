# =========================
# Build Stage
# =========================
FROM gradle:8.7-jdk21 AS builder

WORKDIR /app

COPY build.gradle .
COPY settings.gradle .
COPY gradlew .
COPY gradle gradle

RUN chmod +x gradlew

COPY src src

RUN ./gradlew bootJar --no-daemon

# =========================
# Runtime Stage
# =========================
FROM eclipse-temurin:21-jre

RUN groupadd --system app && useradd --system --gid app app

WORKDIR /app

COPY --from=builder /app/build/libs/*.jar app.jar

USER app

EXPOSE 8080

ENTRYPOINT ["java","-jar","app.jar"]