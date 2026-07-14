# Decisiones de diseño

Documento vivo. Cada decisión numerada con contexto y rationale.

## 1. Modelos son texto, no entidades

Los modelos (`gpt-5`, `claude-sonnet-4`, `meta-llama/Llama-3.3-70B`) **no** son
una tabla en SQLite. Se almacenan siempre embebidos en un contexto de
proveedor: `{provider_id, model_name}`.

**Razón**: la fuente de verdad de qué modelos existen es el proveedor
(`GET /v1/models` upstream), no el gateway. Replicarlos como entidades
persistentes obliga a sincronizar estado, gestionar referencias huérfanas y
corre migrations cuando un proveedor añade un modelo. El gateway solo
descubre (cache TTL en memoria) y enruta.

**Metadatos opcionales** (cost, latencia) si en el futuro se implementa
*Cost Aware* routing: se persistirían en `model_metadata(provider_id, model_name)`
con PK compuesta. No se remueve el target al desaparecer upstream — solo se
marca `stale`.

## 2. Identificación: `provider:model`

Separador elegido: `:` (dos puntos).

- Parsing trivial: `SplitN(name, ":", 2)` → `["openrouter", "meta-llama/Llama-3.3-70B"]`.
- **Sin colisión**: ningún modelo real usa `:` en su nombre. Los modelos
  HF-style (con `/`) se preservan intactos al otro lado del `:`.
- **Sin escapado**: el modelo se guarda tal cual upstream lo nombra.

Trade-off aceptado: no es el formato de facto de LiteLLM/OpenRouter
(`provider/model`). Ellos usan `/` y lidiar con modelos HF-style queda en
doble slash confuso. `provider:model` es más limpio y sin ambigüedad.

Regla de resolución:
```
name contains ":"  → provider = before first ":", model = after (slashes intact)
                      routing directo a ese proveedor, sin pasar por alias
name NO ":"        → buscar en alias
                        encontrado → routing de alias
                        no encontrado → 404 "unknown model"
```

## 3. Sin prefijo → 404

El gateway no adivina. Si un cliente envía `kimi-2.7` sin prefijo y no
existe un alias con ese nombre, el gateway responde 404. El cliente
(opencode, Cursor, etc.) es responsable del discovery y siempre debe enviar
el prefijo `provider:model`.

Justificación: intentar adivinar el proveedor introduciría comportamientos
silenciosos imposibles de depurar y rompería la transparencia.

## 4. Alias: targets cross-provider

Un alias (`coding`, `fast`, `reasoning`) tiene una lista de targets
`[]{provider_id, model_name}`. Los targets pueden ser de proveedores
distintos. Misma `model_name` en dos proveedores distintos = targets
distintos (cada uno con su `provider_id`).

Estrategias de routing por alias:
- `random`
- `round_robin`
- `priority` (orden por `position` en `alias_targets`)
- `failover` (orden por `position`, prueba el siguiente si el anterior falla)

**Selección vs reintento**: La estrategia (`random`, `rr`, `priority`, `failover`)
solo determina el **primer** target. Si ese target falla con un error recuperable,
**todos** los modos reintentan los siguientes targets por orden de `position`
(saltando stale y disabled). Acceso directo `provider:model` **no** reintenta.

### Política de errores y reintentos

| Clase | Códigos / caso | Cooldown key | Reintentar | Soft-stale | Log |
|-------|----------------|--------------|-----------|------------|-----|
| Rate limit | 429 | sí | sí | no | warn |
| Upstream down | 5xx, timeout, network error | sí (solo 5xx) | sí | no | warn |
| Model missing | 404 | no | sí | sí | warn |
| Auth / billing | 401, 403 | no | sí | no | error |
| Client bad request | 400, 422 | no | **no** | no | info |

Si se agotan targets → devolver el último error útil.
**Nunca** borrar targets de SQLite por stale.

