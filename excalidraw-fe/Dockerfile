FROM node:20-alpine AS builder

# Build-time environment variables
# These are injected via `docker build --build-arg VITE_API_URL=...`
ARG VITE_API_URL
ARG VITE_WS_URL

# Expose to Vite (must be prefixed VITE_)
ENV VITE_API_URL=${VITE_API_URL}
ENV VITE_WS_URL=${VITE_WS_URL}

WORKDIR /app

COPY package*.json ./

RUN npm ci

COPY . .

RUN npm run build

FROM nginx:alpine

COPY --from=builder /app/dist /usr/share/nginx/html

COPY docker-entrypoint.sh /docker-entrypoint.sh
RUN chmod +x /docker-entrypoint.sh

COPY nginx.conf.template /etc/nginx/conf.d/default.conf

EXPOSE 80

CMD ["/docker-entrypoint.sh"]
