# reqtrack
Reqtrack is a request tracker tool that helps to identify occured and occuring HTTP requests on a web app. The tool uses the [Playwright](https://github.com/playwright-com/chromedp) and outputs captured requests in a [HAR](https://en.wikipedia.org/wiki/HAR_(file_format)) format. To geht the most out of it, this tool combines static and the dynamic approach and triggers known HTTP request primitves proactively. Additionally the HAR output file will be normalized, filtered, and minified to the interesting entries such as `request`.

This approach is partially based on [RESTful API Inference](https://kilthub.cmu.edu/articles/thesis/RESTful_API_Inference/25796629), a master's thesis that describes the approach to infere API calls within source code.
> This work is a start in this direction and can be further taken on to calling all the API endpoints inferred by combination of static and dynamic analysis and automate the whole process.

## Installation

`go install github.com/m-1tZ/reqtrack`

## Usage

```bash
$ reqtrack -u http://127.0.0.1:5000
$ cat traffic.har

{
    "log": {
        "entries": [{
                    "request": {
                        "method": "POST",
                        "url": "http://127.0.0.1:5000/somerequest",
                        "httpVersion": "HTTP/1.1",
                        "cookies": [],
                        "headers": [...],
                        "queryString": [],
                        "headersSize": 587,
                        "bodySize": 0
                    }
                }, {
                    "request": {
                        "method": "GET",
                        "url": "http://127.0.0.1:5000/test/notexecuted-fetch",
                        "httpVersion": "HTTP/1.1",
                        "cookies": [],
                        "headers": [...],
                        "queryString": [],
                        "headersSize": 470,
                        "bodySize": 0
                    }
                }
...
```
