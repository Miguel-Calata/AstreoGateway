# AstreoGateway

Gateway ligero para modelos de IA, compatible con la API de OpenAI. Unifica
múltiples proveedores (OpenAI, Anthropic, OpenRouter, Groq, ...) bajo un
único endpoint con routing, alias y administración web embebida.

## Objetivo

Un binario. Sin YAML. Bajo consumo. Compatible con cualquier cliente que
hable OpenAI (Cursor, Continue, Open WebUI, LibreChat, Cline, opencode, ...).

## Estado

**Fullstack usable** (admin SPA embebida, admin API, discovery, chat
OpenAI/Anthropic, embeddings OpenAI, healthz). Métricas ricas pendientes.
Ver milestones en `AGENTS.md`.

| Área | Estado |
|------|--------|
| Store + migraciones SQLite | Hecho |
| Admin API + bootstrap | Hecho |
| Discovery + `GET /v1/models` | Hecho |
| Proxy OpenAI → OpenAI | Hecho |
| Traducción Anthropic (texto + tools v1) | Hecho |
| UI admin embebida | Hecho (Vite React SPA en `/admin/*`) |
| `/v1/embeddings` | Hecho (OpenAI-only; Anthropic → 400) |
| Health (`/healthz`) | Hecho |
| Métricas / dashboard | Pendiente |
| Docker (compose + volume) | Hecho |
| Runbook deploy (`docs/deploy.md`) | Hecho |

## Quick start

> Requiere Go según `go.mod` (hoy 1.25). Node.js 20+ para builds de la UI.

```bash
# 1. Build de la UI embebida (primera vez / tras cambios en ui/)
cd ui && npm install && npm run build && cd ..

# 2. Build del binario (incluye la SPA embebida)
go build -o bin/aigw ./cmd/aigw

# 3. Ejecutar
./bin/aigw -addr :18473 -db data/aigw.db -log-level debug

# Alternativa rápida (sin build de UI): abre /admin y verás bootstrap/login.
# La SPA se construye antes del go build.
```

Flags / env: `-addr` (`AIGW_ADDR`), `-db` (`AIGW_DB`), `-log-level`,
`-discovery-ttl`, `-discovery-timeout`, `-proxy-timeout`, `-key-cooldown`,
`-cookie-secure` (`AIGW_COOKIE_SECURE`, default false; usar `true` detrás de HTTPS).

### Bootstrap (UI web abierta en `/admin`)

Abre `http://localhost:18473/admin/` en el navegador. La primera vez pide crear
un admin. Tras login tienes el panel: Providers, API Keys, Aliases, Gateway Keys,
Discovery.

### Bootstrap por API (headless)

```bash
# 1. Crear primer admin (solo si no hay usuarios)
curl -s -X POST http://localhost:18473/admin/api/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"changeme"}'

# 2. Login (cookie de sesión)
curl -s -c cookies.txt -X POST http://localhost:18473/admin/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"changeme"}'

# 3. Provider + API key upstream + gateway key de entrada
# (POST /admin/api/providers, .../api-keys, /admin/api/gateway-keys)
# 4. Chat con Bearer de la gateway key
curl -s http://localhost:18473/v1/chat/completions \
  -H "Authorization: Bearer <gateway_key>" \
  -H 'Content-Type: application/json' \
  -d '{"model":"openai:gpt-4o-mini","messages":[{"role":"user","content":"hola"}]}'
```

## Endpoints

| Endpoint | Estado |
|----------|--------|
| `GET /v1/models` | Implementado — modelos `provider:model` + alias |
| `POST /v1/chat/completions` | Implementado — passthrough OpenAI o traducción Anthropic |
| `POST /v1/embeddings` | Implementado — passthrough OpenAI; Anthropic → 400 |
| `/admin/api/*` | Implementado — bootstrap, auth, CRUD, discovery |
| `/admin/*` (UI) | Implementado — SPA React/Vite con login, CRUD, discovery |
| `GET /healthz` | Implementado — ping DB + uptime |
| Métricas / dashboard | Pendiente (milestone 8) |

**Auth:** bearer `gateway_keys` en `/v1/*`; cookie sesión HMAC en `/admin/api/*`
tras bootstrap.

## Identificación de modelos

`provider:model`. Ejemplos:

- `openai:gpt-5`
- `anthropic:claude-sonnet-4`
- `openrouter:meta-llama/Llama-3.3-70B` (modelo HF-style, slashes intactos)

Sin prefijo: se busca como alias. Si no existe → 404.

## Docker

```bash
# Preferido
docker compose up --build -d
curl -s http://localhost:18473/healthz

# Alternativa (un solo contenedor)
docker build -t aigw .
docker run --rm -p 18473:18473 -v aigw-data:/app/data aigw
curl -s http://localhost:18473/healthz
```

Persistencia: volume named `aigw-data` montado en `/app/data`. La imagen runtime
es distroless (sin shell); el probe de salud es HTTP a `GET /healthz` desde fuera
(orchestrator / compose sidecar).

Producción (proxy, permisos nonroot, backup, upgrade): ver [`docs/deploy.md`](docs/deploy.md).

## Known issues

- Secrets de proveedor se guardan en claro en SQLite (no se devuelven en list/get).

## Documentación

- `AGENTS.md` — stack, estructura, milestones, convenciones.
- `docs/deploy.md` — runbook de producción (compose, proxy, backup, upgrade).
- `docs/decisions.md` — decisiones de diseño con rationale.
- `docs/translation-gaps.md` — gaps de traducción Anthropic↔OpenAI.
