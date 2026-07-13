package gemini

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

	msgID := "chatcmpl-gemini"
	roleSent := false
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
		var chunk GenerateContentResponse
		if err := json.Unmarshal([]byte(payload), &chunk); err != nil {
			if logger != nil {
				logger.Debug("gemini stream: skip bad chunk", "err", err)
			}
			return nil
		}

		if !roleSent {
			roleSent = true
			if err := writeChunk(core.ChatChunk{
				ID:     msgID,
				Object: "chat.completion.chunk",
				Model:  model,
				Choices: []core.ChunkChoice{{
					Index: 0,
					Delta: core.ChunkDelta{Role: "assistant"},
				}},
			}); err != nil {
				return err
			}
		}

		if chunk.UsageMetadata != nil {
			usage = &core.Usage{
				PromptTokens:     chunk.UsageMetadata.PromptTokenCount,
				CompletionTokens: chunk.UsageMetadata.CandidatesTokenCount,
				TotalTokens:      chunk.UsageMetadata.TotalTokenCount,
			}
			if usage.TotalTokens == 0 {
				usage.TotalTokens = usage.PromptTokens + usage.CompletionTokens
			}
		}

		if len(chunk.Candidates) == 0 {
			return nil
		}
		c := chunk.Candidates[0]
		if c.FinishReason != "" {
			fr := mapFinishReason(c.FinishReason)
			if c.Content != nil {
				for _, p := range c.Content.Parts {
					if p.FunctionCall != nil {
						fr = "tool_calls"
						break
					}
				}
			}
			finishReason = &fr
		}

		if c.Content == nil {
			return nil
		}

		for _, p := range c.Content.Parts {
			if p.Text != "" {
				if err := writeChunk(core.ChatChunk{
					ID:     msgID,
					Object: "chat.completion.chunk",
					Model:  model,
					Choices: []core.ChunkChoice{{
						Index: 0,
						Delta: core.ChunkDelta{Content: p.Text},
					}},
				}); err != nil {
					return err
				}
			}
			if p.FunctionCall != nil {
				idx := nextToolIdx
				nextToolIdx++
				i := idx
				args := ""
				if len(p.FunctionCall.Args) > 0 {
					args = string(p.FunctionCall.Args)
				}
				if err := writeChunk(core.ChatChunk{
					ID:     msgID,
					Object: "chat.completion.chunk",
					Model:  model,
					Choices: []core.ChunkChoice{{
						Index: 0,
						Delta: core.ChunkDelta{
							ToolCalls: []core.ToolCall{{
								Index: &i,
								ID:    fmt.Sprintf("call_%d", idx),
								Type:  "function",
								Function: core.FunctionCall{
									Name:      p.FunctionCall.Name,
									Arguments: args,
								},
							}},
						},
					}},
				}); err != nil {
					return err
				}
			}
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
	if err := scanner.Err(); err != nil {
		return err
	}

	final := core.ChatChunk{
		ID:     msgID,
		Object: "chat.completion.chunk",
		Model:  model,
		Choices: []core.ChunkChoice{{
			Index:        0,
			Delta:        core.ChunkDelta{},
			FinishReason: finishReason,
		}},
	}
	if finishReason == nil {
		fr := "stop"
		final.Choices[0].FinishReason = &fr
	}
	if includeUsage && usage != nil {
		final.Usage = usage
	}
	if err := writeChunk(final); err != nil {
		return err
	}
	return writeDone()
}
