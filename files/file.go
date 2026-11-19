package files

import (
	"mime"
	"path/filepath"
	"strings"

	"github.com/chimixiong/deepseek-sdk-go"
)

const (
	maxFilenameBytes     = 512
	maxImageFileBytes    = 64 << 20
	maxDocumentFileBytes = 100 << 20
	maxTextFileBytes     = 32 << 20
)

// Purpose is the intended use of an uploaded file.
type Purpose string

// Documented file purposes.
const (
	// PurposeUserData marks a general user file such as an image.
	PurposeUserData Purpose = "user_data"
	// PurposeTextRecognition asks the service to parse text from a document.
	PurposeTextRecognition Purpose = "text_recognition"
	// PurposeRichLayoutRecognition asks for layout-aware document parsing.
	PurposeRichLayoutRecognition Purpose = "rich_layout_recognition"
)

// ExpiresAnchor is the reference point for a file lifetime.
type ExpiresAnchor string

// AnchorCreatedAt is the only documented anchor: the upload time.
const AnchorCreatedAt ExpiresAnchor = "created_at"

// ExpiresAfter sets a file's lifetime. Both fields are sent together.
type ExpiresAfter struct {
	// Anchor defaults to created_at.
	Anchor ExpiresAnchor
	// Seconds is the lifetime, between 3600 and 2592000.
	Seconds int
}

// CreateRequest is a multipart upload. Purpose is always encoded before File.
type CreateRequest struct {
	Purpose Purpose
	// File is the raw file content.
	File     []byte
	Filename string
	// ContentType defaults to application/octet-stream.
	ContentType string
	// ExpiresAfter makes the file temporary; nil means it never expires.
	ExpiresAfter *ExpiresAfter
}

// Validate checks the documented upload constraints.
func (r *CreateRequest) Validate() error {
	if r.Purpose == "" {
		return deepseek.Invalid("purpose", "is required")
	}
	switch r.Purpose {
	case PurposeUserData, PurposeTextRecognition, PurposeRichLayoutRecognition:
	default:
		return deepseek.Invalid("purpose", "must be %q, %q or %q", PurposeUserData, PurposeTextRecognition, PurposeRichLayoutRecognition)
	}
	if r.Filename == "" {
		return deepseek.Invalid("file", "filename is required")
	}
	if len(r.Filename) > maxFilenameBytes {
		return deepseek.Invalid("file", "filename must be at most %d bytes, got %d", maxFilenameBytes, len(r.Filename))
	}
	if len(r.File) == 0 {
		return deepseek.Invalid("file", "content is required")
	}
	if limit := fileSizeLimit(r.ContentType, r.Filename); len(r.File) > limit {
		return deepseek.Invalid("file", "content must be at most %d bytes (%d MiB) for this file type, got %d bytes", limit, limit>>20, len(r.File))
	}
	if r.ExpiresAfter != nil {
		if r.ExpiresAfter.Anchor != "" && r.ExpiresAfter.Anchor != AnchorCreatedAt {
			return deepseek.Invalid("expires_after[anchor]", "must be %q", AnchorCreatedAt)
		}
		if r.ExpiresAfter.Seconds < 3600 || r.ExpiresAfter.Seconds > 2592000 {
			return deepseek.Invalid("expires_after[seconds]", "must be between 3600 and 2592000")
		}
	}
	return nil
}

// fileSizeLimit returns the documented upload limit for a file. The service
// chooses the limit by file type, not by purpose.
func fileSizeLimit(contentType, filename string) int {
	if mediaType, _, err := mime.ParseMediaType(contentType); err == nil {
		switch {
		case strings.HasPrefix(mediaType, "image/"):
			return maxImageFileBytes
		case mediaType == "application/pdf",
			mediaType == "application/msword",
			mediaType == "application/vnd.openxmlformats-officedocument.wordprocessingml.document",
			mediaType == "application/vnd.ms-powerpoint",
			mediaType == "application/vnd.openxmlformats-officedocument.presentationml.presentation":
			return maxDocumentFileBytes
		case strings.HasPrefix(mediaType, "text/"),
			mediaType == "application/vnd.ms-excel",
			mediaType == "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
			mediaType == "application/epub+zip",
			mediaType == "application/x-mobipocket-ebook":
			return maxTextFileBytes
		}
	}
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".png", ".jpg", ".jpeg", ".webp", ".gif", ".bmp", ".tif", ".tiff":
		return maxImageFileBytes
	case ".pdf", ".doc", ".docx", ".ppt", ".pptx":
		return maxDocumentFileBytes
	case ".txt", ".md", ".csv", ".tsv", ".xls", ".xlsx", ".epub", ".mobi", ".rtf", ".odt":
		return maxTextFileBytes
	}
	// Unknown types get the largest documented limit; the server still
	// classifies the content.
	return maxDocumentFileBytes
}

// Order is the sort direction for file listings.
type Order string

// Documented sort directions.
const (
	OrderAsc  Order = "asc"
	OrderDesc Order = "desc"
)

// ListOptions filters and paginates a file listing. Zero values are omitted.
type ListOptions struct {
	// After is a file id cursor; the API returns files after it.
	After string
	// Limit is the page size, between 1 and 1000.
	Limit *int
	// Order defaults to asc.
	Order   Order
	Purpose Purpose
}

func (o *ListOptions) validate() error {
	if o.Limit != nil && (*o.Limit < 1 || *o.Limit > 1000) {
		return deepseek.Invalid("limit", "must be between 1 and 1000")
	}
	if o.Order != "" && o.Order != OrderAsc && o.Order != OrderDesc {
		return deepseek.Invalid("order", "must be %q or %q", OrderAsc, OrderDesc)
	}
	switch o.Purpose {
	case "", PurposeUserData, PurposeTextRecognition, PurposeRichLayoutRecognition:
	default:
		return deepseek.Invalid("purpose", "must be %q, %q or %q", PurposeUserData, PurposeTextRecognition, PurposeRichLayoutRecognition)
	}
	return nil
}

// File is an uploaded file object.
type File struct {
	ID        string  `json:"id"`
	Object    string  `json:"object"`
	Bytes     int64   `json:"bytes"`
	CreatedAt int64   `json:"created_at"`
	Filename  string  `json:"filename"`
	Purpose   Purpose `json:"purpose"`
	// ExpiresAt is set only when the upload requested a lifetime.
	ExpiresAt *int64 `json:"expires_at,omitempty"`
}

// ListResponse is one page of files.
type ListResponse struct {
	Object  string `json:"object"`
	Data    []File `json:"data"`
	FirstID string `json:"first_id,omitempty"`
	LastID  string `json:"last_id,omitempty"`
	HasMore bool   `json:"has_more"`
}

// DeleteResponse reports a deleted file.
type DeleteResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Deleted bool   `json:"deleted"`
}
