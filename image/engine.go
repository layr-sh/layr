package image

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strconv"
	"strings"

	"github.com/KarpelesLab/gowebp"
	"github.com/disintegration/imaging"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
)

// Named constants for color and dimension parsing.
const (
	defaultQualityFallback = 80
	fullAlphaChannelValue  = 255
	fullAlphaMultiplier    = 255.0
	hexComponentBase       = 16
	hexComponentBitSize    = 8
	decimalComponentBase   = 10
	rotateAngle90          = 90
	rotateAngle180         = 180
	rotateAngle270         = 270
	rgbComponentLength3    = 3
	rgbComponentLength6    = 6
	rgbColonPartsCount     = 3
)

// Engine performs pure-Go image transformations, decodings, encodings, and metadata introspection.
type Engine struct {
	configManager *ConfigManager
}

// NewEngine initializes an image processing engine.
func NewEngine(configManager *ConfigManager) *Engine {
	return &Engine{
		configManager: configManager,
	}
}

// Transform applies the specified processing options to raw input image bytes.
func (engine *Engine) Transform(inputReader io.Reader, processingOptions ProcessingOptions) ([]byte, string, error) {
	inputBytes, readErr := io.ReadAll(inputReader)
	if readErr != nil {
		return nil, "", fmt.Errorf("failed reading image input stream: %w", readErr)
	}

	sourceImage, detectedFormat, decodeErr := engine.decodeImage(bytes.NewReader(inputBytes))
	if decodeErr != nil {
		return nil, "", fmt.Errorf("failed to decode source image: %w", decodeErr)
	}

	// 1. Raw Mode Passthrough Check
	targetFormat := strings.ToLower(processingOptions.Format)
	if targetFormat == "" {
		targetFormat = detectedFormat
	}
	if processingOptions.RawMode && (targetFormat == detectedFormat) {
		return inputBytes, resolveMIMEType(targetFormat), nil
	}

	// 2. Skip Processing Check
	for _, skipExt := range processingOptions.SkipProcessing {
		if strings.EqualFold(skipExt, detectedFormat) {
			return inputBytes, resolveMIMEType(detectedFormat), nil
		}
	}

	workingNRGBA := imaging.Clone(sourceImage)

	// 3. Rotation
	if processingOptions.Rotate != 0 {
		workingNRGBA = rotateImage(workingNRGBA, processingOptions.Rotate)
	}

	// 4. Crop
	if processingOptions.CropWidth > 0 && processingOptions.CropHeight > 0 {
		workingNRGBA = cropImage(workingNRGBA, processingOptions.CropWidth, processingOptions.CropHeight, processingOptions.CropGravity)
	}

	// 5. Resize
	if processingOptions.Width > 0 || processingOptions.Height > 0 {
		workingNRGBA = resizeImage(workingNRGBA, processingOptions)
	}

	// 6. Padding
	if processingOptions.PaddingTop > 0 || processingOptions.PaddingRight > 0 || processingOptions.PaddingBottom > 0 || processingOptions.PaddingLeft > 0 {
		workingNRGBA = padImage(workingNRGBA, processingOptions)
	}

	// 7. Blur
	if processingOptions.Blur > 0 {
		workingNRGBA = imaging.Blur(workingNRGBA, processingOptions.Blur)
	}

	// 8. Sharpen
	if processingOptions.Sharpen > 0 {
		workingNRGBA = imaging.Sharpen(workingNRGBA, processingOptions.Sharpen)
	}

	// 9. Pixelate
	if processingOptions.Pixelate > 1 {
		workingNRGBA = pixelateImage(workingNRGBA, processingOptions.Pixelate)
	}

	// 10. Watermark
	if processingOptions.WatermarkURL != "" {
		watermarkedNRGBA, watermarkErr := engine.applyWatermark(workingNRGBA, processingOptions)
		if watermarkErr == nil {
			workingNRGBA = watermarkedNRGBA
		}
	}

	// 11. Format Encoding
	outputBytes, encodeErr := encodeImage(workingNRGBA, targetFormat, processingOptions.Quality, engine.configManager.Get())
	if encodeErr != nil {
		return nil, "", fmt.Errorf("failed to encode output image to %s: %w", targetFormat, encodeErr)
	}

	return outputBytes, resolveMIMEType(targetFormat), nil
}

