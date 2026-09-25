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
# cloudflared (binario Go estático, funciona sobre alpine)
# =========================
FROM cloudflare/cloudflared:latest AS cloudflared

# =========================
# Runtime Stage
# =========================
FROM nginx:alpine

# Destino del proxy /api (ver nginx/default.conf.template). El default sirve
# para docker-compose; en Azure Container Apps se sobrescribe con la app interna.
ENV API_UPSTREAM=https://core-orchestrator:443

COPY --from=cloudflared /usr/local/bin/cloudflared /usr/local/bin/cloudflared
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx/default.conf.template /etc/nginx/templates/default.conf.template
COPY start.sh /start.sh

EXPOSE 80

# Arranca nginx y, si hay TUNNEL_TOKEN, también el túnel de Cloudflare
# (vecom-api.odo.mx -> localhost:80). Sin token (docker-compose) solo nginx.
CMD ["/start.sh"]
