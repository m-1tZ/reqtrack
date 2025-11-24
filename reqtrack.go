package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/m-1tZ/reqtrack/pkg/helper"
	"github.com/m-1tZ/reqtrack/pkg/scrape"
	pw "github.com/playwright-community/playwright-go"
)

func main() {
	var targetURL string
	var header string
	var totalTimeout float64
	var navTimeout float64
	var proxy string
	var harPath string

	flag.StringVar(&header, "H",
		"User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0",
		"Custom header")
	flag.StringVar(&targetURL, "u", "", "URL to process")
	flag.Float64Var(&totalTimeout, "ttotal", 60, "Timeout for the total processing (default 60s)")
	flag.Float64Var(&navTimeout, "tnav", 7, "Timeout for navigation and script evaluation (default 7s)")
	flag.StringVar(&proxy, "p", "", "Optional proxy (http://127.0.0.1:8080)")
	flag.StringVar(&harPath, "har", "traffic.har", "HAR output file")

	flag.Parse()

	if targetURL == "" {
		log.Fatal("Missing -u URL")
	}

	ctxGlobal, cancel := context.WithTimeout(context.Background(),
		time.Duration(totalTimeout)*time.Second)
	defer cancel()

	// ---- Playwright Setup ----
	// if err := pw.Install(); err != nil {
	// 	log.Fatal(err)
	// }

	pwRunner, err := pw.Run()
	if err != nil {
		log.Fatal(err)
	}

	launchOpts := pw.BrowserTypeLaunchOptions{
		Headless: pw.Bool(true),
	}
	if proxy != "" {
		launchOpts.Proxy = &pw.Proxy{Server: proxy}
	}

	browser, err := pwRunner.Chromium.Launch(launchOpts)
	if err != nil {
		log.Fatal(err)
	}

	// ---- Parse header flag ----
	hKey, hVal := helper.ParseHeaderFlag(header)

	// ---- Browser Context with HAR ----
	browserCtx, err := browser.NewContext(pw.BrowserNewContextOptions{
		RecordHarPath: pw.String(harPath),
		RecordHarMode: pw.HarModeFull,
		ExtraHttpHeaders: map[string]string{
			hKey: hVal,
		},
		IgnoreHttpsErrors: pw.Bool(true),
	})
	if err != nil {
		log.Fatal(err)
	}

	browserCtx.SetDefaultTimeout(float64((30 * time.Second) / time.Millisecond)) // 30s
	browserCtx.SetDefaultNavigationTimeout(float64((time.Duration(totalTimeout) * time.Second) / time.Millisecond))

	// TODO implement timeout from arguments
	// timeout := time.Duration(totalTimeout) * time.Second
	// ctx, cancel := context.WithTimeout(ctx, timeout)
	// defer cancel()

	page, err := browserCtx.NewPage()
	if err != nil {
		log.Fatal(err)
	}

	// ---- CAPTURE (navigate + JS triggers) ----
	// if err = capture.CaptureRequests(ctxGlobal, page, targetURL); err != nil {
	// 	log.Fatal(err)
	// }

	// ---- SCRAPE (static / heuristics) ----
	scrapeHarEntries, err := scrape.ScrapeRequests(ctxGlobal, page, browserCtx, targetURL)
	if err != nil {
		log.Fatal(err)
	}
	// TODO use this and merge with captured har
	fmt.Println(scrapeHarEntries)

	// Scrape results can only be integrated if .har file was written
	// loop over har file and remove response objects. add scraped objects and unique the requests

	// ---- Close so HAR gets written ----
	if err = browserCtx.Close(); err != nil {
		log.Fatal(err)
	}

	log.Printf("Done. HAR will now be deduplicated and minified")
	entries, err := helper.LoadHAREntriesStreaming(harPath)
	if err != nil {
		log.Fatal(err)
	}

	// TODO integrate scrapeHarEntries

	deduped, err := helper.DeduplicateHAREntries(entries, targetURL)
	if err != nil {
		log.Fatal(err)
	}

	err = helper.WriteHAR(harPath+"_deduped.har", deduped)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Done. HAR saved at: %s", harPath)
}
