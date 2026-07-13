# Gaps de traducción Anthropic ↔ OpenAI

Documento abierto. v1 cubre texto + tool calling básico; lo que queda fuera
se lista aquí para iterar post-v1.

## Cubierto en v1

- Texto (mensajes `user`/`assistant`).
- System prompt: campo `system` (Anthropic) ↔ rol `system` (OpenAI).
- Tool calling básico:
  - Definición de tools (`tools` array, funciones con `name`/`description`/
    `input_schema` ↔ `parameters`).
  - Llamada: Anthropic `tool_use` content block ↔ OpenAI `tool_calls` en
    `message`.
  - Resultado: Anthropic `tool_result` content block en rol `user` ↔ OpenAI
    rol `tool`.
- Streaming SSE mensaje a mensaje (delta de texto, delta de tool_call).
- Stop reasons:
  - `end_turn` / `stop` ↔ `stop`
  - `max_tokens` ↔ `length`
  - `tool_use` ↔ `tool_calls`
  - `stop_sequence` ↔ `stop_sequence` (si aplica)

## Gaps conocidos (no cubiertos en v1)

### Tipos de contenido
- **Image inputs**: Anthropic usa `image` content blocks con base64 + media
  type; OpenAI usa `image_url` con data URL o URL. Mapeo posible, pero el
  formato de request es distinto. v1: no se traduce, se pasa el error del
  proveedor.
- **Document inputs** (Anthropic PDF docs): sin equivalente en OpenAI. v1: ignorado.

### Tool calling avanzado
- **Parallel tool calls**: OpenAI permite `parallel_tool_calls: true|false`;
  Anthropic puede devolver múltiples `tool_use` blocks por turno. v1
  traduce los blocks pero no ajusta paralelismo declarado.
- **Tool choice estructurado**:
  - OpenAI: `tool_choice: "auto"|"none"|"required"|{type:"function",
    function:{name}}`
  - Anthropic: `tool_choice: {type:"auto"|"any"|"tool", name?}`
  - v1 mapea `auto↔auto`, `none↔` (no soportado en Anthropic, se envía `auto`),
    `required↔any`, específica↔`tool`. Documentado el detalle `none`.
- **`disable_parallel_tool_use`** (Anthropic): sin mapeo.
- **Cache control** (`cache_control` en Anthropic): sin equivalente en OpenAI;
  v1 lo descarta.

### Mensajes y roles
- **System message en medio de la conversación**: OpenAI lo permite; Anthropic
  exige `system` como campo top-level (una sola string o array de blocks).
  v1: si el cliente envía múltiples `system`, se concatenan.
- **Rol `tool`**: Anthropic no tiene rol `tool`; los tool results van como
  `tool_result` blocks dentro de un mensaje `user`. v1 hace el ajuste.
- **Empty `assistant` turn**: Anthropic puede requerir un `assistant` no
  vacío antes de un `user` tras tool use; OpenAI es más permisivo. v1: si
  hay problemas, se documenta el caso.

### Stop / sampling
- `top_k`: Anthropic lo soporta; OpenAI no en chat completions. v1: si el
  cliente lo envía en formato OpenAI extendido, se descarta al traducir a
  Anthropic; si el proveedor es OpenAI y el cliente lo envía, se descarta.
- `presence_penalty` / `frequency_penalty`: OpenAI sí, Anthropic no. v1: se
  descartan al traducir a Anthropic.
- `logprobs`: distinto formato entre los dos. v1: no se traduce.
- `n` (múltiples completions): OpenAI sí; Anthropic no en el mismo sentido.
  v1: se ignora al traducir a Anthropic.

### Streaming
- **`message_start` / `message_delta` / `message_stop`** (Anthropic) ↔
  `chat.completion.chunk` (OpenAI): v1 mapea, pero eventos de Anthropic como
  `ping` se descartan y el `usage` se reporta distinto (Anthropic emite usage
  en `message_delta`/`message_start`; OpenAI en el chunk final con
  `stream_options: {include_usage: true}`).
