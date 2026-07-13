package openai

import (
	"net/url"
	"path"
	"strings"
)

func BuildChatCompletionsURL(base string) string {
	return buildOpenAIURL(base, "chat/completions")
}

func BuildEmbeddingsURL(base string) string {
	return buildOpenAIURL(base, "embeddings")
}

func buildOpenAIURL(base, segment string) string {
	if base == "" {
		return "/" + segment
	}
	u, err := url.Parse(base)
	if err != nil {
		return strings.TrimRight(base, "/") + "/" + segment
	}
	u.Path = path.Join(u.Path, segment)
	return u.String()
}
