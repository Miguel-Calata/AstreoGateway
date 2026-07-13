package anthropic

import (
	"encoding/json"
	"fmt"

	"astreoGateway/internal/protocol/openai"
)

func AnthropicToOpenAI(resp *MessagesResponse, model string) (*openai.ChatResponse, error) {
	if resp == nil {
		return nil, fmt.Errorf("nil anthropic response")
	}
	msg := openai.ChatMessage{Role: "assistant"}
	var textParts []string
	var toolCalls []openai.ToolCall
	for _, b := range resp.Content {
		switch b.Type {
		case "text":
			textParts = append(textParts, b.Text)
		case "tool_use":
			args := "{}"
			if len(b.Input) > 0 {
				args = string(b.Input)
			}
			toolCalls = append(toolCalls, openai.ToolCall{
				ID:   b.ID,
				Type: "function",
				Function: openai.FunctionCall{
					Name:      b.Name,
					Arguments: args,
				},
			})
		}
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

	out := &openai.ChatResponse{
		ID:     "chatcmpl-" + resp.ID,
		Object: "chat.completion",
		Model:  model,
		Choices: []openai.Choice{{
			Index:        0,
			Message:      msg,
			FinishReason: mapStopReason(resp.StopReason),
		}},
		Usage: &openai.Usage{
			PromptTokens:     resp.Usage.InputTokens,
			CompletionTokens: resp.Usage.OutputTokens,
			TotalTokens:      resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
	return out, nil
}

func mapStopReason(r string) string {
	switch r {
	case "max_tokens":
		return "length"
	case "tool_use":
		return "tool_calls"
	case "end_turn", "stop_sequence", "stop":
		return "stop"
	default:
		if r == "" {
			return "stop"
		}
		return r
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
