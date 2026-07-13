package gemini

import (
	"encoding/json"
	"testing"

	"astreoGateway/internal/protocol/core"
)

func raw(s string) json.RawMessage { return json.RawMessage(s) }

func TestOpenAIToGeminiSystemAndText(t *testing.T) {
	req := &core.ChatRequest{
		Model: "ignored",
		Messages: []core.ChatMessage{
			{Role: "system", Content: raw(`"You are helpful"`)},
			{Role: "user", Content: raw(`"Hello"`)},
		},
	}
	out, err := OpenAIToGemini(req, "gemini-2.5-flash")
	if err != nil {
		t.Fatal(err)
	}
	if out.SystemInstruction == nil || len(out.SystemInstruction.Parts) != 1 {
		t.Fatalf("system: %+v", out.SystemInstruction)
	}
	if out.SystemInstruction.Parts[0].Text != "You are helpful" {
		t.Fatalf("system text: %q", out.SystemInstruction.Parts[0].Text)
	}
	if len(out.Contents) != 1 || out.Contents[0].Role != "user" {
		t.Fatalf("contents: %+v", out.Contents)
	}
	if out.Contents[0].Parts[0].Text != "Hello" {
		t.Fatalf("text: %q", out.Contents[0].Parts[0].Text)
	}
	if out.GenerationConfig == nil || out.GenerationConfig.MaxOutputTokens != DefaultMaxOutputTokens {
		t.Fatalf("max tokens: %+v", out.GenerationConfig)
	}
}

func TestOpenAIToGeminiTools(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.ChatMessage{
			{Role: "user", Content: raw(`"call tool"`)},
			{
				Role: "assistant",
				ToolCalls: []core.ToolCall{{
					ID:   "call_1",
					Type: "function",
					Function: core.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"city":"NYC"}`,
					},
				}},
			},
			{Role: "tool", ToolCallID: "call_1", Content: raw(`"sunny"`)},
		},
		Tools: []core.Tool{{
			Type: "function",
			Function: core.ToolFunction{
				Name:        "get_weather",
				Description: "weather",
				Parameters:  raw(`{"type":"object"}`),
			},
		}},
		ToolChoice: raw(`"auto"`),
	}
	out, err := OpenAIToGemini(req, "gemini")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Tools) != 1 || len(out.Tools[0].FunctionDeclarations) != 1 {
		t.Fatalf("tools: %+v", out.Tools)
	}
	if out.Tools[0].FunctionDeclarations[0].Name != "get_weather" {
		t.Fatalf("tool name: %s", out.Tools[0].FunctionDeclarations[0].Name)
	}
	if out.ToolConfig == nil || out.ToolConfig.FunctionCallingConfig.Mode != "AUTO" {
		t.Fatalf("tool config: %+v", out.ToolConfig)
	}
	if len(out.Contents) != 3 {
		t.Fatalf("expected 3 contents, got %d", len(out.Contents))
	}
	if out.Contents[1].Role != "model" || out.Contents[1].Parts[0].FunctionCall == nil {
		t.Fatalf("expected functionCall, got %+v", out.Contents[1])
	}
	if out.Contents[2].Role != "user" || out.Contents[2].Parts[0].FunctionResponse == nil {
		t.Fatalf("expected functionResponse, got %+v", out.Contents[2])
	}
	if out.Contents[2].Parts[0].FunctionResponse.Name != "get_weather" {
		t.Fatalf("functionResponse name: %s", out.Contents[2].Parts[0].FunctionResponse.Name)
	}
}

func TestOpenAIToGeminiImageRejected(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.ChatMessage{{
			Role:    "user",
			Content: raw(`[{"type":"image_url","image_url":{"url":"http://x"}}]`),
		}},
	}
	_, err := OpenAIToGemini(req, "gemini")
	if err == nil {
		t.Fatal("expected error for image")
	}
}

func TestMapToolChoiceRequired(t *testing.T) {
	tc, err := mapToolChoice(raw(`"required"`))
	if err != nil {
		t.Fatal(err)
	}
	if tc.FunctionCallingConfig.Mode != "ANY" {
		t.Fatalf("got %v", tc.FunctionCallingConfig.Mode)
	}
}

func TestMapToolChoiceFunction(t *testing.T) {
	tc, err := mapToolChoice(raw(`{"type":"function","function":{"name":"foo"}}`))
	if err != nil {
		t.Fatal(err)
	}
	if tc.FunctionCallingConfig.Mode != "ANY" {
		t.Fatalf("mode: %s", tc.FunctionCallingConfig.Mode)
	}
	if len(tc.FunctionCallingConfig.AllowedFunctionNames) != 1 || tc.FunctionCallingConfig.AllowedFunctionNames[0] != "foo" {
		t.Fatalf("allowed: %v", tc.FunctionCallingConfig.AllowedFunctionNames)
	}
}