**Mid-stream failover**: Si el stream ya empezó a escribirse y el upstream
corta (max output tokens, reset, 5xx mid-SSE), hoy no hay reintento.
Es un TODO para el futuro (buffer de headers / replay limitado).
Motivación: algunos providers limitan tokens de salida y abortan mid-stream.

Futuras: `weighted`, `least_latency`, `cost_aware`, `health_based`.

Acceso directo vía `provider:model` **salta** el routing de alias: se elige
ese proveedor directamente. Sin reintento.

Mismo `model_name` en dos proveedores como targets distintos del mismo alias:
**permitido**. Es justo lo que habilita failover real.

## 5. Estado `stale` de modelos

Dos fuentes de stale:

1. **Discovery stale**: Cuando un refresh del proveedor detecta que un modelo
   referenciado por un target ya no aparece en `/v1/models` upstream.
2. **Runtime stale**: Cuando un request a un modelo devuelve 404 (model/function
   not found), se marca soft-stale en memoria para ese target. Se limpia
   automáticamente en el siguiente refresh exitoso del proveedor.

Comportamiento en ambos casos:

- **No eliminar** el target (mutar config silenciosamente pierde intent).
- **No deshabilitar** el alias entero.
- **Marcar el target `stale=true`** en memoria (no en SQLite).
- **Excluirlo de la rotación** activa del routing (random/round-robin/etc. lo
  saltan como si estuviera en cooldown).
- **Exponerlo en la UI** con badge "stale" + acciones manuales.
- Si **todos** los targets de un alias quedan `stale` → alias `degraded` →
  `503 "alias has no available targets"` (no 404: la definición existe).
- Si el modelo **reaparece** en un refresh posterior → `stale=false`
  automáticamente, vuelve a la rotación.

Fundamento: el alias es config persistida por el admin (intencional); la
disponibilidad upstream es volátil. Separar ambas cosas evita que un cierre
temporal o discontinuación permanente te borre el routing definido.

### 5a. Validación de modelos al crear/importar

Al crear o importar aliases (UI y API):

- Si un target `provider:model` no aparece en discovery del proveedor → **warn**
  (no permitir creación salvo checkbox "Allow models not in discovery").
- El backend (`POST/PUT /admin/api/aliases`) rechaza con 400 si un model no
  está en discovery, salvo `?allow_unknown_models=1`.
- En la UI de import, el override degrada los rows "warn" a "ok".
- No se valida si discovery está vacío o no cacheado (best-effort).

## 6. Auth de entrada: gateway keys

Los clientes llaman a `/v1/*` con `Authorization: Bearer <gateway-key>`. El
gateway emite sus propias keys (tabla `gateway_keys`), independientes de las
keys de los proveedores.

- `key_hash` = sha256 hex del token (nunca el plaintext).
- `prefix` = primeros 8 chars, para lookup/display.
- Sin key válida → 401 en `/v1/*`.

## 7. Admin bootstrap

La 1ª vez que se abre `/admin` y la tabla `admin_users` está vacía, la UI
muestra el setup guiado para crear el primer admin (username + password).
Tras eso, `/admin/*` requiere sesión: cookie firmada HMAC + bcrypt para
password.

No hay env var `ADMIN_PASSWORD`: ajustarse a la filosofía "sin YAML, sin
secrets en env". El bootstrap ocurre solo una vez y queda en SQLite.

## 8. Traducción Anthropic v1

Alcance inicial:
- Texto (mensajes user/assistant).
- System prompt (campo `system` de Anthropic ↔ rol `system` de OpenAI).
- Tool calling básico:
  - OpenAI `tools`/`tool_calls`/`tool` role ↔ Anthropic `tools`/`tool_use`/
    `tool_result` content blocks.
- Stop reasons (`stop_reason` Anthropic ↔ `finish_reason` OpenAI).
- Streaming SSE traducido evento a evento.

Gaps y limitaciones documentados en `docs/translation-gaps.md`. Iteraremos
post-v1.

## 9. Streaming

