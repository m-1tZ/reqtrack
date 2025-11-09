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
