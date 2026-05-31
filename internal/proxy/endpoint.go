package proxy

import (
	"net/url"
	"regexp"
	"strings"
)

var versionSegmentRE = regexp.MustCompile(`^v\d+$`)

func baseHasVersionSegment(baseURL string) bool {
	trimmed := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if trimmed == "" {
		return false
	}
	path := trimmed
	if u, err := url.Parse(trimmed); err == nil && u.Path != "" {
		path = u.Path
	}
	path = strings.TrimRight(path, "/")
	if path == "" {
		return false
	}
	segs := strings.Split(path, "/")
	last := segs[len(segs)-1]
	return versionSegmentRE.MatchString(strings.ToLower(last))
}

func joinPath(baseURL, rel string) string {
	base := strings.TrimRight(strings.TrimSpace(baseURL), "/")
	rel = "/" + strings.TrimLeft(rel, "/")
	if baseHasVersionSegment(base) {
		return base + rel
	}
	return base + "/v1" + rel
}

func chatCompletionsURL(baseURL string) string { return joinPath(baseURL, "/chat/completions") }
func messagesURL(baseURL string) string        { return joinPath(baseURL, "/messages") }
func modelsURL(baseURL string) string          { return joinPath(baseURL, "/models") }
