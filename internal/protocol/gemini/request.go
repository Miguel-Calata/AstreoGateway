package gemini

import (
	"encoding/json"
	"fmt"
	"strings"

	"astreoGateway/internal/protocol/core"
)

const DefaultMaxOutputTokens = 8192

func OpenAIToGemini(req *core.ChatRequest, modelName string) (*GenerateContentRequest, error) {
	_ = modelName
	out := &GenerateContentRequest{}

	cfg := &GenerationConfig{MaxOutputTokens: DefaultMaxOutputTokens}
	if req.MaxTokens != nil && *req.MaxTokens > 0 {
		cfg.MaxOutputTokens = *req.MaxTokens
	}
	cfg.Temperature = req.Temperature
	cfg.TopP = req.TopP
	if stops, err := parseStop(req.Stop); err != nil {
		return nil, err
	} else if len(stops) > 0 {
		cfg.StopSequences = stops
	}
	out.GenerationConfig = cfg

	var systemParts []string
	var contents []Content
	var pendingToolParts []Part
	toolNameByID := map[string]string{}

	flushToolResults := func() {
		if len(pendingToolParts) == 0 {
			return
		}
		contents = append(contents, Content{Role: "user", Parts: pendingToolParts})
		pendingToolParts = nil
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
			parts, err := openAIContentToParts(m.Content)
			if err != nil {
				return nil, err
			}
			contents = append(contents, Content{Role: "user", Parts: parts})
		case "assistant":
			flushToolResults()
			var parts []Part
			if len(m.Content) > 0 && string(m.Content) != "null" {
				p, err := openAIContentToParts(m.Content)
				if err != nil {
					return nil, err
				}
				parts = append(parts, p...)
			}
			for _, tc := range m.ToolCalls {
				args := json.RawMessage(tc.Function.Arguments)
				if !json.Valid(args) {
					args = json.RawMessage("{}")
				}
				if tc.ID != "" && tc.Function.Name != "" {
					toolNameByID[tc.ID] = tc.Function.Name
				}
				parts = append(parts, Part{
					FunctionCall: &FunctionCall{
						Name: tc.Function.Name,
						Args: args,
					},
				})
			}
			if len(parts) == 0 {
				parts = []Part{{Text: ""}}
			}
			contents = append(contents, Content{Role: "model", Parts: parts})
		case "tool":
			text, err := contentToText(m.Content)
			if err != nil {
				return nil, err
			}
			name := m.Name
			if name == "" {
				name = toolNameByID[m.ToolCallID]
			}
			if name == "" {
				name = m.ToolCallID
			}
			respObj, _ := json.Marshal(map[string]string{"result": text})
			pendingToolParts = append(pendingToolParts, Part{
				FunctionResponse: &FunctionResponse{
					Name:     name,
					Response: respObj,
				},
			})
		default:
			return nil, fmt.Errorf("unsupported role: %s", m.Role)
		}
	}
	flushToolResults()

	if len(systemParts) > 0 {
		out.SystemInstruction = &Content{
			Parts: []Part{{Text: strings.Join(systemParts, "\n\n")}},
		}
	}
	if len(contents) == 0 {
		return nil, fmt.Errorf("no messages after filtering system")
	}
	out.Contents = contents

	if len(req.Tools) > 0 {
		decls := make([]FunctionDeclaration, 0, len(req.Tools))
		for _, t := range req.Tools {
			schema := t.Function.Parameters
			if len(schema) == 0 {
				schema = json.RawMessage(`{"type":"object","properties":{}}`)
			}
			decls = append(decls, FunctionDeclaration{
				Name:        t.Function.Name,
				Description: t.Function.Description,
				Parameters:  schema,
			})
		}
		out.Tools = []Tool{{FunctionDeclarations: decls}}
	}

	if len(req.ToolChoice) > 0 {
		tc, err := mapToolChoice(req.ToolChoice)
		if err != nil {
			return nil, err
		}
		out.ToolConfig = tc
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

func openAIContentToParts(raw json.RawMessage) ([]Part, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return []Part{{Text: ""}}, nil
	}
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return []Part{{Text: s}}, nil
	}
	var parts []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parts); err != nil {
		return nil, fmt.Errorf("invalid content: %w", err)
	}
	var out []Part
	for _, p := range parts {
		typ := ""
		_ = json.Unmarshal(p["type"], &typ)
		switch typ {
		case "image_url", "image":
			return nil, fmt.Errorf("image inputs not supported in v1")
		case "text", "":
			var t string
			_ = json.Unmarshal(p["text"], &t)
			out = append(out, Part{Text: t})
		default:
			return nil, fmt.Errorf("unsupported content type: %s", typ)
		}
	}
	if len(out) == 0 {
		out = []Part{{Text: ""}}
	}
	return out, nil
}

func mapToolChoice(raw json.RawMessage) (*ToolConfig, error) {
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		mode := "AUTO"
		switch s {
		case "auto":
			mode = "AUTO"
		case "required":
			mode = "ANY"
		case "none":
			mode = "NONE"
		default:
			return nil, fmt.Errorf("unsupported tool_choice: %s", s)
		}
		return &ToolConfig{FunctionCallingConfig: &FunctionCallingConfig{Mode: mode}}, nil
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
		return &ToolConfig{FunctionCallingConfig: &FunctionCallingConfig{
			Mode:                 "ANY",
			AllowedFunctionNames: []string{obj.Function.Name},
		}}, nil
	}
	return &ToolConfig{FunctionCallingConfig: &FunctionCallingConfig{Mode: "AUTO"}}, nil
}
