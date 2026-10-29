# syntax=docker/dockerfile:1
FROM golang:1.26-alpine AS build
WORKDIR /src
COPY backend/go.mod backend/go.sum ./
# Optional public CA bundle supplied by managed cloud builds. TLS stays enabled.
RUN --mount=type=secret,id=proxy_ca \
    if [ -f /run/secrets/proxy_ca ]; then export SSL_CERT_FILE=/run/secrets/proxy_ca; fi; \
    go mod download
COPY backend/ .
RUN CGO_ENABLED=0 go build -mod=readonly -trimpath -o /futryva .
FROM alpine:3.23
WORKDIR /app
COPY --from=build /futryva /futryva
COPY --chown=65532:65532 database/ /app/database/
ENV MIGRATIONS_DIR=/app/database
USER 65532:65532
EXPOSE 8080
CMD ["/futryva"]
