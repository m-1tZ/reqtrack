package helper

import "strings"

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
