# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS go-build
WORKDIR /src

COPY go.mod go.sum* ./
RUN go mod download

COPY . .
RUN go build -trimpath -ldflags="-s -w" -o /out/aigw ./cmd/aigw

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=go-build /out/aigw /app/aigw
USER nonroot
EXPOSE 8080
ENTRYPOINT ["/app/aigw"]
CMD ["-addr", ":8080", "-db", "/app/data/aigw.db"]
