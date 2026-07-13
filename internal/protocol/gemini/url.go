package gemini

import (
	"net/url"
	"path"
	"strings"
)

// buildGeminiURL builds a Gemini API URL from a base URL and a path segment.
// It automatically detects if the base already includes /v1beta and avoids duplication.
func buildGeminiURL(base, segment string) string {
	if base == "" {
		return "/v1beta/" + segment
	}
	u, err := url.Parse(base)
	if err != nil {
		trimmed := strings.TrimRight(base, "/")
		if strings.HasSuffix(trimmed, "/v1beta") || trimmed == "v1beta" {
			return trimmed + "/" + segment
		}
		return trimmed + "/v1beta/" + segment
	}
	u.Path = strings.TrimSuffix(u.Path, "/")
	if strings.HasSuffix(u.Path, "/v1beta") || u.Path == "v1beta" {
		u.Path = path.Join(u.Path, segment)
	} else {
		u.Path = path.Join(u.Path, "v1beta", segment)
	}
	return u.String()
}

func BuildModelsURL(base string) string {
	return buildGeminiURL(base, "models")
}

func BuildGenerateContentURL(base, model string) string {
	return buildGeminiURL(base, "models/"+model+":generateContent")
}

func BuildStreamGenerateContentURL(base, model string) string {
	baseURL := buildGeminiURL(base, "models/"+model+":streamGenerateContent")
	return baseURL + "?alt=sse"
}