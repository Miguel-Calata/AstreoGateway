package openai

import (
	"encoding/json"
	"fmt"
	"io"
)

type ModelEntry struct {
	ID      string `json:"id"`
	Object  string `json:"object,omitempty"`
	OwnedBy string `json:"owned_by,omitempty"`
}

type ModelsResponse struct {
	Object string       `json:"object"`
	Data   []ModelEntry `json:"data"`
}

func ParseModelsResponse(r io.Reader) (*ModelsResponse, error) {
	var resp ModelsResponse
	if err := json.NewDecoder(r).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode models response: %w", err)
	}
	return &resp, nil
}
