# syntax=docker/dockerfile:1

FROM golang:1.23-bookworm AS go-build
WORKDIR /src
COPY go.work go.work.sum ./
COPY apps/api/go.mod ./apps/api/
RUN cd apps/api && go mod download
COPY apps/api ./apps/api
RUN CGO_ENABLED=1 go build -o /out/api ./apps/api/cmd/api \
    && CGO_ENABLED=1 go build -o /out/worker ./apps/api/cmd/worker \
    && CGO_ENABLED=1 go build -o /out/migrate ./apps/api/cmd/migrate

FROM debian:bookworm-slim AS go-runtime
RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*
WORKDIR /app

FROM go-runtime AS api
COPY --from=go-build /out/api /app/api
COPY --from=go-build /out/migrate /app/migrate
CMD ["/app/api"]

FROM go-runtime AS worker
COPY --from=go-build /out/worker /app/worker
CMD ["/app/worker"]

FROM node:22-bookworm-slim AS web-deps
WORKDIR /repo
RUN corepack enable
COPY package.json pnpm-lock.yaml pnpm-workspace.yaml ./
COPY apps/web/package.json ./apps/web/package.json
RUN pnpm install --frozen-lockfile

FROM web-deps AS web-build
COPY apps/web ./apps/web
RUN pnpm --dir apps/web build

FROM node:22-bookworm-slim AS web
ENV NODE_ENV=production
WORKDIR /app
COPY --from=web-build /repo/apps/web/.next/standalone ./
COPY --from=web-build /repo/apps/web/.next/static ./apps/web/.next/static
EXPOSE 3090
CMD ["node", "apps/web/server.js"]
