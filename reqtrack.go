package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/url"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/m-1tZ/reqtrack/pkg/capture"
	"github.com/m-1tZ/reqtrack/pkg/helper"
	"github.com/m-1tZ/reqtrack/pkg/scrape"
	"github.com/m-1tZ/reqtrack/pkg/structs"
)

func main() {
	var targetURL string
	var header string
	var proxy string
	var timeout int

	flag.StringVar(&header, "H", "User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0", "Custom header, e.g., -H 'User-Agent: xyz'")
	flag.IntVar(&timeout, "t", 10, "Timeout to wait")
	flag.StringVar(&targetURL, "u", "", "URL to process")
	flag.StringVar(&proxy, "p", "", "Optional - proxy traffic (http://127.0.0.1:8080)")
	flag.Parse()

	// Chromedp options
	chromedpOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.ProxyServer(proxy),
		chromedp.Flag("no-sandbox", false),
		chromedp.Flag("disable-setuid-sandbox", false),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("blink-settings", fmt.Sprintf("imagesEnabled=%t", false)),
	)

	if proxy != "" {
		chromedpOpts = append(chromedp.DefaultExecAllocatorOptions[:],
			chromedp.ProxyServer(proxy),
		)
	}

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), chromedpOpts...)
	defer cancel()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	// Fill context
	ctx = context.WithValue(ctx, "targetURL", targetURL)
	ctx = context.WithValue(ctx, "header", header)
	ctx = context.WithValue(ctx, "timeout", time.Duration(timeout)*time.Second)

	// Capture/dynamic

	// type RequestEntry - WORKS

	results, err := capture.CaptureRequests(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// out, _ := json.MarshalIndent(results, "", "  ")
	// fmt.Println(string(out))

	// Static
	staticFindings, err := scrape.ScrapeHtml(ctx)
	if err != nil {
		log.Fatal(err)
	}

	// out, _ = json.MarshalIndent(staticFindings, "", "  ")
	// fmt.Println(string(out))

	parsedBase, _ := url.Parse(targetURL)
	baseOrigin := parsedBase.Scheme + "://" + parsedBase.Host

	// Merge dynamic and static findings
	combined := append(results, staticFindings...)

	// Deduplicate exact requests (reuse your logic)
	type reqKey struct {
		Method string
		URL    string
		Body   string
	}

	seen := make(map[reqKey]bool)
	var deduped []*structs.RequestEntry

	for _, r := range combined {
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

	out, _ := json.MarshalIndent(deduped, "", "  ")
	fmt.Println(string(out))

	// Write results
	// 	out, _ := json.MarshalIndent(results, "", "  ")
	// 	_ = os.WriteFile("output.har.json", out, 0644)

	// 	fmt.Println("Captured requests written to output.har.json")
}

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
