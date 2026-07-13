package anthropic

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"astreoGateway/internal/protocol/openai"
)

func OpenAIToAnthropic(req *openai.ChatRequest, modelName string) (*MessagesRequest, error) {
	out := &MessagesRequest{
		Model:       modelName,
		Stream:      req.Stream,
		Temperature: req.Temperature,
		TopP:        req.TopP,
		MaxTokens:   DefaultMaxTokens,
	}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		out.MaxTokens = *req.MaxTokens
	}

	if stops, err := parseStop(req.Stop); err != nil {
		return nil, err
	} else if len(stops) > 0 {
		out.StopSequences = stops
	}

	var systemParts []string
	var messages []Message
	var pendingToolResults []ContentBlock

	flushToolResults := func() {
		if len(pendingToolResults) == 0 {
			return
		}
		messages = append(messages, Message{Role: "user", Content: pendingToolResults})
		pendingToolResults = nil
	}

	for _, m := range req.Messages {
		switch m.Role {
		case "system":
			text, err := contentToText(m.Content)
			if err != nil {
				return nil, err
			}
			if text != "" {
				systemParts = append(systemParts, text)
			}
		case "user":
			flushToolResults()
			blocks, err := openAIContentToBlocks(m.Content)
			if err != nil {
				return nil, err
			}
			messages = append(messages, Message{Role: "user", Content: blocks})
		case "assistant":
			flushToolResults()
			var blocks []ContentBlock
			if len(m.Content) > 0 && string(m.Content) != "null" {
				b, err := openAIContentToBlocks(m.Content)
				if err != nil {
					return nil, err
				}
				blocks = append(blocks, b...)
			}
			for _, tc := range m.ToolCalls {
				input := json.RawMessage(tc.Function.Arguments)
				if !json.Valid(input) {
					input = json.RawMessage("{}")
				}
				blocks = append(blocks, ContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Function.Name,
					Input: input,
				})
			}
			if len(blocks) == 0 {
				blocks = []ContentBlock{{Type: "text", Text: ""}}
			}
			messages = append(messages, Message{Role: "assistant", Content: blocks})
		case "tool":
			text, err := contentToText(m.Content)
			if err != nil {
				return nil, err
			}
			pendingToolResults = append(pendingToolResults, ContentBlock{
				Type:      "tool_result",
				ToolUseID: m.ToolCallID,
				Content:   text,
			})
		default:
			return nil, fmt.Errorf("unsupported role: %s", m.Role)
		}
	}
	flushToolResults()

	if len(systemParts) > 0 {
		out.System = strings.Join(systemParts, "\n\n")
	}
	if len(messages) == 0 {
		return nil, fmt.Errorf("no messages after filtering system")
	}
	out.Messages = messages

	if len(req.Tools) > 0 {
		tools := make([]Tool, 0, len(req.Tools))
		for _, t := range req.Tools {
			schema := t.Function.Parameters
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			tools = append(tools, Tool{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				InputSchema: schema,
			})
		}
		out.Tools = tools
	}

	if len(req.ToolChoice) > 0 {
		tc, err := mapToolChoice(req.ToolChoice)
		if err != nil {
			return nil, err
		}
		out.ToolChoice = tc
	}

	return out, nil
}

func parseStop(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" {
			return nil, nil
		}
		return []string{s}, nil
	}
	var arr []string
	if err := json.Unmarshal(raw, &arr); err != nil {
		return nil, fmt.Errorf("invalid stop: %w", err)
	}
	return arr, nil
}

func contentToText(raw json.RawMessage) (string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	var parts []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return "", fmt.Errorf("invalid content: %w", err)
	}
	var b strings.Builder
	for _, p := range parts {
		typ := ""
		_ = json.Unmarshal(p["type"], &typ)
		if typ == "image_url" || typ == "image" {
			return "", fmt.Errorf("image inputs not supported in v1")
		}
		if typ == "text" || typ == "" {
			var t string
			if err := json.Unmarshal(p["text"], &t); err == nil {
				b.WriteString(t)
			}
		}
	}
	return b.String(), nil
}

func openAIContentToBlocks(raw json.RawMessage) ([]ContentBlock, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []ContentBlock{{Type: "text", Text: ""}}, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []ContentBlock{{Type: "text", Text: s}}, nil
	}
	var parts []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("invalid content: %w", err)
	}
	var blocks []ContentBlock
	for _, p := range parts {
		typ := ""
		_ = json.Unmarshal(p["type"], &typ)
		switch typ {
		case "image_url", "image":
			return nil, fmt.Errorf("image inputs not supported in v1")
		case "text", "":
			var t string
			_ = json.Unmarshal(p["text"], &t)
			blocks = append(blocks, ContentBlock{Type: "text", Text: t})
		default:
			return nil, fmt.Errorf("unsupported content type: %s", typ)
		}
	}
	if len(blocks) == 0 {
		blocks = []ContentBlock{{Type: "text", Text: ""}}
	}
	return blocks, nil
}

func mapToolChoice(raw json.RawMessage) (json.RawMessage, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		switch s {
		case "auto":
			return json.Marshal(map[string]string{"type": "auto"})
		case "required":
			return json.Marshal(map[string]string{"type": "any"})
		case "none":
			slog.Debug("tool_choice none not supported by Anthropic; sending auto")
			return json.Marshal(map[string]string{"type": "auto"})
		default:
			return nil, fmt.Errorf("unsupported tool_choice: %s", s)
		}
	}
	var obj struct {
		Type     string `json:"type"`
		Function struct {
			Name string `json:"name"`
		} `json:"function"`
	}
	if err := json.Unmarshal(raw, &obj); err != nil {
		return nil, fmt.Errorf("invalid tool_choice: %w", err)
	}
	if obj.Type == "function" && obj.Function.Name != "" {
		return json.Marshal(map[string]string{"type": "tool", "name": obj.Function.Name})
	}
	return json.Marshal(map[string]string{"type": "auto"})
}
