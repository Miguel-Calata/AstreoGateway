package protocol

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"

	"astreoGateway/internal/protocol/core"
)

// Protocol abstracts the wire-level translation between the OpenAI-compatible
// public API and an upstream provider's native protocol.
type Protocol interface {
	Name() string

	// Models discovery
	ModelsURL(base string) string
	ModelsAuth(req *http.Request, apiKey string)
	ParseModels(body []byte) ([]core.ModelEntry, error)

	// Chat
	// ChatURL returns the upstream URL for chat completions.
	// If modelInURL is true, the upstream model name is embedded in the URL
	// (e.g. Gemini) and the body does not carry a model field.
	// stream selects the streaming endpoint when the protocol uses a distinct URL.
	ChatURL(base, model string, stream bool) (url string, modelInURL bool)
	AuthHeaders(req *http.Request, apiKey string)
	SupportsEmbeddings() bool

	// TranslateRequest converts an OpenAI ChatRequest body into the upstream
	// protocol's wire format. It returns the upstream body bytes and a parsed
	// copy of the original OpenAI request (for stream options, model name, etc.).
	// For passthrough protocols (OpenAI), the body is returned as-is.
	TranslateRequest(body []byte) (upstreamBody []byte, parsedReq *core.ChatRequest, err error)

	// TranslateResponse converts an upstream non-streaming response body into
	// an OpenAI ChatResponse JSON body.
	// For passthrough protocols (OpenAI), the body is returned as-is.
	TranslateResponse(upstream []byte, model string) ([]byte, error)

	// TranslateStream reads the upstream SSE stream from r and writes the
	// OpenAI-formatted SSE to w. It is responsible for setting response
	// headers (Content-Type, Cache-Control, Connection) and writing the
	// HTTP status code before the first chunk.
	TranslateStream(r io.Reader, w http.ResponseWriter, model string, includeUsage bool, logger *slog.Logger) error
}

var registry = map[string]Protocol{}

// Register adds a Protocol to the global registry. Called from init() of
// each protocol implementation package.
func Register(p Protocol) {
	registry[p.Name()] = p
}

// Get returns the Protocol for the given name, or nil if not registered.
func Get(name string) Protocol {
	return registry[name]
}

// Known returns all registered protocol names.
func Known() []string {
	out := make([]string, 0, len(registry))
	for k := range registry {
		out = append(out, k)
	}
	return out
}

// Validate checks whether a protocol name is registered.
func Validate(name string) error {
	if _, ok := registry[name]; !ok {
		return fmt.Errorf("protocol must be one of: %v", Known())
	}
	return nil
}
