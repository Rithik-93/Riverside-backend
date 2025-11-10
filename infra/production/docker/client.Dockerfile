FROM node:20-alpine AS builder

WORKDIR /app

COPY client/package*.json ./

RUN npm install --prefer-offline --no-audit

COPY client/. .

ARG VITE_API_URL=http://localhost:8081
ARG VITE_WS_URL=ws://localhost:8080
ARG VITE_UPLOAD_URL=http://localhost:8082

ENV VITE_API_URL=$VITE_API_URL
ENV VITE_WS_URL=$VITE_WS_URL
ENV VITE_UPLOAD_URL=$VITE_UPLOAD_URL

RUN npm run build

FROM nginx:1.25-alpine

RUN apk add --no-cache curl

COPY --from=builder /app/dist /usr/share/nginx/html
COPY client/nginx.conf /etc/nginx/conf.d/default.conf
COPY client/docker-entrypoint.sh /custom-entrypoint.sh

RUN chmod +x /custom-entrypoint.sh

EXPOSE 80

HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
    CMD curl -f http://localhost/ || exit 1

ENTRYPOINT ["/custom-entrypoint.sh"]


