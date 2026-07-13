# syntax=docker/dockerfile:1.7

# Build stage: Go binary
FROM golang:1.22-alpine AS go-build
WORKDIR /src

# Cache deps first
COPY go.mod go.sum* ./
RUN go mod download

# Copy source and build
COPY . .
# UI assets expected at internal/web/dist (may be empty in early milestones)
RUN go build -trimpath -ldflags="-s -w" -o /out/aigw ./cmd/aigw

# Runtime stage: minimal image, no shell needed
FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go-build /out/aigw /app/aigw
# Persist data dir
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/app/aigw"]
CMD ["-addr", ":8080", "-db", "/app/data/aigw.db"]