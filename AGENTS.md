# AGENTS.md — AstreoGateway

Gateway ligero en Go compatible con la API de OpenAI que unifica múltiples
proveedores bajo un único endpoint. Pensado para ser simple, rápido y fácil
de desplegar.

## Stack

- **Lenguaje**: Go (versión en `go.mod`)
- **Router HTTP**: `github.com/go-chi/chi/v5` (ligero, sin generar código)
- **SQLite**: `modernc.org/sqlite` (puro Go, sin CGO, Docker multi-arch trivial)
- **Sin ORM**: `database/sql` + scans explícitos
- **Migraciones**: hand-rolled con `embed.FS` + tabla `schema_version`
- **UI**: SPA React/Vite + Tailwind + shadcn/ui, assets embebidos en el binario (`internal/web/dist`)
- **Sin frameworks pesados, sin CGO, sin YAML**

## Estructura

```
cmd/aigw/          entrypoint
internal/
  config/          env + flags
  model/           tipos de dominio (structs puros)
  store/           SQLite + migraciones embebidas + repositorios
  protocol/        interfaz Protocol + registry (despacho por protocol string)
    core/          tipos OpenAI-lingua-franca compartidos
    openai/        passthrough OpenAI-compatible
    anthropic/     traducción bidireccional (texto + tool básico en v1)
    gemini/        traducción Gemini REST v1beta (texto + tool + stream v1)
    registry/      Register(openai|anthropic|gemini) en init
  discovery/       GET /v1/models por proveedor, cache TTL, marcaje stale
  routing/         random / round_robin / priority / failover
  keypool/         selección de keys por proveedor + cooldown
  proxy/           forward genérico vía protocol.Get (passthrough o Translate*)
  api/public/      /v1/* (OpenAI-compatible), auth gateway-keys
  api/admin/       /admin/api/* (CRUD), auth admin
  web/             assets SPA embebidos (embed.FS) con fallback a index.html
  metrics/         counters en memoria, dashboard — placeholder
ui/                fuente del proyecto Vite (React + shadcn/ui + Tailwind)
  src/features/    cada feature Providers, Aliases, GatewayKeys, Discovery, Overview
src/lib/         api.ts (fetch wrapper + tipos), queries.ts (TanStack hooks), format.ts
  src/components/  primitivos shadcn (button, dialog, select, table, switch, ...) + layout
docs/              decisiones y gaps de traducción
```

## Comandos

```bash
# Resolver dependencias (ejecutar una vez tras clonar / pulls con go.mod cambiado)
go mod tidy

# Build del binario (incluye assets de la UI si internal/web/dist existe)
go build -o bin/aigw ./cmd/aigw

# Ejecutar
go run ./cmd/aigw

# Ejecutar con flags
go run ./cmd/aigw -addr :18473 -db data/aigw.db -log-level debug

# Tests
go test ./...

# Vet
go vet ./...

# UI (desde ui/): build de Vite → internal/web/dist
cd ui && npm install && npm run build

# El binario embebe los assets; para recargarlos tras cambios en ui/
# simplemente repite npm run build && go build -o bin/aigw ./cmd/aigw
```

## Decisiones de diseño (resumen — completas en docs/decisions.md)

1. **Modelos son texto**, siempre embebidos como `{provider_id, model_name}`.
   No son entidades en la base de datos.
2. **Identificación**: `slug:model` (parsing `SplitN(s, ":", 2)`).
   Prefijo = `providers.slug` (estable); `name` es solo UI; `id` es UUID interno.
   Razón: ningún modelo real usa `:`; soporta nombres HF-style con slashes.
3. **Sin prefijo → 404**: el gateway no adivina. El cliente (opencode, Cursor)
   hace discovery y siempre envía el prefijo.
4. **Alias**: `[]{provider_id, model_name}`, cross-provider. Routing por alias
   (random / round_robin / priority / failover). Acceso directo a un modelo
   real vía `provider:model` salta el routing.
5. **Modelo obsoleto (`stale`)**: en memoria, se excluye de la rotación, se
   autorrecupera si reaparece en un refresh. Si todos los targets de un alias
   quedan stale → 503 "alias has no available targets".
6. **Auth entrada**: el gateway emite sus propias bearer keys (tabla
   `gateway_keys`). Sin key válida → 401 en `/v1/*`.
7. **Admin bootstrap**: la 1ª vez `/admin` está abierto; se crea el primer
   usuario admin desde dentro. Tras eso, cookie HMAC + bcrypt.
8. **Traducción Anthropic v1**: texto + tool calling básico. Gaps en
   `docs/translation-gaps.md`.
9. **Traducción Gemini v1**: REST `v1beta` (`generateContent` /
   `streamGenerateContent?alt=sse`), texto + tools + discovery. Modelo en URL;
   auth `x-goog-api-key`. Gaps en `docs/translation-gaps.md`. Decisiones §16.
10. **Streaming**: passthrough byte-stream cuando el protocolo del proveedor
    coincide con el del cliente; traducción evento a evento vía
    `Protocol.TranslateStream` cuando no.
11. **`/v1/embeddings`**: solo protocolos con `SupportsEmbeddings()` (OpenAI).
    Anthropic y Gemini → 400.
12. **Sin CGO**: `modernc.org/sqlite` permite cross-compile y Docker multi-arch
    sin requisitos especiales.

## Boot order

1. Parse flags + env → `config.Config`
2. Abrir SQLite (crear si no existe) → aplicar migraciones embebidas
3. Estado en memoria (keypool posiciones, discovery caches) se inicializa vacío
4. Levantar router `chi`: `/v1/*` (público), `/admin/api/*` (admin),
   `/admin/*` (assets SPA)
5. `ListenAndServe`

## Milestones

1. Esqueleto + store + migraciones     ✓
2. Admin API + bootstrap               ✓ (backend)
3. UI mínima embebida                  ✓ (Vite React SPA + login + CRUD + drag aliases)
4. Discovery + `/v1/models`            ✓
5. Proxy passthrough OpenAI→OpenAI     ✓
6. Traducción Anthropic               ✓ (v1)
7. Traducción Gemini                  ✓ (v1: texto + tools + stream + discovery)
8. `/v1/embeddings`                    ✓
9. Métricas + estado de salud          parcial (`GET /healthz`; métricas pendientes)
10. Docker + docs finales              ✓ (Dockerfile alineado a go.mod)

## Known issues

- **Provider secrets en SQLite**: `api_keys.key_value` se guarda en claro en DB
  (list/get admin ya no lo devuelven; create sí, una vez).

## Deploy

Sprints de deploy en `docs/deploy-sprints.mc` (S01–S05 done).
Runbook de producción: `docs/deploy.md`.

## Convenciones

- Sin comentarios en el código salvo que la lógica los justifique.
- Errores con `fmt.Errorf("contexto: %w", err)`.
- IDs: strings (ulid o uuid v7) generados en el dominio, no autoincrement en SQLite.
- Sin logs ruidosos: `slog` estructurado, nunca `fmt.Println` en producción.
- `internal/` para todo; nada exported fuera del binario.