// Inspect introspects the image input stream and returns detailed dimensions and format metadata.
func (engine *Engine) Inspect(inputReader io.Reader, infoOptions InfoOptions) (*GetInfoResponse, error) {
	bufferedBytes, readErr := io.ReadAll(inputReader)
	if readErr != nil {
		return nil, fmt.Errorf("failed reading image stream for inspection: %w", readErr)
	}

	imageConfig, formatName, configErr := image.DecodeConfig(bytes.NewReader(bufferedBytes))
	if configErr != nil {
		return nil, fmt.Errorf("failed to introspect image metadata: %w", configErr)
	}

	hasAlpha := true
	switch formatName {
	case FormatJPEG, FormatJPG:
		hasAlpha = false
	}

	getInfoResponse := &GetInfoResponse{
		Orientation: 1,
		Colorspace:  "sRGB",
		Bands:       3,
		Pages:       1,
		Alpha:       &hasAlpha,
		EXIF:        map[string]any{},
		VideoMeta:   map[string]any{},
	}

	if infoOptions.Size {
		getInfoResponse.Size = int64(len(bufferedBytes))
	}
	if infoOptions.Format {
		getInfoResponse.Format = formatName
		getInfoResponse.MIMEType = resolveMIMEType(formatName)
	}
	if infoOptions.Dimensions {
		getInfoResponse.Width = imageConfig.Width
		getInfoResponse.Height = imageConfig.Height
	}
	if !infoOptions.Alpha {
		getInfoResponse.Alpha = nil
	}
	if !infoOptions.Colorspace {
		getInfoResponse.Colorspace = ""
	}
	if !infoOptions.Bands {
		getInfoResponse.Bands = 0
	}
	if !infoOptions.Pages {
		getInfoResponse.Pages = 0
	}

	return getInfoResponse, nil
}

func (engine *Engine) decodeImage(inputReader io.Reader) (image.Image, string, error) {
	decodedImage, formatName, decodeErr := image.Decode(inputReader)
	if decodeErr != nil {
		return nil, "", fmt.Errorf("unsupported or corrupted image format: %w", decodeErr)
	}

	return decodedImage, strings.ToLower(formatName), nil
}

func resizeImage(sourceNRGBA *image.NRGBA, processingOptions ProcessingOptions) *image.NRGBA {
	boundsRectangle := sourceNRGBA.Bounds()
	sourceWidth := boundsRectangle.Dx()
	sourceHeight := boundsRectangle.Dy()

	effectiveDPR := processingOptions.DPR
	if effectiveDPR <= 0 {
		effectiveDPR = 1.0
	}

	targetWidth := int(float64(processingOptions.Width) * effectiveDPR)
	targetHeight := int(float64(processingOptions.Height) * effectiveDPR)

	if !processingOptions.Enlarge {
		if targetWidth > sourceWidth && targetWidth > 0 {
			targetWidth = sourceWidth
		}
		if targetHeight > sourceHeight && targetHeight > 0 {
			targetHeight = sourceHeight
		}
	}

	resampleFilter := resolveResampleFilter(processingOptions.ResizingAlgorithm)
	anchor := resolveImagingAnchor(processingOptions.Gravity)

	switch processingOptions.ResizeType {
	case ResizeFit:
		if targetWidth > 0 && targetHeight > 0 {
			return imaging.Fit(sourceNRGBA, targetWidth, targetHeight, resampleFilter)
		}
		return imaging.Resize(sourceNRGBA, targetWidth, targetHeight, resampleFilter)

	case ResizeFill, ResizeFillDown:
		if targetWidth > 0 && targetHeight > 0 {
			return imaging.Fill(sourceNRGBA, targetWidth, targetHeight, anchor, resampleFilter)
		}
		return imaging.Resize(sourceNRGBA, targetWidth, targetHeight, resampleFilter)

	case ResizeForce:
		return imaging.Resize(sourceNRGBA, targetWidth, targetHeight, resampleFilter)

	default:
		return imaging.Fit(sourceNRGBA, targetWidth, targetHeight, resampleFilter)
	}
}

