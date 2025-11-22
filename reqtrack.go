package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
	"github.com/m-1tZ/reqtrack/pkg/capture"
	"github.com/m-1tZ/reqtrack/pkg/helper"
	"github.com/m-1tZ/reqtrack/pkg/structs"
)

func main() {
	var targetURL string
	var header string
	var proxy string
	var scrapeTimeout int
	var navTimeout int

	flag.StringVar(&header, "H", "User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0", "Custom header")
	flag.IntVar(&navTimeout, "tnav", 7, "Navigation timeout (seconds)")
	flag.IntVar(&scrapeTimeout, "tscr", 10, "Scraping/trigger timeout (seconds)")
	flag.StringVar(&targetURL, "u", "", "URL to process")
	flag.StringVar(&proxy, "p", "", "Optional proxy (http://127.0.0.1:8080)")
	flag.Parse()

	// --- Exec Allocator Options ---
	chromedpOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", false),
		chromedp.Flag("disable-setuid-sandbox", false),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("blink-settings", "imagesEnabled=false"),
	)

	if proxy != "" {
		chromedpOpts = append(chromedpOpts, chromedp.ProxyServer(proxy))
	}

	// --- Browser Lifecycle ---
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), chromedpOpts...)
	defer allocCancel()

	browserCtx, browserCancel := chromedp.NewContext(allocCtx)
	defer browserCancel()

	// Allow browser to fully initialize
	if err := chromedp.Run(browserCtx); err != nil {
		log.Fatal(err)
	}

	// --- Store shared metadata in the browser context ---
	browserCtx = context.WithValue(browserCtx, "targetURL", targetURL)
	browserCtx = context.WithValue(browserCtx, "header", header)
	browserCtx = context.WithValue(browserCtx, "navTimeout", time.Duration(navTimeout)*time.Second)
	browserCtx = context.WithValue(browserCtx, "scrapeTimeout", time.Duration(scrapeTimeout)*time.Second)

	// --- Enable network + set headers once ---
	headers := helper.ParseHeaderFlag(header)
	if err := chromedp.Run(browserCtx,
		network.Enable(),
		network.SetExtraHTTPHeaders(network.Headers(headers)),
	); err != nil {
		log.Fatal(err)
	}

	// --- DYNAMIC Capture ---
	results, err := capture.CaptureRequests(browserCtx)
	if err != nil {
		log.Fatal(err)
	}

	// --- STATIC HTML Scrape ---
	// staticFindings, err := scrape.ScrapeHtml(browserCtx)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// --- Merge, dedupe, output ---
	// combined := append(results, staticFindings...)
	deduped := DedupeRequests(results, targetURL)

	out, _ := json.MarshalIndent(deduped, "", "  ")
	fmt.Println(string(out))
}

func DedupeRequests(requests []*structs.RequestEntry, targetURL string) []*structs.RequestEntry {
	parsedBase, _ := url.Parse(targetURL)
	baseOrigin := parsedBase.Scheme + "://" + parsedBase.Host

	type reqKey struct {
		Method string
		URL    string
		Body   string
	}

	seen := make(map[reqKey]bool)
	var deduped []*structs.RequestEntry

	for _, r := range requests {
		if r == nil || r.URL == "" {
			continue
		}

		key := reqKey{
			Method: r.Method,
			URL:    helper.SanitizeURL(r.URL, baseOrigin),
		}

		// Use first body entry if present
		if len(r.PostDataEntries) > 0 {
			key.Body = r.PostDataEntries[0].DecodedText
		}

		if !seen[key] {
			seen[key] = true

			// mutate in-place: sanitize URL too
			r.URL = key.URL

			deduped = append(deduped, r)
		}
	}

	return deduped
}

// parsedBase, _ := url.Parse(targetURL) baseOrigin := parsedBase.Scheme + "://" + parsedBase.Host // Merge dynamic and static findings combined := append(results, staticFindings...) // Deduplicate exact requests (reuse your logic) type reqKey struct { Method string URL string Body string } seen := make(map[reqKey]bool) var deduped []*structs.RequestEntry for _, r := range combined { if r.URL == "" { continue } key := reqKey{ Method: r.Method, URL: helper.SanitizeURL(r.URL, baseOrigin), } if len(r.PostDataEntries) > 0 { key.Body = r.PostDataEntries[0].DecodedText } if !seen[key] { seen[key] = true r.URL = helper.SanitizeURL(r.URL, baseOrigin) deduped = append(deduped, r) } } out, _ := json.MarshalIndent(deduped, "", " ") fmt.Println(string(out))
