package structs

type Param struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type BodyDataEntryExtended struct {
	Bytes       string `json:"bytes"`
	DecodedText string `json:"decoded_text"`
}

// type ResponseInfo struct {
// 	Status     int               `json:"status"`
// 	StatusText string            `json:"status_text"`
// 	Headers    map[string]string `json:"headers"`
// 	MIMEType   string            `json:"mime_type"`
// }

type RequestEntry struct {
	URL             string                   `json:"url"`
	Method          string                   `json:"method"`
	Headers         map[string]string        `json:"headers"`
	ContentType     string                   `json:"content_type"`
	QueryParams     []Param                  `json:"query_params"`
	PostDataEntries []*BodyDataEntryExtended `json:"body_data_entries"`
	// Response        *ResponseInfo            `json:"response,omitempty"`
}
