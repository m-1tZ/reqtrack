package scrape

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/m-1tZ/reqtrack/pkg/helper"
	"github.com/m-1tZ/reqtrack/pkg/structs"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

// ScrapeHtml will try to collect inline & external script sources from the current page context.
// IMPORTANT: it will first try to read document.scripts (so it will NOT navigate if the page is already loaded).
// If that yields nothing, it falls back to navigating targetURL once.
func ScrapeHtml(ctx context.Context, targetURL string, header string, timeout time.Duration) ([]*structs.RequestEntry, error) {
	var scripts []string
	// try to read scripts from already-loaded page
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`Array.from(document.scripts).map(s => s.src ? s.src : s.innerText);`, &scripts),
	)
	if err != nil || len(scripts) == 0 {
		// fallback: navigate once and get scripts
		if err := chromedp.Run(ctx,
			chromedp.Navigate(targetURL),
			chromedp.WaitReady("body", chromedp.ByQuery),
			chromedp.Evaluate(`Array.from(document.scripts).map(s => s.src ? s.src : s.innerText);`, &scripts),
			// chromedp.Sleep(5),
		); err != nil {
			return nil, err
		}
	}

	var all []*structs.RequestEntry

	for _, js := range scripts {
		if js == "" {
			continue
		}
		if strings.HasPrefix(js, "http") {
			resp, err := http.Get(js)
			if err != nil {
				continue
			}
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			js = string(body)
		}
		// Main logic
		findings, _ := findHttpPrimitives(ctx, js)
		all = append(all, findings...)
	}
	return all, nil
}

