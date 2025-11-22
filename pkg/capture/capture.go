package capture

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/url"
	urlpkg "net/url"
	"strings"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/m-1tZ/reqtrack/pkg/helper"
	"github.com/m-1tZ/reqtrack/pkg/structs"
)

// CaptureRequests navigates to targetURL, triggers some JS heuristics and captures runtime network events.
func CaptureRequests(ctx context.Context) ([]*structs.RequestEntry, error) {
	var requests []*structs.RequestEntry

	targetURL := ctx.Value("targetURL").(string)
	parsedBase, _ := url.Parse(targetURL)
	baseOrigin := parsedBase.Scheme + "://" + parsedBase.Host
	scrapeTimeout := ctx.Value("scrapeTimeout").(time.Duration)
	navTimeout := ctx.Value("navTimeout").(time.Duration)

	// ---------------------------
	// 1. Get created long-lived browser ctx
	// ---------------------------
	browserCtx := ctx

	// Listen for network events
	chromedp.ListenTarget(browserCtx, func(ev interface{}) {
		switch e := ev.(type) {
		//		case *network.EventResponseReceived: -> response
		// 		case *network.EventDataReceived: -> when data chunk was received over network
		case *network.EventRequestWillBeSent:
			// Skip OPTIONS requests
			if e.Request.Method == "OPTIONS" {
				return
			}

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

			// Post data - but PostDataEntries just present request body in simple text
			if e.Request.PostDataEntries != nil {
				for _, entry := range e.Request.PostDataEntries {
					decoded := ""
					if entry.Bytes != "" {
						if b, err := base64.StdEncoding.DecodeString(entry.Bytes); err == nil {
							decoded = string(b)
						}
					}
					req.PostDataEntries = append(req.PostDataEntries, &structs.BodyDataEntryExtended{
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

			// Fetch missing POST data (for cases like fetch({body: {a:1}}))
			if e.Request.HasPostData {
				// go func(reqID network.RequestID, req *structs.RequestEntry) {
				resp, err := network.GetRequestPostData(e.RequestID).Do(ctx)
				if err == nil && resp != "" {
					decoded := resp
					req.PostDataEntries = append(req.PostDataEntries, &structs.BodyDataEntryExtended{
						Bytes:       base64.StdEncoding.EncodeToString([]byte(decoded)),
						DecodedText: decoded,
					})
				}
				// }(e.RequestID, req)
			}

			requests = append(requests, req)
		}
	})

	// JS to trigger likely network-calling functions (best-effort)
	triggerJS := `(function() {
		// === 1. Trigger all DOM-related network activity ===
		const evs = ['click','submit','change','mouseover','input'];
		for(const el of document.querySelectorAll('*')){
			for(const ev of evs){
				try { el.dispatchEvent(new Event(ev, {bubbles:true})); }catch(e){}
			}
		}

		// Submit all forms
		for(const form of document.forms){
			try { form.submit(); } catch(e){}
		}

		// === 2. Execute zero-argument HTTP-related functions ===
		const httpPattern = /\b(fetch|XMLHttpRequest|axios|\.ajax|sendBeacon|WebSocket|EventSource|Worker|SharedWorker)\b/;

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
	})();
	`

	// TODO do I need nav timeout
	// ---------------------------
	// 2. Navigation with timeout
	// ---------------------------
	navCtx, navCancel := context.WithTimeout(browserCtx, navTimeout)
	defer navCancel()

	if err := chromedp.Run(navCtx,
		chromedp.Navigate(targetURL),
	); err != nil {
		return nil, err
	}

	// ---------------------------
	// 3. Scrape / wait / JS trigger
	//    using full timeout you want
	// ---------------------------
	scrapeCtx, scrapeCancel := context.WithTimeout(browserCtx, scrapeTimeout)
	defer scrapeCancel()

	if err := chromedp.Run(scrapeCtx,
		chromedp.Evaluate(triggerJS, nil),
		chromedp.WaitReady("body", chromedp.ByQuery), // allow network traffic to happen
	); err != nil {
		// ignore context deadline exceeded — expected when waiting ends
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
		if r.URL == "" {
			continue
		}

		key := reqKey{
			Method: r.Method,
			URL:    helper.SanitizeURL(r.URL, baseOrigin),
		}
		if len(r.PostDataEntries) > 0 {
			key.Body = r.PostDataEntries[0].DecodedText
		}
		if !seen[key] {
			seen[key] = true
			r.URL = helper.SanitizeURL(r.URL, baseOrigin)
			deduped = append(deduped, r)
		}
	}

	return deduped, nil
}
