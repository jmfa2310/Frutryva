# syntax=docker/dockerfile:1
FROM node:22-alpine AS build
WORKDIR /app
COPY apps/manager/package.json apps/manager/package-lock.json ./
RUN --mount=type=secret,id=proxy_ca \
    if [ -f /run/secrets/proxy_ca ]; then export NODE_EXTRA_CA_CERTS=/run/secrets/proxy_ca; fi; \
    npm ci --strict-ssl=true
COPY apps/manager/ ./
RUN npm run build

FROM nginx:stable-alpine
COPY --chmod=0444 apps/manager/nginx.conf /etc/nginx/nginx.conf
COPY --from=build /app/dist/ /usr/share/nginx/html/
USER nginx
EXPOSE 8080
CMD ["nginx", "-g", "daemon off;"]
