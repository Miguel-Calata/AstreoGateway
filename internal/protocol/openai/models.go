package openai

import (
	"io"

	"astreoGateway/internal/protocol/core"
)

// Re-export core types for backward compatibility.
type ModelEntry = core.ModelEntry
type ModelsResponse = core.ModelsResponse

// ParseModelsResponse delegates to core.ParseModelsResponse.
func ParseModelsResponse(r io.Reader) (*ModelsResponse, error) {
	return core.ParseModelsResponse(r)
}
