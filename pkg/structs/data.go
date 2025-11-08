package structs

type Param struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// HAR-like entry
type RequestEntry struct {
	URL             string                   `json:"url"`
	Method          string                   `json:"method"`
	ContentType     string                   `json:"contentType,omitempty"`
	Headers         map[string]string        `json:"headers,omitempty"`
	PostDataEntries []*PostDataEntryExtended `json:"postDataEntries,omitempty"`
	QueryParams     []Param                  `json:"queryParams,omitempty"`
	Response        *ResponseInfo            `json:"response,omitempty"`
}

type PostDataEntryExtended struct {
	Bytes       string `json:"bytes,omitempty"`
	DecodedText string `json:"decodedText,omitempty"`
}

type ResponseInfo struct {
	Status     int               `json:"status"`
	StatusText string            `json:"statusText"`
	Headers    map[string]string `json:"headers,omitempty"`
	MIMEType   string            `json:"mimeType,omitempty"`
}
