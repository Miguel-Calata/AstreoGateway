package anthropic

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"astreoGateway/internal/protocol/core"
)

func TranslateStream(r io.Reader, w http.ResponseWriter, model string, includeUsage bool, logger *slog.Logger) error {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, canFlush := w.(http.Flusher)
	writeChunk := func(chunk core.ChatChunk) error {
		b, err := json.Marshal(chunk)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return err
		}
		if canFlush {
			flusher.Flush()
		}
		return nil
	}
	writeDone := func() error {
		if _, err := io.WriteString(w, "data: [DONE]\n\n"); err != nil {
			return err
		}
		if canFlush {
			flusher.Flush()
		}
		return nil
	}

	msgID := "chatcmpl-unknown"
	toolIndexByBlock := map[int]int{}
	nextToolIdx := 0
	var finishReason *string
	var usage *core.Usage

	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	var dataLines []string

	flushEvent := func() error {
		if len(dataLines) == 0 {
			return nil
		}
		payload := strings.Join(dataLines, "\n")
		dataLines = nil
		if payload == "" || payload == "[DONE]" {
			return nil
		}
		var ev StreamEvent
		if err := json.Unmarshal([]byte(payload), &ev); err != nil {
			if logger != nil {
				logger.Debug("anthropic stream: skip bad event", "err", err)
			}
			return nil
		}
		switch ev.Type {
		case "message_start":
			if ev.Message != nil && ev.Message.ID != "" {
				msgID = "chatcmpl-" + ev.Message.ID
			}
			if ev.Message != nil {
				usage = &core.Usage{
					PromptTokens:     ev.Message.Usage.InputTokens,
					CompletionTokens: ev.Message.Usage.OutputTokens,
					TotalTokens:      ev.Message.Usage.InputTokens + ev.Message.Usage.OutputTokens,
				}
			}
			role := "assistant"
			return writeChunk(core.ChatChunk{
				ID:     msgID,
				Object: "chat.completion.chunk",
				Model:  model,
				Choices: []core.ChunkChoice{{
					Index: 0,
					Delta: core.ChunkDelta{Role: role},
				}},
			})
		case "content_block_start":
			if ev.ContentBlock != nil && ev.ContentBlock.Type == "tool_use" {
				idx := nextToolIdx
				toolIndexByBlock[ev.Index] = idx
				nextToolIdx++
				i := idx
				return writeChunk(core.ChatChunk{
					ID:     msgID,
					Object: "chat.completion.chunk",
					Model:  model,
					Choices: []core.ChunkChoice{{
						Index: 0,
						Delta: core.ChunkDelta{
							ToolCalls: []core.ToolCall{{
								Index: &i,
								ID:    ev.ContentBlock.ID,
								Type:  "function",
								Function: core.FunctionCall{
									Name:      ev.ContentBlock.Name,
									Arguments: "",
								},
							}},
						},
					}},
				})
			}
		case "content_block_delta":
			if ev.Delta == nil {
				return nil
			}
			switch ev.Delta.Type {
			case "text_delta":
				return writeChunk(core.ChatChunk{
					ID:     msgID,
					Object: "chat.completion.chunk",
					Model:  model,
					Choices: []core.ChunkChoice{{
						Index: 0,
						Delta: core.ChunkDelta{Content: ev.Delta.Text},
					}},
				})
			case "input_json_delta":
				idx, ok := toolIndexByBlock[ev.Index]
				if !ok {
					idx = nextToolIdx
					toolIndexByBlock[ev.Index] = idx
					nextToolIdx++
				}
				i := idx
				return writeChunk(core.ChatChunk{
					ID:     msgID,
					Object: "chat.completion.chunk",
					Model:  model,
					Choices: []core.ChunkChoice{{
						Index: 0,
						Delta: core.ChunkDelta{
							ToolCalls: []core.ToolCall{{
								Index: &i,
								Type:  "function",
								Function: core.FunctionCall{
									Arguments: ev.Delta.PartialJSON,
								},
							}},
						},
					}},
				})
			}
		case "message_delta":
			if ev.Delta != nil && ev.Delta.StopReason != "" {
				fr := mapStopReason(ev.Delta.StopReason)
				finishReason = &fr
			}
			if ev.Usage != nil {
				if usage == nil {
					usage = &core.Usage{}
				}
				usage.CompletionTokens = ev.Usage.OutputTokens
				if usage.PromptTokens == 0 {
					usage.PromptTokens = ev.Usage.InputTokens
				}
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
		case "message_stop":
			chunk := core.ChatChunk{
				ID:     msgID,
				Object: "chat.completion.chunk",
				Model:  model,
				Choices: []core.ChunkChoice{{
					Index:        0,
					Delta:        core.ChunkDelta{},
					FinishReason: finishReason,
				}},
			}
			if includeUsage && usage != nil {
				chunk.Usage = usage
			}
			if err := writeChunk(chunk); err != nil {
				return err
			}
			return writeDone()
		case "ping":
			return nil
		case "error":
			msg := "upstream stream error"
			if ev.Error != nil && ev.Error.Message != "" {
				msg = ev.Error.Message
			}
			errBody, _ := json.Marshal(map[string]any{
				"error": map[string]string{"message": msg, "type": "upstream_error"},
			})
			if _, err := fmt.Fprintf(w, "data: %s\n\n", errBody); err != nil {
				return err
			}
			if canFlush {
				flusher.Flush()
			}
			return writeDone()
		}
		return nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if err := flushEvent(); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := flushEvent(); err != nil {
		return err
	}
	return scanner.Err()
}
