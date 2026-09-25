# =========================
# Build Stage
# =========================
FROM node:24-alpine AS builder

WORKDIR /app

COPY package*.json ./

RUN npm ci

COPY . .

RUN npm run build

# =========================
# Runtime Stage
# =========================
FROM nginx:alpine

# Destino del proxy /api (ver nginx/default.conf.template). El default sirve
# para docker-compose; en Azure Container Apps se sobrescribe con la app interna.
ENV API_UPSTREAM=https://core-orchestrator:443

COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template

EXPOSE 80

CMD ["nginx","-g","daemon off;"]
