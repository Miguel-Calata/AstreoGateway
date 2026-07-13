package registry

import (
	"astreoGateway/internal/protocol"
	"astreoGateway/internal/protocol/anthropic"
	"astreoGateway/internal/protocol/gemini"
	"astreoGateway/internal/protocol/openai"
)

func init() {
	protocol.Register(openai.New())
	protocol.Register(anthropic.New())
	protocol.Register(gemini.New())
}
