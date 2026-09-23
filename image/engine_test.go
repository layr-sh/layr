package image

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/png"
	"testing"

	"github.com/KarpelesLab/gowebp"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"layr.sh/core"
)

func createTestImagePNG(width int, height int) []byte {
	targetRectangle := image.Rect(0, 0, width, height)
	targetRGBA := image.NewRGBA(targetRectangle)
	for xCoordinate := 0; xCoordinate < width; xCoordinate++ {
		for yCoordinate := 0; yCoordinate < height; yCoordinate++ {
			targetRGBA.Set(xCoordinate, yCoordinate, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, targetRGBA)
	return buffer.Bytes()
}

func createTestImageBMP(width int, height int) []byte {
	targetRectangle := image.Rect(0, 0, width, height)
	targetRGBA := image.NewRGBA(targetRectangle)
	for xCoordinate := 0; xCoordinate < width; xCoordinate++ {
		for yCoordinate := 0; yCoordinate < height; yCoordinate++ {
			targetRGBA.Set(xCoordinate, yCoordinate, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buffer bytes.Buffer
	_ = bmp.Encode(&buffer, targetRGBA)
	return buffer.Bytes()
}

func createTestImageTIFF(width int, height int) []byte {
	targetRectangle := image.Rect(0, 0, width, height)
	targetRGBA := image.NewRGBA(targetRectangle)
	for xCoordinate := 0; xCoordinate < width; xCoordinate++ {
		for yCoordinate := 0; yCoordinate < height; yCoordinate++ {
			targetRGBA.Set(xCoordinate, yCoordinate, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buffer bytes.Buffer
	_ = tiff.Encode(&buffer, targetRGBA, nil)
	return buffer.Bytes()
}

func createTestImageWebP(width int, height int) []byte {
	targetRectangle := image.Rect(0, 0, width, height)
	targetRGBA := image.NewRGBA(targetRectangle)
	for xCoordinate := 0; xCoordinate < width; xCoordinate++ {
		for yCoordinate := 0; yCoordinate < height; yCoordinate++ {
			targetRGBA.Set(xCoordinate, yCoordinate, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	var buffer bytes.Buffer
	_ = gowebp.Encode(&buffer, targetRGBA, &gowebp.Options{Quality: 80})
	return buffer.Bytes()
}

func TestImageEngineUnit(t *testing.T) {
	t.Parallel()

	kernel := core.NewTestKernel(nil)
	configManager := NewConfigManager(kernel)
	engine := NewEngine(configManager)

	pngBytes := createTestImagePNG(200, 100)

	t.Run("inspect image metadata", func(t *testing.T) {
		t.Parallel()
		reader := bytes.NewReader(pngBytes)
		infoOptions := NewDefaultInfoOptions()

		info, inspectErr := engine.Inspect(reader, infoOptions)
		require.NoError(t, inspectErr)
		require.NotNil(t, info)
		require.Equal(t, 200, info.Width)
		require.Equal(t, 100, info.Height)
		require.Equal(t, FormatPNG, info.Format)
		require.Equal(t, "image/png", info.MIMEType)
	})

	t.Run("transform resize and convert to webp", func(t *testing.T) {
		t.Parallel()
		reader := bytes.NewReader(pngBytes)
		processingOptions := ProcessingOptions{
			Width:      50,
			Height:     50,
			ResizeType: "fill",
			Format:     FormatWebP,
			Quality:    80,
		}

		outputBytes, outputContentType, transformErr := engine.Transform(reader, processingOptions)
		require.NoError(t, transformErr)
		require.NotEmpty(t, outputBytes)
		require.Equal(t, "image/webp", outputContentType)

		// Verify transformed dimensions
		transformedReader := bytes.NewReader(outputBytes)
		transformedInfo, inspectErr := engine.Inspect(transformedReader, NewDefaultInfoOptions())
		require.NoError(t, inspectErr)
		require.Equal(t, 50, transformedInfo.Width)
		require.Equal(t, 50, transformedInfo.Height)
	})

	t.Run("transform blur sharpen and rotate", func(t *testing.T) {
		t.Parallel()
		reader := bytes.NewReader(pngBytes)
		processingOptions := ProcessingOptions{
			Rotate:     90,
			Blur:       1.5,
			Sharpen:    1.0,
			Pixelate:   4,
			Format:     FormatPNG,
			ResizeType: "fit",
		}

		outputBytes, outputContentType, transformErr := engine.Transform(reader, processingOptions)
		require.NoError(t, transformErr)
		require.NotEmpty(t, outputBytes)
		require.Equal(t, "image/png", outputContentType)

		// Check rotated dimensions (width: 200, height: 100 -> rotated 90: width: 100, height: 200)
		transformedReader := bytes.NewReader(outputBytes)
		transformedInfo, inspectErr := engine.Inspect(transformedReader, NewDefaultInfoOptions())
		require.NoError(t, inspectErr)
		require.Equal(t, 100, transformedInfo.Width)
		require.Equal(t, 200, transformedInfo.Height)
	})

	t.Run("transform crop padding and format conversions", func(t *testing.T) {
		t.Parallel()

		formats := []string{FormatJPEG, FormatGIF, FormatBMP, FormatTIFF}
		for _, targetFormat := range formats {
			reader := bytes.NewReader(pngBytes)
			processingOptions := ProcessingOptions{
				Width:         80,
				Height:        80,
				CropWidth:     60,
				CropHeight:    60,
				PaddingTop:    5,
				PaddingBottom: 5,
				PaddingLeft:   5,
				PaddingRight:  5,
				Background:    "00ff00",
				Format:        targetFormat,
				ResizeType:    "fit",
			}

			outputBytes, outputContentType, transformErr := engine.Transform(reader, processingOptions)
			require.NoError(t, transformErr)
			require.NotEmpty(t, outputBytes)
			require.NotEmpty(t, outputContentType)

			inspectReader := bytes.NewReader(outputBytes)
			info, inspectErr := engine.Inspect(inspectReader, NewDefaultInfoOptions())
			require.NoError(t, inspectErr)
			require.NotNil(t, info)
		}
	})

	t.Run("raw mode and skip processing", func(t *testing.T) {
		t.Parallel()
		rawProcessingOptions := ProcessingOptions{
			RawMode: true,
			Format:  FormatPNG,
		}
		rawBytes, rawMIME, rawErr := engine.Transform(bytes.NewReader(pngBytes), rawProcessingOptions)
		require.NoError(t, rawErr)
		require.Equal(t, pngBytes, rawBytes)
		require.Equal(t, "image/png", rawMIME)

		skipProcessingOptions := ProcessingOptions{
			SkipProcessing: []string{"png"},
		}
		skipBytes, skipMIME, skipErr := engine.Transform(bytes.NewReader(pngBytes), skipProcessingOptions)
		require.NoError(t, skipErr)
		require.Equal(t, pngBytes, skipBytes)
		require.Equal(t, "image/png", skipMIME)
	})

	t.Run("rotation angles 180 270 and normalize", func(t *testing.T) {
		t.Parallel()
		for _, angle := range []int{180, 270, 360, -90} {
			rotationProcessingOptions := ProcessingOptions{
				Rotate: angle,
				Format: FormatPNG,
			}
			rotatedBytes, _, transformErr := engine.Transform(bytes.NewReader(pngBytes), rotationProcessingOptions)
			require.NoError(t, transformErr)
			require.NotEmpty(t, rotatedBytes)
		}
	})

	t.Run("relative crop and pixelate boundaries", func(t *testing.T) {
		t.Parallel()
		relativeCropProcessingOptions := ProcessingOptions{
			CropWidth:  0.5,
			CropHeight: 0.5,
			Format:     FormatPNG,
		}
		croppedBytes, _, transformErr := engine.Transform(bytes.NewReader(pngBytes), relativeCropProcessingOptions)
		require.NoError(t, transformErr)
		require.NotEmpty(t, croppedBytes)

		// Pixelate when pixelSize >= image width
		hugePixelateProcessingOptions := ProcessingOptions{
			Pixelate: 500,
			Format:   FormatPNG,
		}
		pixelatedBytes, _, pixelateErr := engine.Transform(bytes.NewReader(pngBytes), hugePixelateProcessingOptions)
		require.NoError(t, pixelateErr)
		require.NotEmpty(t, pixelatedBytes)
	})

	t.Run("watermark application and fallback", func(t *testing.T) {
		t.Parallel()
		watermarkPNG := createTestImagePNG(20, 20)
		watermarkProcessingOptions := ProcessingOptions{
			WatermarkURL:     string(watermarkPNG),
			WatermarkScale:   0.2,
			WatermarkOpacity: 0.5,
			WatermarkXOffset: 5,
			WatermarkYOffset: 5,
			Format:           FormatPNG,
		}
		watermarkedBytes, _, watermarkErr := engine.Transform(bytes.NewReader(pngBytes), watermarkProcessingOptions)
		require.NoError(t, watermarkErr)
		require.NotEmpty(t, watermarkedBytes)

		// Watermark with invalid image data
		badWatermarkProcessingOptions := ProcessingOptions{
			WatermarkURL: "invalid-image-bytes",
			Format:       FormatPNG,
		}
		fallbackBytes, _, badWatermarkErr := engine.Transform(bytes.NewReader(pngBytes), badWatermarkProcessingOptions)
		require.NoError(t, badWatermarkErr)
		require.NotEmpty(t, fallbackBytes)
	})

	t.Run("color hex and colon parsing", func(t *testing.T) {
		t.Parallel()
		// 3-digit hex
		threeHexColor := parseColorHex("f00", 1.0)
		require.NotNil(t, threeHexColor)

		// Colon format 255:128:0
		colonHexColor := parseColorHex("255:128:0", 0)
		require.NotNil(t, colonHexColor)

		// Empty color
		emptyHexColor := parseColorHex("", 0.5)
		require.Equal(t, color.Transparent, emptyHexColor)

		// Invalid format
		invalidHexColor := parseColorHex("invalid-color-code", 1.0)
		require.Equal(t, color.Transparent, invalidHexColor)
	})

	t.Run("resampling algorithms and anchors", func(t *testing.T) {
		t.Parallel()
		algorithms := []string{"nearest", "linear", "cubic", "lanczos2", "lanczos3", "unknown"}
		for _, algorithm := range algorithms {
			resampleFilter := resolveResampleFilter(algorithm)
			require.NotNil(t, resampleFilter)
		}

		gravityTypes := []string{
			GravityNorth, GravitySouth, GravityEast, GravityWest,
			GravityNorthEast, GravityNorthWest, GravitySouthEast, GravitySouthWest,
			GravityCenter, GravitySmart, GravityFocus, "unknown",
		}
		for _, gravityType := range gravityTypes {
			anchor := resolveImagingAnchor(GravityOption{Type: gravityType})
			require.NotNil(t, anchor)
		}
	})

	t.Run("resize types and enlarge false", func(t *testing.T) {
		t.Parallel()
		resizeTypes := []string{ResizeFit, ResizeFill, ResizeFillDown, ResizeForce, "unknown"}
		for _, resizeType := range resizeTypes {
			processingOptions := ProcessingOptions{
				Width:             300,
				Height:            300,
				ResizeType:        resizeType,
				Enlarge:           false,
				ResizingAlgorithm: "lanczos3",
				Format:            FormatPNG,
			}
			resizedBytes, _, resizeErr := engine.Transform(bytes.NewReader(pngBytes), processingOptions)
			require.NoError(t, resizeErr)
			require.NotEmpty(t, resizedBytes)
		}
	})

	t.Run("resolve mime types", func(t *testing.T) {
		t.Parallel()
		formats := map[string]string{
			FormatWebP: "image/webp",
			FormatAVIF: "image/avif",
			FormatPNG:  "image/png",
			FormatJPEG: "image/jpeg",
			FormatJPG:  "image/jpeg",
			FormatGIF:  "image/gif",
			FormatSVG:  "image/svg+xml",
			FormatICO:  "image/x-icon",
			FormatTIFF: "image/tiff",
			FormatBMP:  "image/bmp",
			"custom":   "application/octet-stream",
		}
		for format, expectedMIME := range formats {
			require.Equal(t, expectedMIME, resolveMIMEType(format))
		}
	})

	t.Run("inspect options and jpeg format", func(t *testing.T) {
		t.Parallel()
		// Inspect JPEG (hasAlpha = false)
		jpegProcessingOptions := ProcessingOptions{
			Format: FormatJPEG,
		}
		jpegBytes, _, _ := engine.Transform(bytes.NewReader(pngBytes), jpegProcessingOptions)
		jpegInfo, inspectErr := engine.Inspect(bytes.NewReader(jpegBytes), NewDefaultInfoOptions())
		require.NoError(t, inspectErr)
		require.NotNil(t, jpegInfo.Alpha)
		require.False(t, *jpegInfo.Alpha)

		// Inspect with false option flags
		sparseInfoOptions := InfoOptions{
			Size:       true,
			Format:     true,
			Dimensions: false,
			Alpha:      false,
			Colorspace: false,
			Bands:      false,
			Pages:      false,
		}
		sparseInfo, sparseInspectErr := engine.Inspect(bytes.NewReader(pngBytes), sparseInfoOptions)
		require.NoError(t, sparseInspectErr)
		require.Nil(t, sparseInfo.Alpha)
		require.Empty(t, sparseInfo.Colorspace)
		require.Zero(t, sparseInfo.Bands)
		require.Zero(t, sparseInfo.Pages)
		require.Zero(t, sparseInfo.Width)
		require.Zero(t, sparseInfo.Height)
	})

	t.Run("error branches in transform and inspect", func(t *testing.T) {
		t.Parallel()
		errorReader := &errorImageTestReader{}

		_, _, transformReadErr := engine.Transform(errorReader, NewDefaultProcessingOptions())
		require.Error(t, transformReadErr)

		_, _, decodeErr := engine.Transform(bytes.NewReader([]byte("not an image")), NewDefaultProcessingOptions())
		require.Error(t, decodeErr)

		_, inspectReadErr := engine.Inspect(errorReader, NewDefaultInfoOptions())
		require.Error(t, inspectReadErr)

		_, inspectCorruptErr := engine.Inspect(bytes.NewReader([]byte("not an image")), NewDefaultInfoOptions())
		require.Error(t, inspectCorruptErr)

		// decodeImage error path
		_, _, decodeImageErr := engine.decodeImage(errorReader)
		require.Error(t, decodeImageErr)
	})

	t.Run("additional format decoding and inspection paths", func(t *testing.T) {
		t.Parallel()

		// 1. WebP format roundtrip & inspection (triggers DecodeConfig fallback)
		webpProcessingOptions := ProcessingOptions{Format: FormatWebP, Quality: 85}
		webpBytes, _, webpTransformErr := engine.Transform(bytes.NewReader(pngBytes), webpProcessingOptions)
		require.NoError(t, webpTransformErr)
		require.NotEmpty(t, webpBytes)

		webpInfo, webpInspectErr := engine.Inspect(bytes.NewReader(webpBytes), NewDefaultInfoOptions())
		require.NoError(t, webpInspectErr)
		require.Equal(t, FormatWebP, webpInfo.Format)

		_, _, webpDecodeErr := engine.Transform(bytes.NewReader(webpBytes), ProcessingOptions{Format: FormatPNG})
		require.NoError(t, webpDecodeErr)

		// 2. BMP format roundtrip
		bmpProcessingOptions := ProcessingOptions{Format: FormatBMP}
		bmpBytes, _, bmpTransformErr := engine.Transform(bytes.NewReader(pngBytes), bmpProcessingOptions)
		require.NoError(t, bmpTransformErr)
		require.NotEmpty(t, bmpBytes)

		_, _, bmpDecodeErr := engine.Transform(bytes.NewReader(bmpBytes), ProcessingOptions{Format: FormatPNG})
		require.NoError(t, bmpDecodeErr)

		// 3. TIFF format roundtrip
		tiffProcessingOptions := ProcessingOptions{Format: FormatTIFF}
		tiffBytes, _, tiffTransformErr := engine.Transform(bytes.NewReader(pngBytes), tiffProcessingOptions)
		require.NoError(t, tiffTransformErr)
		require.NotEmpty(t, tiffBytes)

		_, _, tiffDecodeErr := engine.Transform(bytes.NewReader(tiffBytes), ProcessingOptions{Format: FormatPNG})
		require.NoError(t, tiffDecodeErr)

		// 4. Resize with single dimension (width only or height only)
		widthOnlyProcessingOptions := ProcessingOptions{
			Width:      50,
			Height:     0,
			ResizeType: ResizeFit,
			Format:     FormatPNG,
		}
		widthOnlyBytes, _, widthOnlyErr := engine.Transform(bytes.NewReader(pngBytes), widthOnlyProcessingOptions)
		require.NoError(t, widthOnlyErr)
		require.NotEmpty(t, widthOnlyBytes)

		fillWidthOnlyProcessingOptions := ProcessingOptions{
			Width:      50,
			Height:     0,
			ResizeType: ResizeFill,
			Format:     FormatPNG,
		}
		fillWidthOnlyBytes, _, fillWidthOnlyErr := engine.Transform(bytes.NewReader(pngBytes), fillWidthOnlyProcessingOptions)
		require.NoError(t, fillWidthOnlyErr)
		require.NotEmpty(t, fillWidthOnlyBytes)

		// 5. Watermark opacity <= 0 defaults to 1.0 and scale <= 0
		watermarkBytes := createTestImagePNG(20, 20)
		watermarkDefaultOpacityProcessingOptions := ProcessingOptions{
			WatermarkURL:     string(watermarkBytes),
			WatermarkOpacity: 0,
			WatermarkScale:   0,
			Format:           FormatPNG,
		}
		defaultOpacityBytes, _, defaultOpacityErr := engine.Transform(bytes.NewReader(pngBytes), watermarkDefaultOpacityProcessingOptions)
		require.NoError(t, defaultOpacityErr)
		require.NotEmpty(t, defaultOpacityBytes)

		// 6. Zero quality fallback to default quality
		zeroConfigManager := NewConfigManager(core.NewTestKernel(nil))
		zeroConfigManager.SetMemoryConfig(Config{DefaultQuality: 0})
		zeroQualityEngine := NewEngine(zeroConfigManager)

		zeroQualityProcessingOptions := ProcessingOptions{
			Format:  FormatJPEG,
			Quality: 0,
		}
		zeroQualityBytes, _, zeroQualityErr := zeroQualityEngine.Transform(bytes.NewReader(pngBytes), zeroQualityProcessingOptions)
		require.NoError(t, zeroQualityErr)
		require.NotEmpty(t, zeroQualityBytes)

		// 7. Non-standard format inspection and full decode fallback (WebP, BMP, TIFF)
		rawWebPBytes := createTestImageWebP(100, 50)
		rawWebPInfo, rawWebPErr := engine.Inspect(bytes.NewReader(rawWebPBytes), NewDefaultInfoOptions())
		require.NoError(t, rawWebPErr)
		require.Equal(t, 100, rawWebPInfo.Width)
		require.Equal(t, 50, rawWebPInfo.Height)
		require.Equal(t, FormatWebP, rawWebPInfo.Format)

		rawBMPBytes := createTestImageBMP(80, 40)
		rawBMPInfo, rawBMPErr := engine.Inspect(bytes.NewReader(rawBMPBytes), NewDefaultInfoOptions())
		require.NoError(t, rawBMPErr)
		require.Equal(t, 80, rawBMPInfo.Width)
		require.Equal(t, 40, rawBMPInfo.Height)
		require.Equal(t, FormatBMP, rawBMPInfo.Format)

		rawTIFFBytes := createTestImageTIFF(60, 30)
		rawTIFFInfo, rawTIFFErr := engine.Inspect(bytes.NewReader(rawTIFFBytes), NewDefaultInfoOptions())
		require.NoError(t, rawTIFFErr)
		require.Equal(t, 60, rawTIFFInfo.Width)
		require.Equal(t, 30, rawTIFFInfo.Height)
		require.Equal(t, FormatTIFF, rawTIFFInfo.Format)

		// 8. Transform encoding error on unsupported format
		_, _, transformErr := engine.Transform(bytes.NewReader(pngBytes), ProcessingOptions{
			Format: "unsupported-format",
		})
		require.Error(t, transformErr)
		require.Contains(t, transformErr.Error(), "failed to encode output image")
	})
}

type errorImageTestReader struct{}

func (errorImageTestReader) Read(destinationSlice []byte) (int, error) {
	return 0, errors.New("simulated read error")
}
