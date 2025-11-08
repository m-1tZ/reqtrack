package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/chromedp/chromedp"
	"github.com/m-1tZ/reqtrack/pkg/capture"
)

func main() {
	var targetURL string
	var header string
	var timeout int

	flag.StringVar(&header, "H", "User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0", "Custom header, e.g., -H 'User-Agent: xyz'")
	flag.IntVar(&timeout, "t", 10, "Timeout to wait")
	flag.StringVar(&targetURL, "u", "", "URL to process")
	flag.Parse()

	// Chromedp options
	chromedpOpts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", true),
		chromedp.Flag("no-sandbox", false),
		chromedp.Flag("disable-setuid-sandbox", false),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-gpu", true),
		chromedp.Flag("disable-dev-shm-usage", true),
		chromedp.Flag("disable-breakpad", true),
		chromedp.Flag("blink-settings", fmt.Sprintf("imagesEnabled=%t", false)),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), chromedpOpts...)
	defer cancel()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	// Capture
	results, err := capture.CaptureRequests(ctx, targetURL, header, time.Duration(timeout)*time.Second)
	if err != nil {
		log.Fatal(err)
	}

	out, _ := json.MarshalIndent(results, "", "  ")
	fmt.Println(string(out))

	// Write results
	// 	out, _ := json.MarshalIndent(results, "", "  ")
	// 	_ = os.WriteFile("output.har.json", out, 0644)

	// 	fmt.Println("Captured requests written to output.har.json")
}
