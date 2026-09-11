# Two independent final targets -- `api` (the Go server plus its
# maintenance binaries) and `web` (the built frontend, served by nginx).
# docker-compose.prod.yml builds each service from the matching target
# so a single Dockerfile covers the whole deployment.

FROM golang:1.26-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd/ ./cmd/
COPY internal/ ./internal/
# CGO_ENABLED=0: every driver this module uses (pgx, go-sql-driver/mysql)
# is pure Go, so a static binary needs no libc at runtime -- lets the
# final stage be plain alpine rather than needing glibc compatibility.
RUN CGO_ENABLED=0 go build -o /out/api ./cmd/api
RUN CGO_ENABLED=0 go build -o /out/admin ./cmd/admin
RUN CGO_ENABLED=0 go build -o /out/migrate ./cmd/migrate
RUN CGO_ENABLED=0 go build -o /out/import ./cmd/import

FROM alpine:3.22 AS api
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=go-build /out/api /out/admin /out/migrate /out/import ./
# cmd/migrate (and cmd/import, indirectly, via the same schema) reads
# migration .sql files off disk at runtime -- they're not embedded into
# the binary -- so the directory has to ship alongside it.
COPY migrations/ ./migrations/
ENTRYPOINT ["/app/api"]

FROM node:22-alpine AS frontend-build
WORKDIR /src
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build

FROM nginx:alpine AS web
COPY --from=frontend-build /src/dist /usr/share/nginx/html
COPY docker/nginx.conf /etc/nginx/conf.d/default.conf
