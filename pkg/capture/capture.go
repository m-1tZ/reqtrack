package capture

import (
	"fmt"
	"log"

	pw "github.com/playwright-community/playwright-go"
)

func CaptureRequests(ctx pw.BrowserContext, page pw.Page, targetURL string) error {
	// Navigate and wait for network to be idle
	_, err := page.Goto(targetURL, pw.PageGotoOptions{
		WaitUntil: pw.WaitUntilStateNetworkidle,
	})
	if err != nil {
		return fmt.Errorf("goto failed: %w", err)
	}

	// Execute your JS trigger
	triggerJS := getTriggerJS()
	if _, err := page.Evaluate(triggerJS); err != nil {
		log.Println("Trigger script error:", err)
	}

	// Wait for triggered network calls to settle
	// TODO wait for state WaitUntilStateNetworkidle
	if err := page.WaitForLoadState(); err != nil {
		return fmt.Errorf("wait for networkidle failed: %w", err)
	}

	return nil
}

// func CaptureRequests(ctx context.Context, browserContext pw.BrowserContext, page pw.Page, targetURL string, timeout time.Duration) error {
// 	// Navigate and wait for network to be idle
// 	_, err := page.Goto(targetURL, pw.PageGotoOptions{
// 		WaitUntil: pw.WaitUntilStateNetworkidle,
// 	})
// 	if err != nil {
// 		return fmt.Errorf("goto failed: %w", err)
// 	}

// 	// Execute your JS trigger with timeout
// 	triggerJS := getTriggerJS()
// 	done := make(chan error, 1)
// 	go func() {
// 		_, err := page.Evaluate(triggerJS)
// 		done <- err
// 	}()

// 	select {
// 	case <-ctx.Done():
// 		return fmt.Errorf("context cancelled or timed out before JS trigger finished")
// 	case err := <-done:
// 		if err != nil {
// 			return fmt.Errorf("trigger script error: %w", err)
// 		}
// 	}

// 	// Wait for triggered network calls to settle
// 	waitCtx, cancel := context.WithTimeout(ctx, timeout)
// 	defer cancel()

// 	if err := page.WaitForLoadState(pw.PageWaitForLoadStateOptions{
// 		State:   pw.WaitUntilStateNetworkidle,
// 		Timeout: timeout.Milliseconds(),
// 	}); err != nil {
// 		return fmt.Errorf("wait for networkidle failed: %w", err)
// 	}

// 	return nil
// }

func getTriggerJS() string {
	return `(function() {
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
	})();`
}
