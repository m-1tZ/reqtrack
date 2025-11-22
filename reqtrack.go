package main

import (
	"flag"
	"log"

	"github.com/m-1tZ/reqtrack/pkg/capture"
	"github.com/m-1tZ/reqtrack/pkg/helper"
	pw "github.com/playwright-community/playwright-go"
)

func main() {
	var targetURL string
	var header string
	var totalTimeout int64
	var proxy string
	var harPath string

	flag.StringVar(&header, "H",
		"User-Agent: Mozilla/5.0 (X11; Linux x86_64; rv:144.0) Gecko/20100101 Firefox/144.0",
		"Custom header")

	flag.StringVar(&targetURL, "u", "", "URL to process")
	flag.Int64Var(&totalTimeout, "t", 30, "Timeout for the total processing (default 30s)")
	flag.StringVar(&proxy, "p", "", "Optional proxy (http://127.0.0.1:8080)")
	flag.StringVar(&harPath, "har", "traffic.har", "HAR output file")

	flag.Parse()

	if targetURL == "" {
		log.Fatal("Missing -u URL")
	}

	// ---- Playwright Setup ----
	if err := pw.Install(); err != nil {
		log.Fatal(err)
	}

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
	ctx, err := browser.NewContext(pw.BrowserNewContextOptions{
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

	// TODO implement timeout from arguments
	// timeout := time.Duration(totalTimeout) * time.Second
	// ctx, cancel := context.WithTimeout(ctx, timeout)
	// defer cancel()

	page, err := ctx.NewPage()
	if err != nil {
		log.Fatal(err)
	}

	// ---- CAPTURE (navigate + JS triggers) ----
	if err := capture.CaptureRequests(ctx, page, targetURL); err != nil {
		log.Fatal(err)
	}

	// ---- SCRAPE (static / heuristics) ----
	// if err := ScrapeRequests(ctx, page); err != nil {
	// 	log.Fatal(err)
	// }

	// Scrape results can only be integrated if .har file was written
	// loop over har file and remove response objects. add scraped objects and unique the requests

	// ---- Close so HAR gets written ----
	if err := ctx.Close(); err != nil {
		log.Fatal(err)
	}

	log.Printf("Done. HAR saved at: %s", harPath)
}
