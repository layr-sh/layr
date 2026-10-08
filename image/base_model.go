// Package image defines the image optimization, resizing, and transformation engine.
package image

// Supported image output formats.
const (
	FormatWebP = "webp"
	FormatAVIF = "avif"
	FormatPNG  = "png"
	FormatJPEG = "jpeg"
	FormatJPG  = "jpg"
	FormatGIF  = "gif"
	FormatSVG  = "svg"
	FormatICO  = "ico"
	FormatTIFF = "tiff"
	FormatBMP  = "bmp"
)

// GetInfoResponse represents the inspection metadata returned by the /info endpoints.
type GetInfoResponse struct {
	Size        int64          `json:"size,omitempty"`
	Format      string         `json:"format,omitempty"`
	MIMEType    string         `json:"mime_type,omitempty"`
	Width       int            `json:"width,omitempty"`
	Height      int            `json:"height,omitempty"`
	Orientation int            `json:"orientation,omitempty"`
	Colorspace  string         `json:"colorspace,omitempty"`
	Bands       int            `json:"bands,omitempty"`
	Pages       int            `json:"pages,omitempty"`
	Alpha       *bool          `json:"alpha,omitempty"`
	EXIF        map[string]any `json:"exif,omitempty"`
	XMP         string         `json:"xmp,omitempty"`
	VideoMeta   map[string]any `json:"video_meta,omitempty"`
}
