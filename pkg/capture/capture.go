package capture

import (
	"context"
	"encoding/base64"
	"fmt"
	urlpkg "net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/m-1tZ/reqtrack/pkg/helper"
	"github.com/m-1tZ/reqtrack/pkg/structs"
)

// CaptureRequests navigates to targetURL, triggers some JS heuristics and captures runtime network events.
func CaptureRequests(ctx context.Context, targetURL string, header string, timeout time.Duration) ([]*structs.RequestEntry, error) {
	var requests []*structs.RequestEntry

	headers := helper.ParseHeaderFlag(header)

	// Enable network
	// TODO implement RunResponse with navigation timeout
	if err := chromedp.Run(ctx, network.Enable(), network.SetExtraHTTPHeaders(network.Headers(headers))); err != nil {
		return nil, err
	}

	// Listen for network events
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch e := ev.(type) {
		case *network.EventRequestWillBeSent:
			req := &structs.RequestEntry{
				URL:     e.Request.URL,
				Method:  e.Request.Method,
				Headers: make(map[string]string),
			}

			// Headers + detect Content-Type
			for k, v := range e.Request.Headers {
				req.Headers[k] = fmt.Sprintf("%v", v)
				if strings.ToLower(k) == "content-type" {
					req.ContentType = fmt.Sprintf("%v", v)
				}
			}

			// Query params
			if u, err := urlpkg.Parse(e.Request.URL); err == nil { // alias net/url as urlpkg
				q := u.Query()
				for name, values := range q {
					for _, v := range values {
						req.QueryParams = append(req.QueryParams, structs.Param{
							Name:  name,
							Value: v,
						})
					}
				}
			}

			// Post data
			if e.Request.PostDataEntries != nil {
				for _, entry := range e.Request.PostDataEntries {
					decoded := ""
					if entry.Bytes != "" {
						if b, err := base64.StdEncoding.DecodeString(entry.Bytes); err == nil {
							decoded = string(b)
						}
					}
					req.PostDataEntries = append(req.PostDataEntries, &structs.PostDataEntryExtended{
						Bytes:       entry.Bytes,
						DecodedText: decoded,
					})

					// Parse form-urlencoded into QueryParams
					// TODO parse other CT as well
					if strings.Contains(strings.ToLower(req.ContentType), "application/x-www-form-urlencoded") {
						if vals, err := urlpkg.ParseQuery(decoded); err == nil {
							for name, values := range vals {
								for _, v := range values {
									req.QueryParams = append(req.QueryParams, structs.Param{
										Name:  name,
										Value: v,
									})
								}
							}
						}
					}
				}
			}
			requests = append(requests, req)

		case *network.EventResponseReceived:
			for _, r := range requests {
				if r.URL == e.Response.URL && r.Response == nil {
					r.Response = &structs.ResponseInfo{
						Status:     int(e.Response.Status),
						StatusText: e.Response.StatusText,
						Headers:    make(map[string]string),
						MIMEType:   e.Response.MimeType,
					}
					for k, v := range e.Response.Headers {
						r.Response.Headers[k] = fmt.Sprintf("%v", v)
					}
					break
				}
			}
		}
	})

	// JS to trigger likely network-calling functions (best-effort)
	// TODO .test here?
	triggerJS := `(function() {
		const httpPattern = /\b(fetch|XMLHttpRequest|axios|\.ajax|sendBeacon|WebSocket|EventSource)\b/;

		// Scan all global properties for functions
		function collectNetworkFunctions(root) {
			const results = [];
			const seen = new WeakSet();

			for (const key of Object.getOwnPropertyNames(root)) {
				try {
					const val = root[key];
					if (typeof val === "function" && !seen.has(val)) {
						seen.add(val);

						// Cheap pre-check: function name
						if (!httpPattern.test(val.name)) {
							// Expensive: fallback to toString only if necessary
							const src = Function.prototype.toString.call(val);
							if (httpPattern.test(src) && !src.includes("[native code]")) {
								results.push({ name: key, fn: val });
							}
						} else {
							results.push({ name: key, fn: val });
						}
					}
				} catch (e) {
					// Skip non-readable properties
				}
			}
			return results;
		}

		// Collect once
		const candidates = collectNetworkFunctions(window);

		console.log("Found HTTP-related functions:", candidates.map(c => c.name));

		// Try triggering them
		for (const { name, fn } of candidates) {
			try {
				if (fn.length === 0) {
					console.log("Triggering:", name);
					fn();
				}
			} catch (e) {
				console.warn("Error executing", name, e);
			}
		}

		// Also trigger forms (optional)
		for (const form of document.forms) {
			try { form.submit(); } catch(e) {}
		}
	})();
	`

	// navigate + evaluate + wait
	// TODO RunResponse with navigation timeout
	if err := chromedp.Run(ctx,
		chromedp.Navigate(targetURL),
		chromedp.Evaluate(triggerJS, nil),
		//chromedp.Sleep(wait), # TODO we do we need this?
	); err != nil {
		return nil, err
	}

	// Deduplicate exact requests
	type reqKey struct {
		Method string
		URL    string
		Body   string
	}
	seen := make(map[reqKey]bool)
	var deduped []*structs.RequestEntry
	for _, r := range requests {
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