- **`usage`**: Anthropic lo da en streaming en `message_delta`; OpenAI requiere
  `stream_options.include_usage`. v1: si el cliente pide usage, se sintetiza
  desde el final del stream Anthropic.

### Otros
- **`stop` sequences**: ambos lo soportan, formato ligeramente distinto
  (array de strings). v1 mapea directo.
- **`response_format` / JSON mode**: OpenAI sí; Anthropic no. v1: descartado
  al traducir a Anthropic (el modelo se puede instruir vía prompt).
- **`seed`**: OpenAI sí; Anthropic no. Descartado.

## Estrategia para gaps

Cuando un parámetro del cliente no tiene equivalente en el proveedor destino:
1. Si es de tipo de contenido (image, document) → 400 con mensaje claro.
2. Si es sampling/avanzado → se descarta silenciosamente y se loguea a `debug`.
3. Si es tool choice estructurado → mejor esfuerzo con mapeo documentado arriba.

Errores 5xx del proveedor durante streaming ya iniciado: se cierra el stream
del cliente con un evento `error` (formato OpenAI) para que el cliente lo
interprete como fallo, no como finalización normal.

---

# Gaps de traducción Gemini ↔ OpenAI

v1 cubre texto + tool calling básico + streaming + discovery. Lo demás queda
fuera para post-v1.

## Cubierto en v1

- Texto (`user`/`assistant` ↔ `user`/`model` + `parts[].text`).
- System prompt: rol `system` OpenAI ↔ `systemInstruction`.
- Tools: OpenAI `tools[].function` ↔ `tools[].functionDeclarations`.
- Tool calls: OpenAI `tool_calls` ↔ `parts[].functionCall` (args JSON object).
- Tool results: rol `tool` ↔ `parts[].functionResponse` en rol `user`
  (`response: {"result": "<content>"}`).
- `tool_choice`: `auto`→`AUTO`, `required`→`ANY`, `none`→`NONE`,
  función concreta → `ANY` + `allowedFunctionNames`.
- Streaming SSE (`:streamGenerateContent?alt=sse`) → `chat.completion.chunk`.
- Finish reasons: `STOP`→`stop`, `MAX_TOKENS`→`length`,
  `SAFETY`/`RECITATION`/…→`content_filter`; presencia de `functionCall`→
  `tool_calls`.
- Usage: `usageMetadata` ↔ `usage`.
- Discovery: lista modelos, strip de prefijo `models/`, filtro
  `generateContent`.

## Gaps conocidos (no cubiertos en v1)

### Tipos de contenido
- **Multimodal** (`inlineData` imagen/audio/video): no se traduce; image
  inputs OpenAI → 400.
- **Thoughts / thinking parts**: no se reenvían al cliente.

### Sampling / request
- **`topK`**: Gemini sí; OpenAI chat no estándar → no se reenvía desde el
  cliente.
- **`n` / múltiples candidates**: se fuerza un solo candidate.
- **`logprobs`**: no soportado.
- **`response_format` / JSON mode**: no mapeado a Gemini
  `responseMimeType` / schema en v1.
- **`seed`**, **presence/frequency_penalty**: descartados.

### Tool calling
- **IDs de tool call**: Gemini no siempre expone id estable; v1 sintetiza
  `call_<i>` en la respuesta OpenAI.
- **functionResponse envelope**: se envuelve el content string como
  `{"result": "..."}`; no se rehidrata JSON estructurado del cliente.
- **Parallel tool calls**: se traducen múltiples parts; sin flag de
  paralelismo.

### Streaming
- Gemini emite chunks con el mismo shape que la response completa; no hay
  eventos tipados Anthropic-style. v1 acumula deltas de texto y functionCall
  completo por chunk.
- Usage suele llegar en el chunk final; se emite en el chunk de cierre si
  `stream_options.include_usage`.

### API surface
- **Interactions API** (stateful v1beta2): no usada; solo `generateContent`.
- **Embeddings** (`:embedContent`): rechazado en el gateway (400).
- **Vertex AI** vs AI Studio: se asume AI Studio
  (`generativelanguage.googleapis.com` + `x-goog-api-key`).