// ---- Tree-sitter static JS detection ----
func findHttpPrimitives(ctx context.Context, jsCode string) ([]*structs.RequestEntry, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())

	tree, err := parser.ParseCtx(ctx, nil, []byte(jsCode))
	if err != nil {
		return nil, fmt.Errorf("failed to parse JS code: %w", err)
	}

	root := tree.RootNode()
	var results []*structs.RequestEntry
	src := []byte(jsCode)

	// --- helpers ---
	extractString := func(node *sitter.Node) string {
		if node == nil {
			return ""
		}
		switch node.Type() {
		case "string":
			return strings.Trim(node.Content([]byte(jsCode)), `"'`)
		case "template_string":
			return strings.Trim(node.Content([]byte(jsCode)), "`")
		case "object":
			// Raw object literal in place of string
			return node.Content([]byte(jsCode))
		}
		return ""
	}

	var extractObjectProperty func(objNode *sitter.Node, propName string) string
	extractObjectProperty = func(objNode *sitter.Node, propName string) string {
		if objNode == nil || (objNode.Type() != "object" && objNode.Type() != "object_pattern") {
			return ""
		}
		for i := 0; i < int(objNode.NamedChildCount()); i++ {
			child := objNode.NamedChild(i)
			if child.Type() == "pair" {
				keyNode := child.ChildByFieldName("key")
				valueNode := child.ChildByFieldName("value")
				if keyNode != nil && strings.Trim(keyNode.Content([]byte(jsCode)), `"'`) == propName {
					return extractString(valueNode)
				}
				// Special: headers.{Content-Type}
				if keyNode != nil && strings.Trim(keyNode.Content([]byte(jsCode)), `"'`) == "headers" && valueNode.Type() == "object" {
					return extractObjectProperty(valueNode, propName)
				}
			}
		}
		return ""
	}

	// Guess content type from body or explicit header.
	guessContentType := func(explicitType, body, method string) string {
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

	// --- AST walker ---
	var walk func(node *sitter.Node)
	walk = func(node *sitter.Node) {
		if node == nil {
			return
		}

		if node.Type() == "call_expression" {
			funcNode := node.ChildByFieldName("function")
			if funcNode != nil {
				funcName := funcNode.Content(src)
				prim := ""
				isFetch := false
				isXHROpen := false
				isXHRSend := false

				switch funcNode.Type() {
				case "identifier":
					if funcName == "fetch" {
						prim = "fetch"
						isFetch = true
					}
					if funcName == "axios" {
						prim = "axios"
					}
				case "member_expression":
					obj := funcNode.ChildByFieldName("object")
					prop := funcNode.ChildByFieldName("property")
					if obj != nil && prop != nil {
						// axios.<method>()
						if obj.Content(src) == "axios" {
							prim = "axios." + prop.Content(src)
						}
						// $.ajax(...)
						if obj.Content(src) == "$" && prop.Content(src) == "ajax" {
							prim = "$.ajax"
						}
						// XMLHttpRequest.open/send(...)
						if prop.Content(src) == "open" {
							prim = "XMLHttpRequest.open"
							isXHROpen = true
						}
						if prop.Content(src) == "send" {
							prim = "XMLHttpRequest.send"
							isXHRSend = true
						}
					}
				}

				if prim != "" {
					argsNode := node.ChildByFieldName("arguments")
					var url, method, ctype, body string

					if argsNode != nil {
						args := []*sitter.Node{}
						for i := 0; i < int(argsNode.NamedChildCount()); i++ {
							args = append(args, argsNode.NamedChild(i))
						}

						// --- fetch() ---
						if isFetch {
							if len(args) >= 1 {
								url = extractString(args[0])
							}
							if len(args) >= 2 && args[1].Type() == "object" {
								method = extractObjectProperty(args[1], "method")
								ctype = extractObjectProperty(args[1], "Content-Type")
								body = extractObjectProperty(args[1], "body")
							}
						}

						// --- XMLHttpRequest.open(method, url) ---
						if isXHROpen {
							if len(args) >= 2 {
								method = extractString(args[0])
								url = extractString(args[1])
							}
						}

						// --- XMLHttpRequest.send(body) ---
						if isXHRSend && len(args) >= 1 {
							body = extractString(args[0])
						}

						// --- axios.post(...) / axios.get(...) etc. ---
						if strings.HasPrefix(prim, "axios.") {
							if len(args) >= 1 {
								url = extractString(args[0])
							}
							if len(args) >= 2 {
								method = strings.ToUpper(strings.TrimPrefix(prim, "axios."))

								// Second argument is usually the body
								if args[1].Type() == "object" || args[1].Type() == "array" {
									body = args[1].Content(src)
								}

								// Third argument may be config
								if len(args) >= 3 && args[2].Type() == "object" {
									ctype = extractObjectProperty(args[2], "Content-Type")
								}
							}
						}

						// --- axios({ url, method, data, headers }) ---
						if prim == "axios" {
							if len(args) >= 1 && args[0].Type() == "object" {
								url = extractObjectProperty(args[0], "url")
								method = extractObjectProperty(args[0], "method")
								ctype = extractObjectProperty(args[0], "Content-Type")
								body = extractObjectProperty(args[0], "data")
							}
							if len(args) >= 2 && args[1].Type() == "object" {
								// axios(url, config)
								url = extractString(args[0])
								method = extractObjectProperty(args[1], "method")
								ctype = extractObjectProperty(args[1], "Content-Type")
								body = extractObjectProperty(args[1], "data")
							}
						}

						// --- $.ajax({...}) ---
						if prim == "$.ajax" && len(args) >= 1 && args[0].Type() == "object" {
							url = extractObjectProperty(args[0], "url")
							method = extractObjectProperty(args[0], "method")
							if method == "" {
								method = extractObjectProperty(args[0], "type")
							}
							ctype = extractObjectProperty(args[0], "Content-Type")
							body = extractObjectProperty(args[0], "data")
						}
					}

					// Normalize method
					method = strings.ToUpper(strings.TrimSpace(method))
					if method == "" {
						method = "GET"
					}

					// Guess content type if missing
					ctype = guessContentType(ctype, body, method)

					req := &structs.RequestEntry{
						URL:         url,
						Method:      method,
						ContentType: ctype,
						Headers:     map[string]string{},
					}

					// Fill query params if any
					if strings.Contains(url, "?") {
						req.QueryParams = helper.ParseQueryParams(url)
					}

					// Fill post data entries if available
					if body != "" {
						req.PostDataEntries = []*structs.PostDataEntryExtended{
							{
								Bytes:       fmt.Sprintf("%d", len(body)),
								DecodedText: body,
							},
						}
					}

					results = append(results, req)
				}
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)

	// --- Deduplicate identical requests and remove empty URL ---
	type reqKey struct {
		Method string
		URL    string
		Body   string
	}
	seen := make(map[reqKey]bool)
	var deduped []*structs.RequestEntry
	for _, r := range results {
		if r.URL == "" {
			continue
		}

		key := reqKey{
			Method: r.Method,
			URL:    r.URL,
		}
		if len(r.PostDataEntries) > 0 {
			key.Body = r.PostDataEntries[0].DecodedText
		}
		if !seen[key] {
			seen[key] = true
			deduped = append(deduped, r)
		}
	}

	return deduped, nil
}
