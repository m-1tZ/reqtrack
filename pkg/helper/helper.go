package helper

import (
	"strings"

	"github.com/m-1tZ/reqtrack/pkg/structs"
)

// parseHeaderFlag parses "-H" arguments like "Header1: Value1; Header2: Value2"
func ParseHeaderFlag(headerStr string) map[string]interface{} {
	headers := make(map[string]interface{})
	for _, h := range strings.Split(headerStr, ";") {
		h = strings.TrimSpace(h)
		if h == "" {
			continue
		}
		parts := strings.SplitN(h, ":", 2)
		if len(parts) == 2 {
			headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	return headers
}

func ParseQueryParams(rawURL string) []structs.Param {
	qIndex := strings.Index(rawURL, "?")
	if qIndex == -1 {
		return nil
	}
	queryStr := rawURL[qIndex+1:]
	params := strings.Split(queryStr, "&")
	var result []structs.Param
	for _, p := range params {
		if p == "" {
			continue
		}
		kv := strings.SplitN(p, "=", 2)
		name := kv[0]
		val := ""
		if len(kv) == 2 {
			val = kv[1]
		}
		result = append(result, structs.Param{Name: name, Value: val})
	}
	return result
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
