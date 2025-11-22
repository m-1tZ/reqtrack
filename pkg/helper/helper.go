package helper

import (
	"strings"
)

func ParseHeaderFlag(h string) (string, string) {
	for i := 0; i < len(h); i++ {
		if h[i] == ':' {
			return h[:i], h[i+1:]
		}
	}
	return h, ""
}

func SanitizeURL(raw, baseOrigin string) string {
	// sanitizeURL removes ${...} template expressions and normalizes the URL.
	// baseOrigin should be like: "https://example.com"
	if raw == "" {
		return ""
	}

	u := raw

	// --- remove ${...} template expressions entirely ---
	for {
		start := strings.Index(u, "${")
		if start == -1 {
			break
		}
		end := strings.Index(u[start:], "}")
		if end == -1 {
			break
		}
		u = u[:start] + u[start+end+1:]
	}

	u = strings.TrimSpace(u)

	// --- whitelist characters ---
	// allowed: A–Z a–z 0–9 / - _ . : = ? & % #
	cleaned := make([]rune, 0, len(u))
	for _, r := range u {
		if (r >= 'a' && r <= 'z') ||
			(r >= 'A' && r <= 'Z') ||
			(r >= '0' && r <= '9') ||
			strings.ContainsRune("/-_.:=?&%#", r) {
			cleaned = append(cleaned, r)
		}
		// all others dropped
	}
	u = string(cleaned)

	// TODO sanitized version is not returned
	//fmt.Println(u)

	if u == "" {
		return ""
	}

	// --- protocol-relative URLs //foo.com/path ---
	if strings.HasPrefix(u, "//") {
		return "https:" + u
	}

	// --- already absolute ---
	if strings.HasPrefix(u, "http://") || strings.HasPrefix(u, "https://") {
		return u
	}

	// --- relative absolute (/path) ---
	if strings.HasPrefix(u, "/") {
		return baseOrigin + u
	}

	// --- relative without leading slash (path/foo) ---
	return baseOrigin + "/" + u
}