func cropImage(sourceNRGBA *image.NRGBA, cropWidth float64, cropHeight float64, gravityOption GravityOption) *image.NRGBA {
	boundsRectangle := sourceNRGBA.Bounds()
	targetWidth := int(cropWidth)
	targetHeight := int(cropHeight)

	if cropWidth > 0 && cropWidth <= 1.0 {
		targetWidth = int(float64(boundsRectangle.Dx()) * cropWidth)
	}
	if cropHeight > 0 && cropHeight <= 1.0 {
		targetHeight = int(float64(boundsRectangle.Dy()) * cropHeight)
	}

	anchor := resolveImagingAnchor(gravityOption)
	return imaging.CropAnchor(sourceNRGBA, targetWidth, targetHeight, anchor)
}

func padImage(sourceNRGBA *image.NRGBA, processingOptions ProcessingOptions) *image.NRGBA {
	boundsRectangle := sourceNRGBA.Bounds()
	newWidth := boundsRectangle.Dx() + processingOptions.PaddingLeft + processingOptions.PaddingRight
	newHeight := boundsRectangle.Dy() + processingOptions.PaddingTop + processingOptions.PaddingBottom

	backgroundColor := parseColorHex(processingOptions.Background, processingOptions.BackgroundAlpha)
	extendedNRGBA := imaging.New(newWidth, newHeight, backgroundColor)

	return imaging.Paste(extendedNRGBA, sourceNRGBA, image.Pt(processingOptions.PaddingLeft, processingOptions.PaddingTop))
}

func rotateImage(sourceNRGBA *image.NRGBA, angle int) *image.NRGBA {
	normalizedAngle := (angle%360 + 360) % 360
	switch normalizedAngle {
	case rotateAngle90:
		return imaging.Rotate270(sourceNRGBA)
	case rotateAngle180:
		return imaging.Rotate180(sourceNRGBA)
	case rotateAngle270:
		return imaging.Rotate90(sourceNRGBA)
	default:
		return sourceNRGBA
	}
}

func pixelateImage(sourceNRGBA *image.NRGBA, pixelSize int) *image.NRGBA {
	boundsRectangle := sourceNRGBA.Bounds()
	if boundsRectangle.Dx() <= pixelSize || boundsRectangle.Dy() <= pixelSize {
		return sourceNRGBA
	}

	smallWidth := boundsRectangle.Dx() / pixelSize
	smallHeight := boundsRectangle.Dy() / pixelSize

	downscaledNRGBA := imaging.Resize(sourceNRGBA, smallWidth, smallHeight, imaging.NearestNeighbor)
	return imaging.Resize(downscaledNRGBA, boundsRectangle.Dx(), boundsRectangle.Dy(), imaging.NearestNeighbor)
}

func (engine *Engine) applyWatermark(sourceNRGBA *image.NRGBA, processingOptions ProcessingOptions) (*image.NRGBA, error) {
	watermarkImage, _, decodeErr := engine.decodeImage(strings.NewReader(processingOptions.WatermarkURL))
	if decodeErr != nil {
		return nil, decodeErr
	}

	watermarkNRGBA := imaging.Clone(watermarkImage)
	if processingOptions.WatermarkScale > 0 {
		scaledWidth := int(float64(sourceNRGBA.Bounds().Dx()) * processingOptions.WatermarkScale)
		if scaledWidth > 0 {
			watermarkNRGBA = imaging.Resize(watermarkNRGBA, scaledWidth, 0, imaging.Lanczos)
		}
	}

	opacity := processingOptions.WatermarkOpacity
	if opacity <= 0 {
		opacity = 1.0
	}

	return imaging.Overlay(sourceNRGBA, watermarkNRGBA, image.Pt(processingOptions.WatermarkXOffset, processingOptions.WatermarkYOffset), opacity), nil
}

func encodeImage(sourceImage image.Image, format string, quality int, runtimeConfig Config) ([]byte, error) {
	effectiveQuality := quality
	if effectiveQuality <= 0 {
		effectiveQuality = runtimeConfig.DefaultQuality
	}
	if effectiveQuality <= 0 {
		effectiveQuality = defaultQualityFallback
	}

	var outputBuffer bytes.Buffer

	switch format {
	case FormatWebP:
		encodeErr := gowebp.Encode(&outputBuffer, sourceImage, &gowebp.Options{
			Quality: float32(effectiveQuality),
		})
		return outputBuffer.Bytes(), encodeErr

	case FormatPNG:
		encodeErr := png.Encode(&outputBuffer, sourceImage)
		return outputBuffer.Bytes(), encodeErr

	case FormatGIF:
		encodeErr := gif.Encode(&outputBuffer, sourceImage, nil)
		return outputBuffer.Bytes(), encodeErr

	case FormatBMP:
		encodeErr := bmp.Encode(&outputBuffer, sourceImage)
		return outputBuffer.Bytes(), encodeErr

	case FormatTIFF:
		encodeErr := tiff.Encode(&outputBuffer, sourceImage, nil)
		return outputBuffer.Bytes(), encodeErr

	case FormatJPEG, FormatJPG, "":
		encodeErr := jpeg.Encode(&outputBuffer, sourceImage, &jpeg.Options{
			Quality: effectiveQuality,
		})
		return outputBuffer.Bytes(), encodeErr

	default:
		return nil, fmt.Errorf("unsupported output format: %q", format)
	}
}

