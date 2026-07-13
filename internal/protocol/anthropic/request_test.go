package anthropic

import (
	"encoding/json"
	"testing"

	"astreoGateway/internal/protocol/core"
)

func TestOpenAIToAnthropicSystemAndText(t *testing.T) {
	req := &core.ChatRequest{
		Model: "ignored",
		Messages: []core.ChatMessage{
			{Role: "system", Content: raw(`"You are helpful"`)},
			{Role: "user", Content: raw(`"Hello"`)},
		},
	}
	out, err := OpenAIToAnthropic(req, "claude-sonnet-4")
	if err != nil {
		t.Fatal(err)
	}
	if out.Model != "claude-sonnet-4" {
		t.Fatalf("model: %s", out.Model)
	}
	if out.System != "You are helpful" {
		t.Fatalf("system: %q", out.System)
	}
	if out.MaxTokens != DefaultMaxTokens {
		t.Fatalf("max_tokens default: %d", out.MaxTokens)
	}
	if len(out.Messages) != 1 || out.Messages[0].Role != "user" {
		t.Fatalf("messages: %+v", out.Messages)
	}
	if out.Messages[0].Content[0].Text != "Hello" {
		t.Fatalf("text: %q", out.Messages[0].Content[0].Text)
	}
}

func TestOpenAIToAnthropicTools(t *testing.T) {
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
	out, err := OpenAIToAnthropic(req, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Tools) != 1 || out.Tools[0].Name != "get_weather" {
		t.Fatalf("tools: %+v", out.Tools)
	}
	if len(out.Messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(out.Messages))
	}
	if out.Messages[1].Content[0].Type != "tool_use" {
		t.Fatalf("expected tool_use, got %s", out.Messages[1].Content[0].Type)
	}
	if out.Messages[2].Content[0].Type != "tool_result" {
		t.Fatalf("expected tool_result, got %s", out.Messages[2].Content[0].Type)
	}
}

func TestOpenAIToAnthropicImageRejected(t *testing.T) {
	req := &core.ChatRequest{
		Messages: []core.ChatMessage{{
			Role:    "user",
			Content: raw(`[{"type":"image_url","image_url":{"url":"http://x"}}]`),
		}},
	}
	_, err := OpenAIToAnthropic(req, "claude")
	if err == nil {
		t.Fatal("expected error for image")
	}
}

func TestMapToolChoiceRequired(t *testing.T) {
	tc, err := mapToolChoice(raw(`"required"`))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]string
	json.Unmarshal(tc, &m)
	if m["type"] != "any" {
		t.Fatalf("got %v", m)
	}
}

func raw(s string) json.RawMessage { return json.RawMessage(s) }
