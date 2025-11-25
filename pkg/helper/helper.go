package helper

import (
	"encoding/json"
	"net/url"
	"os"
	"regexp"
	"strings"

	"github.com/m-1tZ/reqtrack/pkg/structs"
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
	// for {
	start := strings.Index(u, "${")
	if start != -1 {
		return ""
	}
	// end := strings.Index(u[start:], "}")
	// if end == -1 {
	// 	break
	// }
	// u = u[:start] + u[start+end+1:]
	// }

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

	//  sanitized version is not returned
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

// Guess content type using static analysis + heuristics.
func GuessContentType(explicitType string, body string, method string) string {
	// --- 1) Explicit Content-Type always wins ---
	if explicitType != "" {
		return explicitType
	}

	// --- 2) No body: assign default only for non-GET/HEAD ---
	body = strings.TrimSpace(body)
	if body == "" {
		if strings.EqualFold(method, "GET") || strings.EqualFold(method, "HEAD") {
			return ""
		}
		return "application/x-www-form-urlencoded"
	}

	// --- 3) Detect JSON from common JS patterns ---

	// JSON.stringify(...)
	if strings.Contains(body, "JSON.stringify") {
		return "application/json"
	}

	// JS object/array literal (static)
	if (strings.HasPrefix(body, "{") && strings.Contains(body, ":")) ||
		(strings.HasPrefix(body, "[") && strings.HasSuffix(body, "]")) {
		return "application/json"
	}

	// Strict literal JSON
	if json.Valid([]byte(body)) {
		return "application/json"
	}

	// --- 4) Detect FormData() ---
	if strings.Contains(body, "FormData(") {
		return "multipart/form-data"
	}

	// --- 5) Detect URLSearchParams() ---
	if strings.Contains(body, "URLSearchParams(") {
		return "application/x-www-form-urlencoded"
	}

	// --- 6) Detect Blob/File types ---
	if strings.Contains(body, "new Blob(") || strings.Contains(body, "new File(") {
		// Try to extract explicit type: { type: "..." }
		if idx := strings.Index(body, "type"); idx != -1 {
			if colon := strings.Index(body[idx:], ":"); colon != -1 {
				rest := body[idx+colon+1:]
				t := extractFirstQuotedString(rest)
				if t != "" {
					return t
				}
			}
		}
		return "application/octet-stream"
	}

	// --- 7) Detect XML content ---
	if strings.HasPrefix(body, "<?xml") {
		return "application/xml"
	}
	if looksLikeXML(body) {
		return "application/xml"
	}

	// --- 8) Detect base64 → binary ---
	if isLikelyBase64(body) {
		return "application/octet-stream"
	}

	// --- 9) Detect form-urlencoded ---
	if isLikelyFormURLEncoded(body) {
		return "application/x-www-form-urlencoded"
	}

	// --- 10) Fallback ---
	return "application/x-www-form-urlencoded"
}

// Detect very likely XML but not HTML.
func looksLikeXML(s string) bool {
	if !strings.HasPrefix(s, "<") {
		return false
	}
	if strings.HasPrefix(s, "<html") || strings.HasPrefix(s, "<!DOCTYPE html") {
		return false
	}
	return strings.Contains(s, "</")
}

// Extract first quoted string from snippet: "value"
func extractFirstQuotedString(src string) string {
	for _, q := range []string{`"`, `'`} {
		start := strings.Index(src, q)
		if start == -1 {
			continue
		}
		end := strings.Index(src[start+1:], q)
		if end == -1 {
			continue
		}
		return src[start+1 : start+1+end]
	}
	return ""
}

// Base64 heuristic (static)
func isLikelyBase64(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 20 {
		return false
	}
	if !regexp.MustCompile(`^[A-Za-z0-9+/=]+$`).MatchString(s) {
		return false
	}
	// Ends with "=" padding
	return strings.HasSuffix(s, "=") || strings.HasSuffix(s, "==")
}

// Form-URL-Encoded heuristic
func isLikelyFormURLEncoded(s string) bool {
	if !strings.Contains(s, "=") {
		return false
	}
	if strings.Contains(s, "{") || strings.Contains(s, "[") {
		return false
	}
	parts := strings.Split(s, "&")
	for _, p := range parts {
		if !strings.Contains(p, "=") {
			return false
		}
	}
	return true
}

// Merge scraped HAR entries with loaded HAR entries.
// Scraped entries come without response objects, but that’s fine – we only keep Request anyway.
func MergeHAREntries(base []*structs.HAREntry, scraped []*structs.HAREntry) []*structs.HAREntry {
	// Just append – dedupe happens later
	merged := make([]*structs.HAREntry, 0, len(base)+len(scraped))
	merged = append(merged, base...)
	merged = append(merged, scraped...)
	return merged
}

// DeduplicateEntries removes duplicate HAR entries based on request key
func DeduplicateHAREntries(
	results []*structs.HAREntry,
	targetURL string,
) ([]*structs.HAREntry, error) {

	// --- Deduplicate identical requests and remove empty URL ---
	type RequestKey struct {
		Method      string
		URL         string
		Body        string
		ContentType string
	}

	parsedBase, err := url.Parse(targetURL)
	if err != nil {
		return nil, err
	}
	baseOrigin := parsedBase.Scheme + "://" + parsedBase.Host

	seen := make(map[RequestKey]bool)
	var deduped []*structs.HAREntry

	for _, entry := range results {
		req := &entry.Request
		if req.URL == "" {
			continue
		}

		// Sanitize and normalize URLs (if variable ${} and then /<path>, just remove and pretend to target the root of the current origin) such as
		// "url": "${f.getAuthServiceUrl()}/userinfo?origin=client",
		// "url": "${(0,Vl.pN)(\"/_/api/commerce/prod\")}/shop/init-transaction/datatrans",
		// "url": "/assets/files/lawyers/Anwaltsnetz_Tabelle.json",
		// "url": "http://localhost:3000/external-executed",
		// We always want absolute URLs, not relativ - thus get the targetURL and take the scheme + host and append relative
		normalizedURL := SanitizeURL(req.URL, baseOrigin)
		body := ""
		if req.PostData != nil && req.PostData.Text != "" {
			body = req.PostData.Text
		}

		contentType := ""
		// extractContentType returns the POST Content-Type in this priority order:
		// 1. HARRequest.PostData.MimeType
		// 2. Header: Content-Type
		if req.PostData != nil && req.PostData.MimeType != "" {
			contentType = req.PostData.MimeType
		} else {
			for _, h := range req.Headers {
				if strings.EqualFold(h.Name, "Content-Type") {
					contentType = h.Value
				}
			}
		}

		key := RequestKey{
			Method:      req.Method,
			URL:         normalizedURL,
			Body:        body,
			ContentType: contentType,
		}

		// Deduplicate
		if !seen[key] {
			seen[key] = true

			// Mutate HAR entry to sanitized URL
			req.URL = normalizedURL

			deduped = append(deduped, entry)
		}
	}

	return deduped, nil
}

func LoadHAREntriesStreaming(path string) ([]*structs.HAREntry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	dec := json.NewDecoder(f)

	var entries []*structs.HAREntry

	// Navigate to "entries" token
	for {
		t, err := dec.Token()
		if err != nil {
			return nil, err
		}
		if key, ok := t.(string); ok && key == "entries" {
			break
		}
	}

	// Read the '['
	_, err = dec.Token()
	if err != nil {
		return nil, err
	}

	// Stream each entry
	for dec.More() {
		var e structs.HAREntry
		if err := dec.Decode(&e); err != nil {
			return nil, err
		}
		entries = append(entries, &e)
	}

	return entries, nil
}

func WriteHAR(path string, entries []*structs.HAREntry) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.Write([]byte(`{"log":{"entries":[`))
	if err != nil {
		return err
	}

	enc := json.NewEncoder(out)
	first := true

	for _, e := range entries {
		if !first {
			out.Write([]byte(","))
		}
		first = false

		if err := enc.Encode(e); err != nil {
			return err
		}
	}

	_, err = out.Write([]byte(`]}}`))
	return err
}