func resolveResampleFilter(algorithm string) imaging.ResampleFilter {
	switch strings.ToLower(algorithm) {
	case "nearest":
		return imaging.NearestNeighbor
	case "linear":
		return imaging.Linear
	case "cubic":
		return imaging.CatmullRom
	case "lanczos2":
		return imaging.Lanczos
	case "lanczos3":
		return imaging.Lanczos
	default:
		return imaging.Lanczos
	}
}

func resolveImagingAnchor(gravityOption GravityOption) imaging.Anchor {
	switch gravityOption.Type {
	case GravityNorth:
		return imaging.Top
	case GravitySouth:
		return imaging.Bottom
	case GravityEast:
		return imaging.Right
	case GravityWest:
		return imaging.Left
	case GravityNorthEast:
		return imaging.TopRight
	case GravityNorthWest:
		return imaging.TopLeft
	case GravitySouthEast:
		return imaging.BottomRight
	case GravitySouthWest:
		return imaging.BottomLeft
	case GravityCenter, GravitySmart, GravityFocus:
		return imaging.Center
	default:
		return imaging.Center
	}
}

func parseColorHex(colorHex string, alpha float64) color.Color {
	trimmedHex := strings.TrimPrefix(strings.TrimSpace(colorHex), "#")
	if trimmedHex == "" {
		return color.Transparent
	}

	alphaByte := uint8(alpha * fullAlphaMultiplier)
	if alpha <= 0 {
		alphaByte = fullAlphaChannelValue
	}

	if len(trimmedHex) == rgbComponentLength6 {
		r, _ := strconv.ParseUint(trimmedHex[0:2], hexComponentBase, hexComponentBitSize)
		g, _ := strconv.ParseUint(trimmedHex[2:4], hexComponentBase, hexComponentBitSize)
		b, _ := strconv.ParseUint(trimmedHex[4:6], hexComponentBase, hexComponentBitSize)
		return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: alphaByte}
	}

	if len(trimmedHex) == rgbComponentLength3 {
		r, _ := strconv.ParseUint(string([]byte{trimmedHex[0], trimmedHex[0]}), hexComponentBase, hexComponentBitSize)
		g, _ := strconv.ParseUint(string([]byte{trimmedHex[1], trimmedHex[1]}), hexComponentBase, hexComponentBitSize)
		b, _ := strconv.ParseUint(string([]byte{trimmedHex[2], trimmedHex[2]}), hexComponentBase, hexComponentBitSize)
		return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: alphaByte}
	}

	// Check RGB colon format: 255:255:255
	if strings.Contains(colorHex, ":") {
		parts := strings.Split(colorHex, ":")
		if len(parts) == rgbColonPartsCount {
			r, _ := strconv.ParseUint(parts[0], decimalComponentBase, hexComponentBitSize)
			g, _ := strconv.ParseUint(parts[1], decimalComponentBase, hexComponentBitSize)
			b, _ := strconv.ParseUint(parts[2], decimalComponentBase, hexComponentBitSize)
			return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: alphaByte}
		}
	}

	return color.Transparent
}

func resolveMIMEType(format string) string {
	switch strings.ToLower(format) {
	case FormatWebP:
		return "image/webp"
	case FormatAVIF:
		return "image/avif"
	case FormatPNG:
		return "image/png"
	case FormatJPEG, FormatJPG:
		return "image/jpeg"
	case FormatGIF:
		return "image/gif"
	case FormatSVG:
		return "image/svg+xml"
	case FormatICO:
		return "image/x-icon"
	case FormatTIFF:
		return "image/tiff"
	case FormatBMP:
		return "image/bmp"
	default:
		return "application/octet-stream"
	}
}
