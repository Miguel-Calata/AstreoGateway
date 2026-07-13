package anthropic

import (
	"net/url"
	"path"
	"strings"
)

func BuildMessagesURL(base string) string {
	if base == "" {
		return "/messages"
	}
	u, err := url.Parse(base)
	if err != nil {
		return strings.TrimRight(base, "/") + "/messages"
	}
	u.Path = path.Join(u.Path, "messages")
	return u.String()
}
