# reqtrack
Reqtrack is a request tracker tool that helps to identify occured and occuring HTTP requests on a web app. The tool uses the [Chrome DevTools Protocol](https://github.com/chromedp/chromedp) and outputs interfered and captured requests. To geht the most out of it, this tool triggers known HTTP request primitves proactively.

This approach is based on [RESTful API Inference](https://kilthub.cmu.edu/articles/thesis/RESTful_API_Inference/25796629), a master's thesis that describes the approach to infere API calls within source code.
> This work is a start in this direction and can be further taken on to calling all the API endpoints inferred by combination of static and dynamic analysis and automate the whole process.

## Installation

`go install github.com/m-1tZ/reqtrack`

## Usage

```bash
$ reqtrack -u http://127.0.0.1:5000

[
  {
    "url": "http://127.0.0.1:5000/",
    "method": "GET",
    "headers": {
      "Upgrade-Insecure-Requests": "1",
      "User-Agent": "Mozilla/5.0 (X11",
      "rv": "144.0) Gecko/20100101 Firefox/144.0",
      "sec-ch-ua": "\"Not_A Brand\";v=\"99\", \"Chromium\";v=\"142\"",
      "sec-ch-ua-mobile": "?0",
      "sec-ch-ua-platform": "\"Linux\""
    },
    "response": {
      "status": 200,
      "statusText": "OK",
      "headers": {
        "Connection": "close",
        "Content-Length": "7328",
        "Content-Type": "text/html; charset=utf-8",
        "Date": "Sat, 08 Nov 2025 21:20:54 GMT",
        "Server": "Werkzeug/3.1.3 Python/3.13.7"
      },
      "mimeType": "text/html"
    }
  },
  {
    "url": "http://localhost:3000/test/fetch1",
    "method": "GET",
    "headers": {
      "Referer": "http://127.0.0.1:5000/",
      "User-Agent": "Mozilla/5.0 (X11",
      "rv": "144.0) Gecko/20100101 Firefox/144.0",
      "sec-ch-ua": "\"Not_A Brand\";v=\"99\", \"Chromium\";v=\"142\"",
      "sec-ch-ua-mobile": "?0",
      "sec-ch-ua-platform": "\"Linux\""
    }
  },
  {
    "url": "http://localhost:3000/test/fetch2",
    "method": "POST",
    "contentType": "text/plain;charset=UTF-8",
    "headers": {
      "Content-Type": "text/plain;charset=UTF-8",
      "Referer": "http://127.0.0.1:5000/",
      "User-Agent": "Mozilla/5.0 (X11",
      "rv": "144.0) Gecko/20100101 Firefox/144.0",
      "sec-ch-ua": "\"Not_A Brand\";v=\"99\", \"Chromium\";v=\"142\"",
      "sec-ch-ua-mobile": "?0",
      "sec-ch-ua-platform": "\"Linux\""
    },
    "postDataEntries": [
      {
        "bytes": "eyJhIjoxfQ==",
        "decodedText": "{\"a\":1}"
      }
    ]
  },
...
]
```