package scrape

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/m-1tZ/reqtrack/pkg/helper"
	"github.com/m-1tZ/reqtrack/pkg/structs"
	pw "github.com/playwright-community/playwright-go"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

// ScrapeHtml will try to collect inline & external script sources from the current page context.
// IMPORTANT: it will first try to read document.scripts (so it will NOT navigate if the page is already loaded).
// If that yields nothing, it falls back to navigating targetURL once.
func ScrapeRequests(
	page pw.Page,
	browserCtx pw.BrowserContext,
	targetURL string,
	navTimeout float64,
	parseTimeout float64,
) ([]*structs.HAREntry, error) {

	// ---------------------------------------------------------
	// 1) Navigate (Playwright handles timeout)
	// ---------------------------------------------------------
	_, err := page.Goto(targetURL, pw.PageGotoOptions{
		WaitUntil: pw.WaitUntilStateNetworkidle,
	})
	if err != nil {
		return nil, fmt.Errorf("goto failed: %w", err)
	}

	// Wait until DOM is loaded
	// if err := page.WaitForLoadState(pw.LoadStateDOMContentLoaded); err != nil {
	// 	return nil, err
	// }

	// ---------------------------------------------------------
	// 2) Extract <script> contents & URLs from DOM
	// ---------------------------------------------------------
	rawJSON, err := page.Evaluate(`() => JSON.stringify(Array.from(document.scripts).map(s => s.src || s.textContent || s.innerHTML))`)
	if err != nil {
		return nil, err
	}

	jsonStr, ok := rawJSON.(string)
	if !ok {
		return nil, fmt.Errorf("expected JSON string, got %T", rawJSON)
	}

	// contains https://localhost/test.js and raw content like: const form = document.getElementById(\...
	var scriptList []string
	if err := json.Unmarshal([]byte(jsonStr), &scriptList); err != nil {
		return nil, err
	}
	// ---------------------------------------------------------
	// 3) Fetch external JS via Playwright (keeps proxy/headers/cookies)
	// ---------------------------------------------------------
	request := browserCtx.Request()

	combinedScripts := make([]string, 0, len(scriptList))

	for _, item := range scriptList {
		if strings.TrimSpace(item) == "" {
			continue
		}

		if strings.HasPrefix(item, "http://") || strings.HasPrefix(item, "https://") {
			// Fetch through Playwright's network stack
			resp, err := request.Get(item, pw.APIRequestContextGetOptions{
				Timeout:           pw.Float(navTimeout),
				IgnoreHttpsErrors: pw.Bool(true),
			})

			if err != nil {
				log.Printf("Failed to fetch external JS %s: %v", item, err)
				continue
			}
			txt, err := resp.Text()
			if err != nil {
				continue
			}
			combinedScripts = append(combinedScripts, txt)
		} else {
			// Inline JS
			combinedScripts = append(combinedScripts, item)
		}
	}

	// ---------------------------------------------------------
	// 4) Run tree-sitter static JS detection on each JS script
	// ---------------------------------------------------------
	var all []*structs.HAREntry
	for _, js := range combinedScripts {
		if js == "" {
			continue
		}

		// --- Skip JS > 4 MB ---
		if len(js) > 4*1024*1024 {
			log.Printf("skipping JS >4MB (%d bytes)", len(js))
			continue
		}

		findings, err := findHttpPrimitives(context.Background(), js, parseTimeout)
		if err != nil {
			log.Printf("tree-sitter error: %v", err)
			continue
		}
		all = append(all, findings...)
	}

	// ---------------------------------------------------------
	// 5) Normalize & dedupe just like your original code
	// ---------------------------------------------------------
	// Normalize HAR entries
	results, err := helper.DeduplicateHAREntries(all, targetURL)
	if err != nil {
		return nil, fmt.Errorf("Error in DeduplicateHAREntries: %w", err)
	}

	return results, nil
}

// ---- Tree-sitter static JS detection ----
func findHttpPrimitives(parentCtx context.Context, jsCode string, parseTimeout float64) ([]*structs.HAREntry, error) {
	// --- apply timeout ---
	ctx, cancel := context.WithTimeout(parentCtx, time.Duration(parseTimeout)*time.Second)
	defer cancel()

	// Channel for results
	type result struct {
		val []*structs.HAREntry
		err error
	}
	ch := make(chan result, 1)

	// --- worker routine ---
	go func() {

		parser := sitter.NewParser()
		parser.SetLanguage(javascript.GetLanguage())

		tree, err := parser.ParseCtx(ctx, nil, []byte(jsCode))
		if err != nil {
			ch <- result{nil, fmt.Errorf("failed to parse JS code: %w", err)}
		}

		root := tree.RootNode()
		var results []*structs.HAREntry
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

		// --- AST walker ---
		var walk func(node *sitter.Node)
		walk = func(node *sitter.Node) {
			// abort if context expired
			select {
			case <-ctx.Done():
				return
			default:
			}

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

							if body == "" && len(args) >= 2 {
								bodyNode := args[1]

								switch bodyNode.Type() {

								case "object":
									ctype = "application/json"

								case "array":
									ctype = "application/json"

								case "call_expression":
									fn := bodyNode.ChildByFieldName("function")
									if fn != nil {
										fname := fn.Content(src)

										if fname == "JSON.stringify" {
											ctype = "application/json"
										}
										if fname == "FormData" {
											ctype = "multipart/form-data"
										}
										if fname == "URLSearchParams" {
											ctype = "application/x-www-form-urlencoded"
										}
										if fname == "atob" {
											ctype = "application/octet-stream"
										}
									}

								case "new_expression":
									ctor := bodyNode.ChildByFieldName("constructor")
									if ctor != nil {
										cname := ctor.Content(src)
										if cname == "FormData" {
											ctype = "multipart/form-data"
										}
										if cname == "Blob" || cname == "File" {
											ctype = "application/octet-stream"
										}
									}
								}
							}
						}

						// Normalize method
						method = strings.ToUpper(strings.TrimSpace(method))
						if method == "" {
							method = "GET"
						}

						entry := &structs.HAREntry{
							Request: structs.HARRequest{
								Method:      method,
								URL:         url,
								HTTPVersion: "HTTP/1.1",
								Cookies:     []structs.HARCookie{},
								Headers:     []structs.HARNameValue{},
								Query:       []structs.HARNameValue{},
								PostData:    nil,
								HeaderSize:  -1,
								BodySize:    -1,
							},
						}

						// Guess content type if missing
						ctype = helper.GuessContentType(ctype, body, method)

						// if ctype != "" {
						// 	entry.Request.Headers = append(entry.Request.Headers, structs.HARNameValue{
						// 		Name:  "Content-Type",
						// 		Value: ctype,
						// 	})
						// }

						// Fill query params if any
						if strings.Contains(url, "?") {
							qp := helper.ParseQueryParams(url)
							for _, v := range qp {
								entry.Request.Query = append(entry.Request.Query, structs.HARNameValue{
									Name:  v.Name,
									Value: v.Value,
								})
							}
						}

						// Fill post data entries if available
						if body != "" {
							entry.Request.PostData = &structs.HARPostData{
								MimeType: ctype,
								Text:     body,
							}
							entry.Request.BodySize = len(body)
						}

						results = append(results, entry)
					}
				}
			}

			for i := 0; i < int(node.ChildCount()); i++ {
				walk(node.Child(i))
			}
		}

		walk(root)
		ch <- result{results, nil}
	}()

	// --- wait for worker ---
	select {
	case <-ctx.Done():
		return nil, fmt.Errorf("JS parse timed out")
	case res := <-ch:
		return res.val, res.err
	}
	//return results, nil
}
