package openai

import (
	"astreoGateway/internal/protocol/core"
)

// Re-export core types for backward compatibility.
type ChatRequest = core.ChatRequest
type StreamOptions = core.StreamOptions
type ChatMessage = core.ChatMessage
type Tool = core.Tool
type ToolFunction = core.ToolFunction
type ToolCall = core.ToolCall
type FunctionCall = core.FunctionCall
type ChatResponse = core.ChatResponse
type Choice = core.Choice
type ChatChunk = core.ChatChunk
type ChunkChoice = core.ChunkChoice
type ChunkDelta = core.ChunkDelta
type Usage = core.Usage
