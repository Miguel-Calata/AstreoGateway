package gemini

import (
	"encoding/json"
	"fmt"

	"astreoGateway/internal/protocol/core"
)

func GeminiToOpenAI(resp *GenerateContentResponse, model string) (*core.ChatResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil gemini response")
	}
	msg := core.ChatMessage{Role: "assistant"}
	var textParts []string
	var toolCalls []core.ToolCall
	finishReason := "stop"

	if len(resp.Candidates) > 0 {
		c := resp.Candidates[0]
		if c.FinishReason != "" {
			finishReason = mapFinishReason(c.FinishReason)
		}
		if c.Content != nil {
			for i, p := range c.Content.Parts {
				if p.Text != "" {
					textParts = append(textParts, p.Text)
				}
				if p.FunctionCall != nil {
					args := "{}"
					if len(p.FunctionCall.Args) > 0 {
						args = string(p.FunctionCall.Args)
					}
					id := fmt.Sprintf("call_%d", i)
					toolCalls = append(toolCalls, core.ToolCall{
						ID:   id,
						Type: "function",
						Function: core.FunctionCall{
							Name:      p.FunctionCall.Name,
							Arguments: args,
						},
					})
				}
			}
		}
	}
	if len(toolCalls) > 0 && finishReason == "stop" {
		finishReason = "tool_calls"
	}
	if len(textParts) > 0 {
		content, _ := json.Marshal(joinText(textParts))
		msg.Content = content
	} else {
		msg.Content = json.RawMessage(`""`)
	}
	if len(toolCalls) > 0 {
		msg.ToolCalls = toolCalls
	}

	out := &core.ChatResponse{
		ID:     "chatcmpl-gemini",
		Object: "chat.completion",
		Model:  model,
		Choices: []core.Choice{{
			Index:        0,
			Message:      msg,
			FinishReason: finishReason,
		}},
	}
	if resp.UsageMetadata != nil {
		out.Usage = &core.Usage{
			PromptTokens:     resp.UsageMetadata.PromptTokenCount,
			CompletionTokens: resp.UsageMetadata.CandidatesTokenCount,
			TotalTokens:      resp.UsageMetadata.TotalTokenCount,
		}
		if out.Usage.TotalTokens == 0 {
			out.Usage.TotalTokens = out.Usage.PromptTokens + out.Usage.CompletionTokens
		}
	}
	return out, nil
}

func mapFinishReason(r string) string {
	switch r {
	case "STOP":
		return "stop"
	case "MAX_TOKENS":
		return "length"
	case "SAFETY", "RECITATION", "BLOCKLIST", "PROHIBITED_CONTENT", "SPII":
		return "content_filter"
	case "OTHER", "FINISH_REASON_UNSPECIFIED", "":
		return "stop"
	default:
		return "stop"
	}
}

func joinText(parts []string) string {
	if len(parts) == 1 {
		return parts[0]
	}
	out := ""
	for _, p := range parts {
		out += p
	}
	return out
}
