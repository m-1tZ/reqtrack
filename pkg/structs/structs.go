package structs

// Full HAR file root
type HAR struct {
	Log HARLog `json:"log"`
}

// "log" object
type HARLog struct {
	Version string     `json:"version"`
	Creator HARCreator `json:"creator"`
	Browser HARBrowser `json:"browser"`
	Pages   []HARPage  `json:"pages"`
	Entries []HAREntry `json:"entries"`
}

// "creator" section
type HARCreator struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// "browser" section
type HARBrowser struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// Page metadata
type HARPage struct {
	StartedDateTime string       `json:"startedDateTime"`
	ID              string       `json:"id"`
	Title           string       `json:"title"`
	PageTimings     HARPageTimes `json:"pageTimings"`
}

// Page timings
type HARPageTimes struct {
	OnContentLoad int `json:"onContentLoad"`
	OnLoad        int `json:"onLoad"`
}

// ∙∙∙ REQUEST ENTRY ∙∙∙
type HAREntry struct {
	StartedDateTime string      `json:"startedDateTime"`
	Time            float64     `json:"time"`
	Request         HARRequest  `json:"request"`
	Response        HARResponse `json:"response"`
	Cache           interface{} `json:"cache"`
	Timings         HARTimings  `json:"timings"`
	ServerIPAddress string      `json:"serverIPAddress,omitempty"`
	Connection      string      `json:"connection,omitempty"`
	Pageref         string      `json:"pageref,omitempty"`
}

// Request section
type HARRequest struct {
	Method      string         `json:"method"`
	URL         string         `json:"url"`
	HTTPVersion string         `json:"httpVersion"`
	Cookies     []HARCookie    `json:"cookies"`
	Headers     []HARNameValue `json:"headers"`
	Query       []HARNameValue `json:"queryString"`
	PostData    *HARPostData   `json:"postData,omitempty"`
	HeaderSize  int            `json:"headersSize"`
	BodySize    int            `json:"bodySize"`
}

// POST data section
type HARPostData struct {
	MimeType string         `json:"mimeType"`
	Params   []HARPostParam `json:"params,omitempty"`
	Text     string         `json:"text,omitempty"`
}

// POST form params
type HARPostParam struct {
	Name        string `json:"name"`
	Value       string `json:"value"`
	FileName    string `json:"fileName,omitempty"`
	ContentType string `json:"contentType,omitempty"`
}

// Name/value pair (for headers, query params)
type HARNameValue struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Response section (we will empty this)
type HARResponse struct {
	Status      int            `json:"status"`
	StatusText  string         `json:"statusText"`
	HTTPVersion string         `json:"httpVersion"`
	Cookies     []HARCookie    `json:"cookies"`
	Headers     []HARNameValue `json:"headers"`
	Content     HARContent     `json:"content"`
	RedirectURL string         `json:"redirectURL"`
	HeadersSize int            `json:"headersSize"`
	BodySize    int            `json:"bodySize"`
}

// Response bodies (we keep empty)
type HARContent struct {
	Size        int    `json:"size"`
	MimeType    string `json:"mimeType"`
	Text        string `json:"text,omitempty"`
	Encoding    string `json:"encoding,omitempty"`
	Compression int    `json:"compression,omitempty"`
}

// Cookies
type HARCookie struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Path     string `json:"path,omitempty"`
	Domain   string `json:"domain,omitempty"`
	Expires  string `json:"expires,omitempty"`
	HTTPOnly bool   `json:"httpOnly,omitempty"`
	Secure   bool   `json:"secure,omitempty"`
	SameSite string `json:"sameSite,omitempty"`
}

// Timings object
type HARTimings struct {
	Blocked float64 `json:"blocked"`
	DNS     float64 `json:"dns"`
	Connect float64 `json:"connect"`
	Send    float64 `json:"send"`
	Wait    float64 `json:"wait"`
	Receive float64 `json:"receive"`
	Ssl     float64 `json:"ssl"`
}
