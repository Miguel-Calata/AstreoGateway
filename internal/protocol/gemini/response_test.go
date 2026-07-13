package gemini

import (
	"encoding/json"
	"testing"
)

func TestGeminiToOpenAIText(t *testing.T) {
	resp := &GenerateContentResponse{
		Candidates: []Candidate{{
			Content: &Content{
				Role:  "model",
				Parts: []Part{{Text: "Hi"}},
			},
			FinishReason: "STOP",
		}},
		UsageMetadata: &UsageMetadata{
			PromptTokenCount:     10,
			CandidatesTokenCount: 2,
			TotalTokenCount:      12,
		},
	}
	out, err := GeminiToOpenAI(resp, "gemini-2.5-flash")
	if err != nil {
		t.Fatal(err)
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

func TestGeminiToOpenAIFunctionCall(t *testing.T) {
	resp := &GenerateContentResponse{
		Candidates: []Candidate{{
			Content: &Content{
				Parts: []Part{{
					FunctionCall: &FunctionCall{
						Name: "search",
						Args: json.RawMessage(`{"q":"x"}`),
					},
				}},
			},
			FinishReason: "STOP",
		}},
	}
	out, err := GeminiToOpenAI(resp, "gemini")
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

func TestMapFinishReason(t *testing.T) {
	if mapFinishReason("MAX_TOKENS") != "length" {
		t.Fatal()
	}
	if mapFinishReason("SAFETY") != "content_filter" {
		t.Fatal()
	}
	if mapFinishReason("STOP") != "stop" {
		t.Fatal()
	}
}
