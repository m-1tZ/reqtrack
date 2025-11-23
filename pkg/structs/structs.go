package structs

type Param struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// ---- HAR -----

// Full HAR file root
type HAR struct {
	Log HARLog `json:"log"`
}

// "log" object
type HARLog struct {
	Entries []HAREntry `json:"entries"`
}

// ∙∙∙ REQUEST ENTRY ∙∙∙
type HAREntry struct {
	Request HARRequest `json:"request"`
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
