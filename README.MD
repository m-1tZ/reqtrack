# reqtrack
Reqtrack is a request tracker tool that helps to identify occured and occuring HTTP requests on a web app. The tool uses the [Chrome DevTools Protocol](https://github.com/chromedp/chromedp) and outputs interfered and captured requests. To geht the most out of it, this tool combines static and the dynamic approach and triggers known HTTP request primitves proactively.

This approach is based on [RESTful API Inference](https://kilthub.cmu.edu/articles/thesis/RESTful_API_Inference/25796629), a master's thesis that describes the approach to infere API calls within source code.
> This work is a start in this direction and can be further taken on to calling all the API endpoints inferred by combination of static and dynamic analysis and automate the whole process.

## Installation

`go install github.com/m-1tZ/reqtrack`

## Usage

```bash
$ reqtrack -u http://127.0.0.1:5000

[
...
]
```


https://github.com/chromedp/chromedp/issues/42#issuecomment-500191682