- **Passthrough byte-stream** cuando el protocolo del proveedor coincide con
  el del cliente (OpenAI→OpenAI). Cero parsing, mínima latencia, binario
  ligero.
- **Traducción evento a evento** cuando los protocolos difieren
  (Anthropic/Gemini→OpenAI client). Cada paquete `protocol/<name>/stream.go`
  implementa `Protocol.TranslateStream` y re-serializa SSE al formato OpenAI.

Dos code paths, justificados por el peso del parsing de traducción. El
despacho es vía `protocol.Get(prov.Protocol)`.

## 10. `/v1/embeddings`

Solo rutear a proveedores con `SupportsEmbeddings() == true` (hoy: OpenAI).
Si el target resuelto es Anthropic o Gemini → `400 "protocol does not support
embeddings"`.

## 11. Sin CGO

`modernc.org/sqlite` (puro Go) en lugar de `mattn/go-sqlite3` (CGO).

Ventajas:
- `go build` cross-compile trivial.
- Docker multi-arch sin toolchain C.
- Binario portable.

Coste: ~10-20% más lento en writes. Aceptable para un gateway que no es
write-heavy (la tabla de config no crece con el tráfico).

## 12. Sin ORM

`database/sql` + scans explícitos. Los `model/` structs son POJOs sin tags
de ORM. Los repo funciones en `store/` hacen `Query`/`QueryRow` y `Scan`
manual. Razón: control total, sin magic, sin搞定 un generator, y las queries
complejas (agregación de alias con targets) son más legibles en SQL crudo.

## 13. Stack HTTP: chi + stdlib

`github.com/go-chi/chi/v5` para routing (middlewares limpios, rutas por
método, subrouters). El resto es stdlib: `net/http`, `log/slog`,
`database/sql`, `embed`, `crypto/hmac`, `crypto/sha256`, `golang.org/x/crypto/bcrypt`.

`chi` es ~1000 LOC, sin generar código, sin reflection. Encaja con "ligero".

## 14. Join de URLs OpenAI

Los paths upstream OpenAI se construyen con `url.Parse` + `path.Join` del
segmento relativo (`models`, `chat/completions`, …), igual que Anthropic
(`BuildMessagesURL`) y discovery (`buildModelsURL`). No se concatena
`BaseURL + "/v1/..."`.

Efecto: si el admin guarda `https://api.openai.com/v1` o
`https://api.openai.com/v1/`, el path final es `.../v1/chat/completions`
(sin doble `/v1`). Si guarda un base sin prefijo de versión
(`http://localhost:8080`), se une solo el segmento (`.../chat/completions`),
sin adivinar `/v1`.

Helper: `protocol/openai.BuildChatCompletionsURL`.

## 15. Keypool reload en caliente

`keypool.Pool.Load` se invoca al arranque y tras create/update/delete de API
keys en admin. Full reload bajo mutex: simple y correcto. Los cooldowns en
memoria se pierden en el reload (aceptable; las keys nuevas/disabled pesan
más que cooldowns efímeros de 429).

## 16. Traducción Gemini v1

Protocolo REST `v1beta` stateless (`generateContent` /
`streamGenerateContent?alt=sse`). No se usa la API Interactions stateful.

Alcance v1:
- Texto (`contents` + `parts` con `text`).
- System prompt → `systemInstruction`.
- Tool calling básico: `tools.functionDeclarations` ↔ OpenAI `tools`;
  `functionCall` / `functionResponse` ↔ `tool_calls` / rol `tool`.
- Streaming SSE (`alt=sse`) traducido a `chat.completion.chunk`.
- Discovery: `GET .../v1beta/models`, filtrado por
  `supportedGenerationMethods` que incluya `generateContent`.
- Auth: header `x-goog-api-key`.
- Modelo en la URL (`modelInURL=true`); no en el body.
- `base_url` flexible: con o sin sufijo `/v1beta` (el builder no duplica).

Embeddings: rechazados (`SupportsEmbeddings=false`). Gaps en
`docs/translation-gaps.md`.