package anthropic

import (
	"encoding/json"
	"testing"
)

func TestAnthropicToOpenAIText(t *testing.T) {
	resp := &MessagesResponse{
		ID:         "msg_1",
		StopReason: "end_turn",
		Content:    []ContentBlock{{Type: "text", Text: "Hi"}},
		Usage:      Usage{InputTokens: 10, OutputTokens: 2},
	}
	out, err := AnthropicToOpenAI(resp, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if out.ID != "chatcmpl-msg_1" {
		t.Fatalf("id: %s", out.ID)
	}
	if out.Choices[0].FinishReason != "stop" {
		t.Fatalf("finish: %s", out.Choices[0].FinishReason)
	}
	var content string
	json.Unmarshal(out.Choices[0].Message.Content, &content)
	if content != "Hi" {
		t.Fatalf("content: %q", content)
	}
	if out.Usage.TotalTokens != 12 {
		t.Fatalf("usage: %+v", out.Usage)
	}
}

func TestAnthropicToOpenAIToolUse(t *testing.T) {
	resp := &MessagesResponse{
		ID:         "msg_2",
		StopReason: "tool_use",
		Content: []ContentBlock{{
			Type:  "tool_use",
			ID:    "tu_1",
			Name:  "search",
			Input: json.RawMessage(`{"q":"x"}`),
		}},
	}
	out, err := AnthropicToOpenAI(resp, "claude")
	if err != nil {
		t.Fatal(err)
	}
	if out.Choices[0].FinishReason != "tool_calls" {
		t.Fatalf("finish: %s", out.Choices[0].FinishReason)
	}
	if len(out.Choices[0].Message.ToolCalls) != 1 {
		t.Fatalf("tool_calls: %+v", out.Choices[0].Message.ToolCalls)
	}
	if out.Choices[0].Message.ToolCalls[0].Function.Name != "search" {
		t.Fatalf("name: %s", out.Choices[0].Message.ToolCalls[0].Function.Name)
	}
}

func TestMapStopReason(t *testing.T) {
	if mapStopReason("max_tokens") != "length" {
		t.Fatal()
	}
	if mapStopReason("tool_use") != "tool_calls" {
		t.Fatal()
	}
}
