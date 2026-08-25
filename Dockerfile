# syntax=docker/dockerfile:1.7

FROM node:24.17-alpine AS frontend-builder

WORKDIR /src/frontend
COPY frontend/package*.json ./
RUN --mount=type=cache,target=/root/.npm,sharing=locked \
    npm ci --prefer-offline --no-audit --no-fund
COPY frontend/ ./
RUN npm run build

FROM golang:1.26.4-alpine AS backend-builder

ARG GOPROXY=https://goproxy.cn,direct
ENV GOPROXY=${GOPROXY}

WORKDIR /src/backend
COPY backend/go.mod backend/go.sum ./
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    go mod download
COPY backend/cmd ./cmd
RUN --mount=type=cache,target=/go/pkg/mod,sharing=locked \
    --mount=type=cache,target=/root/.cache/go-build,sharing=locked \
    CGO_ENABLED=0 GOOS=linux go build \
      -tags=nomsgpack \
      -trimpath \
      -ldflags="-s -w" \
      -o /bin/modernfonts \
      ./cmd/server

FROM alpine:3.21

ARG ALPINE_MIRROR=https://mirrors.cloud.tencent.com/alpine

WORKDIR /app
ENV GIN_MODE=release \
    PUBLIC_DIR=/app/public

RUN sed -i \
      -e "s#https://dl-cdn.alpinelinux.org/alpine#${ALPINE_MIRROR}#g" \
      -e "s#http://dl-cdn.alpinelinux.org/alpine#${ALPINE_MIRROR}#g" \
      /etc/apk/repositories \
    && apk add --no-cache ca-certificates su-exec tzdata \
    && adduser -D -h /home/appuser appuser \
    && mkdir -p /app/data \
    && chown -R appuser:appuser /app

COPY --from=backend-builder /bin/modernfonts /app/modernfonts
COPY --from=frontend-builder /src/frontend/dist /app/public
COPY backend/docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

EXPOSE 8080

ENTRYPOINT ["docker-entrypoint.sh"]
CMD ["/app/modernfonts"]
