# AstreoGateway

Gateway ligero para modelos de IA, compatible con la API de OpenAI. Unifica
múltiples proveedores (OpenAI, Anthropic, OpenRouter, Groq, ...) bajo un
único endpoint con routing, alias y administración web embebida.

## Objetivo

Un binario. Sin YAML. Bajo consumo. Compatible con cualquier cliente que
hable OpenAI (Cursor, Continue, Open WebUI, LibreChat, Cline, opencode, ...).

## Estado

**Backend core usable** (admin API, discovery, chat OpenAI/Anthropic con
traducción). UI embebida, embeddings y métricas/health están pendientes.
Ver milestones en `AGENTS.md`.

| Área | Estado |
|------|--------|
| Store + migraciones SQLite | Hecho |
| Admin API + bootstrap | Hecho |
| Discovery + `GET /v1/models` | Hecho |
| Proxy OpenAI → OpenAI | Hecho |
| Traducción Anthropic (texto + tools v1) | Hecho |
| UI admin embebida | Pendiente (501) |
| `/v1/embeddings` | Pendiente (501) |
| Métricas + health | Pendiente |
| Docker + docs de deploy | Parcial |

## Quick start

> Requiere Go según `go.mod` (hoy 1.25). Node.js solo cuando exista la UI (M3).

```bash
go mod tidy
go run ./cmd/aigw -addr :8080 -db data/aigw.db -log-level debug
```

Flags / env: `-addr` (`AIGW_ADDR`), `-db` (`AIGW_DB`), `-log-level`,
`-discovery-ttl`, `-discovery-timeout`, `-proxy-timeout`, `-key-cooldown`.

### Bootstrap mínimo (sin UI)

```bash
# 1. Crear primer admin (solo si no hay usuarios)
curl -s -X POST http://localhost:8080/admin/api/bootstrap \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"changeme"}'

# 2. Login (cookie de sesión)
curl -s -c cookies.txt -X POST http://localhost:8080/admin/api/login \
  -H 'Content-Type: application/json' \
  -d '{"username":"admin","password":"changeme"}'

# 3. Provider + API key upstream + gateway key de entrada
# (POST /admin/api/providers, .../api-keys, /admin/api/gateway-keys)
# 4. Chat con Bearer de la gateway key
curl -s http://localhost:8080/v1/chat/completions \
  -H "Authorization: Bearer <gateway_key>" \
  -H 'Content-Type: application/json' \
  -d '{"model":"openai:gpt-4o-mini","messages":[{"role":"user","content":"hola"}]}'
```

## Endpoints

| Endpoint | Estado |
|----------|--------|
| `GET /v1/models` | Implementado — modelos `provider:model` + alias |
| `POST /v1/chat/completions` | Implementado — passthrough OpenAI o traducción Anthropic |
| `POST /v1/embeddings` | Stub 501 (milestone 7) |
| `/admin/api/*` | Implementado — bootstrap, auth, CRUD, discovery |
| `/admin/*` (UI) | Stub 501 (milestone 3) |
| Health / métricas | No existen aún (milestone 8) |

**Auth:** bearer `gateway_keys` en `/v1/*`; cookie sesión HMAC en `/admin/api/*`
tras bootstrap.

## Identificación de modelos

`provider:model`. Ejemplos:

- `openai:gpt-5`
- `anthropic:claude-sonnet-4`
- `openrouter:meta-llama/Llama-3.3-70B` (modelo HF-style, slashes intactos)

Sin prefijo: se busca como alias. Si no existe → 404.

## Known issues

- El keypool de API keys solo se carga al arranque; crear/editar/borrar keys
  en admin no recarga el pool hasta reiniciar.
- La URL de chat OpenAI concatena `BaseURL + "/v1/chat/completions"`; si el
  base ya incluye `/v1` puede quedar `.../v1/v1/...`.
- List/get de API keys de proveedor en admin devuelve el secret en JSON.
- Cookie de sesión admin sin flag `Secure`.
- UI no embebida; Dockerfile usa Go 1.22 mientras `go.mod` declara 1.25.

## Documentación

- `AGENTS.md` — stack, estructura, milestones, convenciones.
- `docs/decisions.md` — decisiones de diseño con rationale.
- `docs/translation-gaps.md` — gaps de traducción Anthropic↔OpenAI.
