# syntax=docker/dockerfile:1.7

# ---- UI build (Vite -> /web-dist) ----
FROM node:20-alpine AS ui-build
WORKDIR /ui
COPY ui/package.json ui/package-lock.json* ./
RUN npm ci --no-audit --no-fund
COPY ui/ ./
# vite.config.ts builds into ../internal/web/dist; resolve to an absolute path.
RUN mkdir -p /web-dist && npm run build && cp -r ../internal/web/dist/. /web-dist/

# ---- Go build (embeds ui assets) ----
FROM golang:1.25-alpine AS go-build
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
# Ensure the embedded dist has the freshly built UI (overwrites .gitkeep-only dir).
COPY --from=ui-build /web-dist ./internal/web/dist
RUN go build -trimpath -ldflags="-s -w" -o /out/aigw ./cmd/aigw

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go-build /out/aigw /app/aigw
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/app/aigw"]
CMD ["-addr", ":8080", "-db", "/app/data/aigw.db"]