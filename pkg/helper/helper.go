package helper

import (
	"encoding/json"
	"net/url"
	"os"
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

// Guess content type from body or explicit header.
// TODO improve
func GuessContentType(explicitType string, body string, method string) string {

	if explicitType != "" {
		return explicitType
	}
	body = strings.TrimSpace(body)
	if body == "" {
		// No body -> only assign default if non-GET
		if strings.EqualFold(method, "GET") || strings.EqualFold(method, "HEAD") {
			return ""
		}
		return "application/x-www-form-urlencoded"
	}

	// Detect XML
	if strings.HasPrefix(body, "<?xml version=") {
		return "application/xml"
	}

	// Detect JSON
	if (strings.HasPrefix(body, "{") && strings.HasSuffix(body, "}")) ||
		(strings.HasPrefix(body, "[") && strings.HasSuffix(body, "]")) {
		if json.Valid([]byte(body)) {
			return "application/json"
		}
	}

	// Detect form-encoded
	if _, err := url.ParseQuery(body); err == nil && strings.Contains(body, "=") {
		return "application/x-www-form-urlencoded"
	}

	// Fallback default
	return "application/x-www-form-urlencoded"
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
