package main

import (
	"flag"
	"log"
	"time"

	"github.com/m-1tZ/reqtrack/pkg/capture"
	"github.com/m-1tZ/reqtrack/pkg/helper"
	"github.com/m-1tZ/reqtrack/pkg/scrape"
	pw "github.com/playwright-community/playwright-go"
)

func main() {
	var targetURL string
	var header string
	var parseTimeout float64
	var navTimeout float64
	var proxy string
	var harPath string

	flag.StringVar(&header, "H",
		"User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0",
		"Custom header")
	flag.StringVar(&targetURL, "u", "", "URL to process")
	flag.Float64Var(&parseTimeout, "tparse", 30, "Timeout for parsing of scripts with AST (default 30s)")
	flag.Float64Var(&navTimeout, "tnav", 7, "Timeout for navigation and script evaluation (default 7s)")
	flag.StringVar(&proxy, "p", "", "Optional proxy (http://127.0.0.1:8080)")
	flag.StringVar(&harPath, "har", "traffic.har", "HAR output file")

	flag.Parse()

	if targetURL == "" {
		log.Fatal("Missing -u URL")
	}

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

	// browserCtx.SetDefaultTimeout(float64((2 * time.Second) / time.Millisecond)) // 30s
	browserCtx.SetDefaultNavigationTimeout(float64((time.Duration(navTimeout) * time.Second) / time.Millisecond))

	page, err := browserCtx.NewPage()
	if err != nil {
		log.Fatal(err)
	}

	// ---- CAPTURE (navigate + JS triggers) ----
	if err = capture.CaptureRequests(page, targetURL); err != nil {
		log.Fatal(err)
	}

	// ---- SCRAPE (static / heuristics) ----
	scrapeHarEntries, err := scrape.ScrapeRequests(page, browserCtx, targetURL, float64((time.Duration(navTimeout)*time.Second)/time.Millisecond), parseTimeout)
	if err != nil {
		log.Fatal(err)
	}

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

	// ---- MERGE SCRAPED + HAR LOADED ----
	merged := helper.MergeHAREntries(entries, scrapeHarEntries)

	deduped, err := helper.DeduplicateHAREntries(merged, targetURL)
	if err != nil {
		log.Fatal(err)
	}

	err = helper.WriteHAR(harPath, deduped)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Done. HAR saved at: %s", harPath)
}
