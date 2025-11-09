package scrape

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/m-1tZ/reqtrack/pkg/structs"
	sitter "github.com/smacker/go-tree-sitter"
	"github.com/smacker/go-tree-sitter/javascript"
)

// // Navigate with timeout
// navCtx, navCancel := context.WithTimeout(ctx, navTimeout)
// defer navCancel()

// resp, err := chromedp.RunResponse(navCtx,
// 	chromedp.Navigate(targetURL),
// )
// if err != nil {
// 	return nil, err
// }

// // After navigation, run serializeFormsAndRequests with timeout
// scrapeCtx, scrapeCancel := context.WithTimeout(ctx, scrapingTimeout)
// defer scrapeCancel()

// var scrapeResult ScrapeResult
// if err := chromedp.Run(scrapeCtx,
// 	chromedp.Evaluate("serializeFormsAndRequests()", &scrapeResult),
// ); err != nil {
// 	return nil, err
// }

// ScrapeHtml will try to collect inline & external script sources from the current page context.
// IMPORTANT: it will first try to read document.scripts (so it will NOT navigate if the page is already loaded).
// If that yields nothing, it falls back to navigating targetURL once.
func ScrapeHtml(ctx context.Context, targetURL string, header string, wait time.Duration) ([]structs.StaticHttpPrimitive, error) {
	var scripts []string
	// try to read scripts from already-loaded page
	err := chromedp.Run(ctx,
		chromedp.Evaluate(`Array.from(document.scripts).map(s => s.src ? s.src : s.innerText);`, &scripts),
	)
	if err != nil || len(scripts) == 0 {
		// fallback: navigate once and get scripts
		if err := chromedp.Run(ctx,
			chromedp.Navigate(targetURL),
			chromedp.Sleep(wait),
			chromedp.Evaluate(`Array.from(document.scripts).map(s => s.src ? s.src : s.innerText);`, &scripts),
		); err != nil {
			return nil, err
		}
	}

	var all []structs.StaticHttpPrimitive
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
		findings, _ := findHttpPrimitives(ctx, js)
		all = append(all, findings...)
	}
	return all, nil
}

// ---- Tree-sitter static JS detection ----
func findHttpPrimitives(ctx context.Context, jsCode string) ([]structs.StaticHttpPrimitive, error) {
	parser := sitter.NewParser()
	parser.SetLanguage(javascript.GetLanguage())

	tree, err := parser.ParseCtx(ctx, nil, []byte(jsCode))
	if err != nil {
		return nil, fmt.Errorf("failed to parse JS code: %w", err)
	}

	root := tree.RootNode()
	var results []structs.StaticHttpPrimitive

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

	guessContentType := func(explicitType, body string) string {
		if explicitType != "" {
			return explicitType
		}
		if body == "" {
			return ""
		}
		trimmed := strings.TrimSpace(body)
		if strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}") {
			return "application/json"
		}
		if strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]") {
			return "application/json"
		}
		if regexp.MustCompile(`^[^=&]+=[^=&]+(&[^=&]+=[^=&]+)*$`).MatchString(trimmed) {
			return "application/x-www-form-urlencoded"
		}
		return "text/plain"
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
				funcName := funcNode.Content([]byte(jsCode))
				prim := ""
				isFetch := false
				isXHROpen := false

				switch funcNode.Type() {
				case "identifier":
					if funcName == "fetch" {
						prim = "fetch"
						isFetch = true
					}
				case "member_expression":
					obj := funcNode.ChildByFieldName("object")
					prop := funcNode.ChildByFieldName("property")
					if obj != nil && prop != nil {
						if obj.Content([]byte(jsCode)) == "axios" {
							prim = "axios." + prop.Content([]byte(jsCode))
						}
						if obj.Content([]byte(jsCode)) == "$" && prop.Content([]byte(jsCode)) == "ajax" {
							prim = "$.ajax"
						}
						if prop.Content([]byte(jsCode)) == "open" {
							prim = "XMLHttpRequest.open"
							isXHROpen = true
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

						if isXHROpen {
							if len(args) >= 2 {
								method = extractString(args[0])
								url = extractString(args[1])
							}
						}

						if strings.HasPrefix(prim, "axios.") {
							if len(args) >= 1 {
								url = extractString(args[0])
							}
							if len(args) >= 2 && args[1].Type() == "object" {
								method = strings.ToUpper(strings.TrimPrefix(prim, "axios."))
								ctype = extractObjectProperty(args[1], "Content-Type")
								body = extractObjectProperty(args[1], "data")
							}
						}

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

					// Guess content type if missing
					ctype = guessContentType(ctype, body)

					results = append(results, structs.StaticHttpPrimitive{
						Primitive:   prim,
						URL:         url,
						Method:      method,
						ContentType: ctype,
						Body:        body,
					})
				}
			}
		}

		for i := 0; i < int(node.ChildCount()); i++ {
			walk(node.Child(i))
		}
	}

	walk(root)
	return results, nil
}
