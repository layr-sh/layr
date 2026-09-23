package image

import (
	"encoding/base64"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Standard gravity type definitions.
const (
	GravityNorth     = "no"
	GravitySouth     = "so"
	GravityEast      = "ea"
	GravityWest      = "we"
	GravityCenter    = "ce"
	GravityNorthEast = "noea"
	GravityNorthWest = "nowe" //nolint:misspell
	GravitySouthEast = "soea"
	GravitySouthWest = "sowe"
	GravitySmart     = "sm"
	GravityFocus     = "fp"
)

// Standard resizing types.
const (
	ResizeFit      = "fit"
	ResizeFill     = "fill"
	ResizeFillDown = "fill-down"
	ResizeForce    = "force"
	ResizeAuto     = "auto"
)

const (
	argCountOne   = 1
	argCountTwo   = 2
	argCountThree = 3
	argCountFour  = 4
	argCountFive  = 5

	maxPresetRecursionDepth = 5
)

// GravityOption represents positioning coordinates and gravity anchors.
type GravityOption struct {
	Type    string
	XOffset float64
	YOffset float64
}

// ProcessingOptions contains all parsed transformation parameters.
type ProcessingOptions struct {
	ResizeType           string
	Width                int
	Height               int
	MinWidth             int
	MinHeight            int
	Enlarge              bool
	Extend               bool
	ExtendGravity        GravityOption
	ResizingAlgorithm    string
	DPR                  float64
	Gravity              GravityOption
	CropWidth            float64
	CropHeight           float64
	CropGravity          GravityOption
	TrimThreshold        float64
	TrimColor            string
	TrimEqualHor         bool
	TrimEqualVer         bool
	PaddingTop           int
	PaddingRight         int
	PaddingBottom        int
	PaddingLeft          int
	AutoRotate           bool
	Rotate               int
	Background           string
	BackgroundAlpha      float64
	Blur                 float64
	Sharpen              float64
	Pixelate             int
	UnsharpenMode        float64
	UnsharpenWeight      float64
	UnsharpenDividor     float64
	UnsharpenThreshold   float64
	WatermarkOpacity     float64
	WatermarkGravity     GravityOption
	WatermarkXOffset     int
	WatermarkYOffset     int
	WatermarkScale       float64
	WatermarkURL         string
	Format               string
	Quality              int
	FormatQuality        map[string]int
	AutoQualityMethod    string
	AutoQualityTarget    float64
	AutoQualityMin       int
	AutoQualityMax       int
	StripMetadata        bool
	KeepCopyright        bool
	StripColorProfile    bool
	ZoomX                float64
	ZoomY                float64
	Presets              []string
	Cachebuster          string
	Expires              int64
	RawMode              bool
	VideoThumbnailSecond float64
	FallbackImageURL     string
	SkipProcessing       []string
}

// NewDefaultProcessingOptions creates a baseline options struct with default values.
func NewDefaultProcessingOptions() ProcessingOptions {
	return ProcessingOptions{
		ResizeType:        ResizeFit,
		ResizingAlgorithm: "lanczos3",
		DPR:               1.0,
		Gravity:           GravityOption{Type: GravityCenter},
		ExtendGravity:     GravityOption{Type: GravityCenter},
		CropGravity:       GravityOption{Type: GravityCenter},
		WatermarkGravity:  GravityOption{Type: GravityCenter},
		AutoRotate:        true,
		BackgroundAlpha:   1.0,
		StripMetadata:     true,
		KeepCopyright:     true,
		StripColorProfile: true,
		ZoomX:             1.0,
		ZoomY:             1.0,
		FormatQuality:     make(map[string]int),
		SkipProcessing:    []string{},
	}
}

// InfoOptions contains flags for controlling the fields included in the /info JSON response.
type InfoOptions struct {
	Size               bool
	Format             bool
	Dimensions         bool
	EXIF               bool
	EXIFCanonicalNames bool
	XMP                bool
	VideoMeta          bool
	Colorspace         bool
	Bands              bool
	Pages              bool
	Alpha              bool
}

// NewDefaultInfoOptions creates default info options with standard fields enabled.
func NewDefaultInfoOptions() InfoOptions {
	return InfoOptions{
		Size:               true,
		Format:             true,
		Dimensions:         true,
		EXIF:               true,
		EXIFCanonicalNames: false,
		XMP:                true,
		VideoMeta:          false,
		Colorspace:         true,
		Bands:              true,
		Pages:              true,
		Alpha:              true,
	}
}

// ParseURLPath extracts the raw options string and resolved source URL from the trailing URL path.
func ParseURLPath(pathAfterSignature string) (string, string, string, error) {
	trimmedPath := strings.TrimPrefix(pathAfterSignature, "/")
	if trimmedPath == "" {
		return "", "", "", errors.New("empty image path")
	}

	var optionsString string
	var sourceURL string
	var extensionOverride string

	// 1. Plain URL handling: /{options}/plain/{source_url}@{ext}
	const plainDelimiter = "/plain/"
	if strings.Contains(trimmedPath, plainDelimiter) {
		splitParts := strings.SplitN(trimmedPath, plainDelimiter, 2)
		optionsString = splitParts[0]
		rawSource := strings.TrimSpace(splitParts[1])
		if rawSource == "" {
			return "", "", "", errors.New("missing source url in plain path")
		}

		if atIndex := strings.LastIndex(rawSource, "@"); atIndex != -1 {
			extensionOverride = strings.TrimPrefix(rawSource[atIndex+1:], ".")
			sourceURL = rawSource[:atIndex]
		} else {
			sourceURL = rawSource
		}

		decodedSourceURL, unescapeErr := url.PathUnescape(sourceURL)
		if unescapeErr == nil && strings.HasPrefix(decodedSourceURL, "http") {
			sourceURL = decodedSourceURL
		}

		return optionsString, sourceURL, extensionOverride, nil
	}

	if strings.HasPrefix(trimmedPath, "plain/") {
		rawSource := strings.TrimPrefix(trimmedPath, "plain/")
		rawSource = strings.TrimSpace(rawSource)
		if rawSource == "" {
			return "", "", "", errors.New("missing source url in plain path")
		}

		if atIndex := strings.LastIndex(rawSource, "@"); atIndex != -1 {
			extensionOverride = strings.TrimPrefix(rawSource[atIndex+1:], ".")
			sourceURL = rawSource[:atIndex]
		} else {
			sourceURL = rawSource
		}

		decodedSourceURL, unescapeErr := url.PathUnescape(sourceURL)
		if unescapeErr == nil && strings.HasPrefix(decodedSourceURL, "http") {
			sourceURL = decodedSourceURL
		}

		return "", sourceURL, extensionOverride, nil
	}

	// 2. Encoded URL handling: /{options}/{encoded_source_url}
	pathSegments := strings.Split(trimmedPath, "/")
	encodedSource := pathSegments[len(pathSegments)-1]

	if len(pathSegments) > 1 {
		optionsString = strings.Join(pathSegments[:len(pathSegments)-1], "/")
	}

	if atIndex := strings.LastIndex(encodedSource, "@"); atIndex != -1 {
		extensionOverride = strings.TrimPrefix(encodedSource[atIndex+1:], ".")
		encodedSource = encodedSource[:atIndex]
	} else if dotIndex := strings.LastIndex(encodedSource, "."); dotIndex != -1 {
		extensionOverride = encodedSource[dotIndex+1:]
		encodedSource = encodedSource[:dotIndex]
	}

	if encodedSource == "" {
		return "", "", "", errors.New("missing source url in encoded path")
	}

	// Decode URL-safe Base64 without padding (or with standard padding fallback)
	decodedBytes, decodeErr := base64.RawURLEncoding.DecodeString(encodedSource)
	if decodeErr != nil {
		decodedStandardBytes, standardDecodeErr := base64.URLEncoding.DecodeString(encodedSource)
		if standardDecodeErr != nil {
			decodedStdBytes, stdErr := base64.StdEncoding.DecodeString(encodedSource)
			if stdErr != nil {
				return "", "", "", fmt.Errorf("failed to decode base64 source url: %w", decodeErr)
			}
			sourceURL = string(decodedStdBytes)
		} else {
			sourceURL = string(decodedStandardBytes)
		}
	} else {
		sourceURL = string(decodedBytes)
	}

	if atIndex := strings.LastIndex(sourceURL, "@"); atIndex != -1 {
		extensionOverride = strings.TrimPrefix(sourceURL[atIndex+1:], ".")
		sourceURL = sourceURL[:atIndex]
	}

	return optionsString, sourceURL, extensionOverride, nil
}

// ParseProcessingOptions parses slash-separated options into a ProcessingOptions struct,
// recursively expanding any referenced presets.
func ParseProcessingOptions(optionsString string, presetResolver func(name string) (string, error)) (ProcessingOptions, error) {
	processingOptions := NewDefaultProcessingOptions()
	if optionsString == "" || optionsString == "_" {
		return processingOptions, nil
	}

	expandedSegments, expandErr := expandPresets(optionsString, presetResolver, 0)
	if expandErr != nil {
		return processingOptions, expandErr
	}

	for _, segment := range expandedSegments {
		segment = strings.TrimSpace(segment)
		if segment == "" || segment == "_" {
			continue
		}

		parts := strings.Split(segment, ":")
		optionName := parts[0]
		args := parts[1:]

		switch optionName {
		case "resize", "rs":
			parseResizeOption(&processingOptions, args)
		case "size", "s":
			parseSizeOption(&processingOptions, args)
		case "resizing_type", "rt":
			if len(args) > 0 {
				processingOptions.ResizeType = args[0]
			}
		case "resizing_algorithm", "ra":
			if len(args) > 0 {
				processingOptions.ResizingAlgorithm = args[0]
			}
		case "width", "w":
			if len(args) > 0 {
				processingOptions.Width, _ = strconv.Atoi(args[0])
			}
		case "height", "h":
			if len(args) > 0 {
				processingOptions.Height, _ = strconv.Atoi(args[0])
			}
		case "min_width", "min-width", "mw":
			if len(args) > 0 {
				processingOptions.MinWidth, _ = strconv.Atoi(args[0])
			}
		case "min_height", "min-height", "mh":
			if len(args) > 0 {
				processingOptions.MinHeight, _ = strconv.Atoi(args[0])
			}
		case "dpr":
			if len(args) > 0 {
				if parsedDPR, parseErr := strconv.ParseFloat(args[0], 64); parseErr == nil && parsedDPR > 0 {
					processingOptions.DPR = parsedDPR
				}
			}
		case "enlarge", "el":
			if len(args) > 0 {
				processingOptions.Enlarge = parseBooleanFlag(args[0])
			}
		case "extend", "ex":
			parseExtendOption(&processingOptions, args)
		case "gravity", "g":
			parseGravityOption(&processingOptions.Gravity, args)
		case "crop", "c":
			parseCropOption(&processingOptions, args)
		case "trim", "t":
			parseTrimOption(&processingOptions, args)
		case "padding", "pd":
			parsePaddingOption(&processingOptions, args)
		case "auto_rotate", "ar":
			if len(args) > 0 {
				processingOptions.AutoRotate = parseBooleanFlag(args[0])
			}
		case "rotate", "rot":
			if len(args) > 0 {
				processingOptions.Rotate, _ = strconv.Atoi(args[0])
			}
		case "background", "bg":
			if len(args) > 0 {
				processingOptions.Background = strings.Join(args, ":")
			}
		case "background_alpha", "bga":
			if len(args) > 0 {
				processingOptions.BackgroundAlpha, _ = strconv.ParseFloat(args[0], 64)
			}
		case "blur", "bl":
			if len(args) > 0 {
				processingOptions.Blur, _ = strconv.ParseFloat(args[0], 64)
			}
		case "sharpen", "sh":
			if len(args) > 0 {
				processingOptions.Sharpen, _ = strconv.ParseFloat(args[0], 64)
			}
		case "pixelate", "pix":
			if len(args) > 0 {
				processingOptions.Pixelate, _ = strconv.Atoi(args[0])
			}
		case "unsharpen", "ush":
			parseUnsharpenOption(&processingOptions, args)
		case "watermark", "wm":
			parseWatermarkOption(&processingOptions, args)
		case "watermark_url", "wmu":
			if len(args) > 0 {
				decodedURLBytes, decodeErr := base64.RawURLEncoding.DecodeString(args[0])
				if decodeErr == nil {
					processingOptions.WatermarkURL = string(decodedURLBytes)
				} else {
					processingOptions.WatermarkURL = args[0]
				}
			}
		case "format", "f", "ext":
			if len(args) > 0 {
				processingOptions.Format = strings.ToLower(args[0])
			}
		case "quality", "q":
			if len(args) > 0 {
				processingOptions.Quality, _ = strconv.Atoi(args[0])
			}
		case "format_quality", "fq":
			parseFormatQualityOption(&processingOptions, args)
		case "autoquality", "aq":
			parseAutoQualityOption(&processingOptions, args)
		case "strip_metadata", "sm":
			if len(args) > 0 {
				processingOptions.StripMetadata = parseBooleanFlag(args[0])
			}
		case "keep_copyright", "kcr":
			if len(args) > 0 {
				processingOptions.KeepCopyright = parseBooleanFlag(args[0])
			}
		case "strip_color_profile", "scp":
			if len(args) > 0 {
				processingOptions.StripColorProfile = parseBooleanFlag(args[0])
			}
		case "zoom", "z":
			parseZoomOption(&processingOptions, args)
		case "cachebuster", "cb":
			if len(args) > 0 {
				processingOptions.Cachebuster = args[0]
			}
		case "expires", "exp":
			if len(args) > 0 {
				processingOptions.Expires, _ = strconv.ParseInt(args[0], 10, 64)
			}
		case "raw":
			if len(args) > 0 {
				processingOptions.RawMode = parseBooleanFlag(args[0])
			}
		case "video_thumbnail_second", "vts":
			if len(args) > 0 {
				processingOptions.VideoThumbnailSecond, _ = strconv.ParseFloat(args[0], 64)
			}
		case "fallback_image_url", "fiu":
			if len(args) > 0 {
				decodedFallbackBytes, decodeErr := base64.RawURLEncoding.DecodeString(args[0])
				if decodeErr == nil {
					processingOptions.FallbackImageURL = string(decodedFallbackBytes)
				} else {
					processingOptions.FallbackImageURL = args[0]
				}
			}
		case "skip_processing", "skp":
			processingOptions.SkipProcessing = append(processingOptions.SkipProcessing, args...)
		}
	}

	if processingOptions.Expires > 0 && time.Now().Unix() > processingOptions.Expires {
		return processingOptions, fmt.Errorf("image request expired at timestamp %d", processingOptions.Expires)
	}

	return processingOptions, nil
}

// ParseInfoOptions parses slash-separated info option segments.
func ParseInfoOptions(optionsString string) InfoOptions {
	infoOptions := NewDefaultInfoOptions()
	if optionsString == "" || optionsString == "_" {
		return infoOptions
	}

	segments := strings.Split(optionsString, "/")
	for _, segment := range segments {
		parts := strings.Split(segment, ":")
		name := parts[0]
		args := parts[1:]
		enabled := true
		if len(args) > 0 {
			enabled = parseBooleanFlag(args[0])
		}

		switch name {
		case "size", "s":
			infoOptions.Size = enabled
		case "format", "f":
			infoOptions.Format = enabled
		case "dimensions", "d":
			infoOptions.Dimensions = enabled
		case "exif":
			infoOptions.EXIF = enabled
			if len(args) > 1 {
				infoOptions.EXIFCanonicalNames = parseBooleanFlag(args[1])
			}
		case "xmp":
			infoOptions.XMP = enabled
		case "video_meta":
			infoOptions.VideoMeta = enabled
		case "colorspace":
			infoOptions.Colorspace = enabled
		case "bands":
			infoOptions.Bands = enabled
		case "pages":
			infoOptions.Pages = enabled
		case "alpha":
			infoOptions.Alpha = enabled
		}
	}

	return infoOptions
}

func parseResizeOption(processingOptions *ProcessingOptions, args []string) {
	if len(args) > 0 && args[0] != "" {
		processingOptions.ResizeType = args[0]
	}
	if len(args) > argCountOne && args[1] != "" {
		processingOptions.Width, _ = strconv.Atoi(args[1])
	}
	if len(args) > argCountTwo && args[2] != "" {
		processingOptions.Height, _ = strconv.Atoi(args[2])
	}
	if len(args) > argCountThree && args[3] != "" {
		processingOptions.Enlarge = parseBooleanFlag(args[3])
	}
	if len(args) > argCountFour && args[4] != "" {
		processingOptions.Extend = parseBooleanFlag(args[4])
	}
}

func parseSizeOption(processingOptions *ProcessingOptions, args []string) {
	processingOptions.ResizeType = ResizeFit
	if len(args) > 0 && args[0] != "" {
		processingOptions.Width, _ = strconv.Atoi(args[0])
	}
	if len(args) > argCountOne && args[1] != "" {
		processingOptions.Height, _ = strconv.Atoi(args[1])
	}
	if len(args) > argCountTwo && args[2] != "" {
		processingOptions.Enlarge = parseBooleanFlag(args[2])
	}
	if len(args) > argCountThree && args[3] != "" {
		processingOptions.Extend = parseBooleanFlag(args[3])
	}
}

func parseExtendOption(processingOptions *ProcessingOptions, args []string) {
	if len(args) > 0 {
		processingOptions.Extend = parseBooleanFlag(args[0])
	}
	if len(args) > argCountOne {
		parseGravityOption(&processingOptions.ExtendGravity, args[1:])
	}
}

func parseGravityOption(gravityOption *GravityOption, args []string) {
	if len(args) == 0 {
		return
	}
	gravityOption.Type = args[0]
	if gravityOption.Type == GravityFocus {
		if len(args) > argCountOne {
			gravityOption.XOffset, _ = strconv.ParseFloat(args[1], 64)
		}
		if len(args) > argCountTwo {
			gravityOption.YOffset, _ = strconv.ParseFloat(args[2], 64)
		}
		return
	}
	if len(args) > argCountOne {
		gravityOption.XOffset, _ = strconv.ParseFloat(args[1], 64)
	}
	if len(args) > argCountTwo {
		gravityOption.YOffset, _ = strconv.ParseFloat(args[2], 64)
	}
}

func parseCropOption(processingOptions *ProcessingOptions, args []string) {
	if len(args) > 0 {
		processingOptions.CropWidth, _ = strconv.ParseFloat(args[0], 64)
	}
	if len(args) > argCountOne {
		processingOptions.CropHeight, _ = strconv.ParseFloat(args[1], 64)
	}
	if len(args) > argCountTwo {
		parseGravityOption(&processingOptions.CropGravity, args[2:])
	}
}

func parseTrimOption(processingOptions *ProcessingOptions, args []string) {
	if len(args) > 0 {
		processingOptions.TrimThreshold, _ = strconv.ParseFloat(args[0], 64)
	}
	if len(args) > argCountOne {
		processingOptions.TrimColor = args[1]
	}
	if len(args) > argCountTwo {
		processingOptions.TrimEqualHor = parseBooleanFlag(args[2])
	}
	if len(args) > argCountThree {
		processingOptions.TrimEqualVer = parseBooleanFlag(args[3])
	}
}

func parsePaddingOption(processingOptions *ProcessingOptions, args []string) {
	switch len(args) {
	case argCountOne:
		padding, _ := strconv.Atoi(args[0])
		processingOptions.PaddingTop = padding
		processingOptions.PaddingRight = padding
		processingOptions.PaddingBottom = padding
		processingOptions.PaddingLeft = padding
	case argCountTwo:
		vertical, _ := strconv.Atoi(args[0])
		horizontal, _ := strconv.Atoi(args[1])
		processingOptions.PaddingTop = vertical
		processingOptions.PaddingBottom = vertical
		processingOptions.PaddingRight = horizontal
		processingOptions.PaddingLeft = horizontal
	case argCountFour:
		processingOptions.PaddingTop, _ = strconv.Atoi(args[0])
		processingOptions.PaddingRight, _ = strconv.Atoi(args[1])
		processingOptions.PaddingBottom, _ = strconv.Atoi(args[2])
		processingOptions.PaddingLeft, _ = strconv.Atoi(args[3])
	}
}

func parseUnsharpenOption(processingOptions *ProcessingOptions, args []string) {
	if len(args) > 0 {
		processingOptions.UnsharpenMode, _ = strconv.ParseFloat(args[0], 64)
	}
	if len(args) > argCountOne {
		processingOptions.UnsharpenWeight, _ = strconv.ParseFloat(args[1], 64)
	}
	if len(args) > argCountTwo {
		processingOptions.UnsharpenDividor, _ = strconv.ParseFloat(args[2], 64)
	}
	if len(args) > argCountThree {
		processingOptions.UnsharpenThreshold, _ = strconv.ParseFloat(args[3], 64)
	}
}

func parseWatermarkOption(processingOptions *ProcessingOptions, args []string) {
	if len(args) > 0 {
		processingOptions.WatermarkOpacity, _ = strconv.ParseFloat(args[0], 64)
	}
	if len(args) > argCountOne {
		parseGravityOption(&processingOptions.WatermarkGravity, []string{args[1]})
	}
	if len(args) > argCountTwo {
		processingOptions.WatermarkXOffset, _ = strconv.Atoi(args[2])
	}
	if len(args) > argCountThree {
		processingOptions.WatermarkYOffset, _ = strconv.Atoi(args[3])
	}
	if len(args) > argCountFour {
		processingOptions.WatermarkScale, _ = strconv.ParseFloat(args[4], 64)
	}
	if len(args) > argCountFive {
		decodedURLBytes, decodeErr := base64.RawURLEncoding.DecodeString(args[5])
		if decodeErr == nil {
			processingOptions.WatermarkURL = string(decodedURLBytes)
		} else {
			processingOptions.WatermarkURL = args[5]
		}
	}
}

func parseFormatQualityOption(processingOptions *ProcessingOptions, args []string) {
	for i := 0; i+1 < len(args); i += argCountTwo {
		format := strings.ToLower(args[i])
		quality, parseErr := strconv.Atoi(args[i+1])
		if parseErr == nil {
			processingOptions.FormatQuality[format] = quality
		}
	}
}

func parseAutoQualityOption(processingOptions *ProcessingOptions, args []string) {
	if len(args) > 0 {
		processingOptions.AutoQualityMethod = args[0]
	}
	if len(args) > argCountOne {
		processingOptions.AutoQualityTarget, _ = strconv.ParseFloat(args[1], 64)
	}
	if len(args) > argCountTwo {
		processingOptions.AutoQualityMin, _ = strconv.Atoi(args[2])
	}
	if len(args) > argCountThree {
		processingOptions.AutoQualityMax, _ = strconv.Atoi(args[3])
	}
}

func parseZoomOption(processingOptions *ProcessingOptions, args []string) {
	if len(args) > 0 {
		zoomFactor, parseErr := strconv.ParseFloat(args[0], 64)
		if parseErr == nil {
			processingOptions.ZoomX = zoomFactor
			processingOptions.ZoomY = zoomFactor
		}
	}
	if len(args) > argCountOne {
		zoomYFactor, parseErr := strconv.ParseFloat(args[1], 64)
		if parseErr == nil {
			processingOptions.ZoomY = zoomYFactor
		}
	}
}

func parseBooleanFlag(value string) bool {
	lower := strings.ToLower(strings.TrimSpace(value))
	return lower == "1" || lower == "t" || lower == "true" || lower == "yes"
}

func expandPresets(optionsString string, presetResolver func(name string) (string, error), depth int) ([]string, error) {
	if depth > maxPresetRecursionDepth {
		return nil, errors.New("maximum preset recursion depth exceeded")
	}

	var result []string
	segments := strings.Split(optionsString, "/")

	for _, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			continue
		}

		if strings.HasPrefix(segment, "preset:") || strings.HasPrefix(segment, "pr:") {
			parts := strings.Split(segment, ":")
			presetNames := parts[1:]
			for _, presetName := range presetNames {
				if presetResolver == nil {
					return nil, fmt.Errorf("preset resolver unavailable for preset %q", presetName)
				}
				resolvedOptions, resolveErr := presetResolver(presetName)
				if resolveErr != nil {
					return nil, fmt.Errorf("failed to resolve preset %q: %w", presetName, resolveErr)
				}
				nestedSegments, expandErr := expandPresets(resolvedOptions, presetResolver, depth+1)
				if expandErr != nil {
					return nil, expandErr
				}
				result = append(result, nestedSegments...)
			}
			continue
		}

		result = append(result, segment)
	}

	return result, nil
}
