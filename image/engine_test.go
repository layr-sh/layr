package image

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"

	"image"
	"image/color"
	"image/gif"
	"image/png"
	"io"
	"testing"

	"github.com/KarpelesLab/gowebp"
	"github.com/gen2brain/jpegn"
	"github.com/gen2brain/pngn"
	"github.com/stretchr/testify/require"
	"golang.org/x/image/bmp"
	"golang.org/x/image/tiff"
	"layr.sh/core"
)

func TestImageMetadataValidationUnit(t *testing.T) {
	t.Parallel()
	engine := NewEngine(NewConfigManager(core.NewTestKernel(nil)))

	t.Run("PNG metadata reader rejects decoded source", func(t *testing.T) {
		t.Parallel()
		// The built-in PNG decoder validates the same chunks as the metadata
		// reader. A uniquely matched codec exercises metadata-reader failure
		// independently without replacing the production decoder or its API.
		const codecMagic = "LYR\x00PNG\x00"
		image.RegisterFormat(FormatPNG, codecMagic, func(io.Reader) (image.Image, error) {
			return image.NewNRGBA(image.Rect(0, 0, 1, 1)), nil
		}, func(io.Reader) (image.Config, error) {
			return image.Config{ColorModel: color.NRGBAModel, Width: 1, Height: 1}, nil
		})
		inputBytes := []byte(codecMagic)
		_, _, decodeErr := image.Decode(bytes.NewReader(inputBytes))
		require.NoError(t, decodeErr)

		outputBytes, contentType, transformErr := engine.Transform(bytes.NewReader(inputBytes), NewDefaultProcessingOptions())
		require.ErrorContains(t, transformErr, "failed to read source metadata")
		require.Empty(t, outputBytes)
		require.Empty(t, contentType)
	})

	t.Run("JPEG invalid metadata segment length", func(t *testing.T) {
		t.Parallel()
		var inputBuffer bytes.Buffer
		require.NoError(t, jpegn.Encode(&inputBuffer, image.NewNRGBA(image.Rect(0, 0, 1, 1)), nil))
		inputBytes := inputBuffer.Bytes()
		inputBytes = append(append(append([]byte{}, inputBytes[:2]...), 0, 0, 0, 1), inputBytes[2:]...)
		_, _, decodeErr := image.Decode(bytes.NewReader(inputBytes))
		require.NoError(t, decodeErr)

		outputBytes, contentType, transformErr := engine.Transform(bytes.NewReader(inputBytes), NewDefaultProcessingOptions())
		require.ErrorContains(t, transformErr, "invalid JPEG metadata segment length")
		require.Empty(t, outputBytes)
		require.Empty(t, contentType)
	})
}

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

func TestImageRuntimeEncodingUnit(t *testing.T) {
	t.Parallel()
	for _, enabled := range []bool{false, true} {
		t.Run(fmt.Sprint(enabled), func(t *testing.T) {
			t.Parallel()
			configManager := NewConfigManager(core.NewTestKernel(nil))
			config := DefaultConfig()
			config.PNGInterlaced = enabled
			config.JPEGProgressive = enabled
			configManager.SetMemoryConfig(config)
			engine := NewEngine(configManager)
			processingOptions := NewDefaultProcessingOptions()
			processingOptions.Format = FormatPNG
			output, _, err := engine.Transform(bytes.NewReader(createTestImagePNG(8, 8)), processingOptions)
			require.NoError(t, err)
			require.Equal(t, enabled, output[28] == 1)
			_, err = png.Decode(bytes.NewReader(output))
			require.NoError(t, err)
			processingOptions.Format = FormatJPEG
			output, _, err = engine.Transform(bytes.NewReader(createTestImagePNG(8, 8)), processingOptions)
			require.NoError(t, err)
			require.Equal(t, enabled, bytes.Contains(output, []byte{0xff, 0xc2}))
			require.Equal(t, !enabled, bytes.Contains(output, []byte{0xff, 0xc0}))
		})
	}
}

func TestImageRuntimeMetadataUnit(t *testing.T) {
	t.Parallel()
	for _, strip := range []bool{false, true} {
		t.Run(fmt.Sprint(strip), func(t *testing.T) {
			t.Parallel()
			configManager := NewConfigManager(core.NewTestKernel(nil))
			config := DefaultConfig()
			config.StripMetadata = strip
			configManager.SetMemoryConfig(config)
			engine := NewEngine(configManager)
			var inputBuffer bytes.Buffer
			err := pngn.Encode(&inputBuffer, image.NewNRGBA(image.Rect(0, 0, 8, 8)), &pngn.EncodeOptions{Metadata: &pngn.Metadata{Text: []pngn.Text{{Keyword: "Copyright", Value: "owner"}, {Keyword: "Description", Value: "private"}}}})
			require.NoError(t, err)
			output, _, err := engine.Transform(bytes.NewReader(inputBuffer.Bytes()), NewDefaultProcessingOptions())
			require.NoError(t, err)
			metadata, err := pngn.DecodeMetadata(bytes.NewReader(output))
			require.NoError(t, err)
			require.Contains(t, metadata.Text, pngn.Text{Keyword: "Copyright", Value: "owner"})
			require.Equal(t, !strip, bytes.Contains(output, []byte("private")))
		})
	}
}

func TestImageJPEGMetadataUnit(t *testing.T) {
	t.Parallel()
	for _, copyright := range []string{"ab", "copyright owner"} {
		t.Run(copyright, func(t *testing.T) {
			t.Parallel()
			for _, strip := range []bool{false, true} {
				t.Run(fmt.Sprint(strip), func(t *testing.T) {
					t.Parallel()
					configManager := NewConfigManager(core.NewTestKernel(nil))
					config := DefaultConfig()
					config.StripMetadata = strip
					config.StripColorProfile = strip
					configManager.SetMemoryConfig(config)
					engine := NewEngine(configManager)
					payload := append([]byte("Exif\x00\x00II\x2a\x00\x08\x00\x00\x00\x01\x00\x98\x82\x02\x00"), make([]byte, 12)...)
					binary.LittleEndian.PutUint32(payload[20:], uint32(len(copyright)+1))
					if len(copyright)+1 <= 4 {
						copy(payload[24:28], copyright)
					} else {
						binary.LittleEndian.PutUint32(payload[24:], 26)
						payload = append(append(payload, []byte(copyright)...), 0)
					}
					var inputBuffer bytes.Buffer
					require.NoError(t, jpegn.Encode(&inputBuffer, image.NewNRGBA(image.Rect(0, 0, 8, 8)), &jpegn.EncodeOptions{Exif: payload, Segments: []jpegn.Segment{{Marker: 0xfe, Data: []byte("private metadata")}, {Marker: 0xe2, Data: []byte("ICC_PROFILE\x00profile")}}}))
					processingOptions := NewDefaultProcessingOptions()
					processingOptions.Format = FormatJPEG
					output, _, err := engine.Transform(bytes.NewReader(inputBuffer.Bytes()), processingOptions)
					require.NoError(t, err)
					exif, err := jpegn.DecodeExif(bytes.NewReader(output))
					require.NoError(t, err)
					require.Equal(t, copyright, exif.Copyright)
					require.Equal(t, !strip, bytes.Contains(output, []byte("private metadata")))
					require.Equal(t, !strip, bytes.Contains(output, []byte("ICC_PROFILE")))
				})
			}
		})
	}
}

func TestImageWatermarkTransparencyUnit(t *testing.T) {
	t.Parallel()
	engine := NewEngine(NewConfigManager(core.NewTestKernel(nil)))
	sourceNRGBA := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	watermarkNRGBA := image.NewNRGBA(sourceNRGBA.Bounds())
	for y := range 4 {
		for x := range 4 {
			sourceNRGBA.SetNRGBA(x, y, color.NRGBA{R: 255, A: 255})
			watermarkNRGBA.SetNRGBA(x, y, color.NRGBA{B: 255, A: 255})
		}
	}
	var watermarkBuffer bytes.Buffer
	require.NoError(t, png.Encode(&watermarkBuffer, watermarkNRGBA))
	processingOptions := NewDefaultProcessingOptions()
	processingOptions.WatermarkURL = "https://example.com/watermark.png"
	processingOptions.watermarkData = watermarkBuffer.Bytes()
	processingOptions.WatermarkOpacity = 0
	outputNRGBA, err := engine.applyWatermark(sourceNRGBA, processingOptions)
	require.NoError(t, err)
	require.Equal(t, sourceNRGBA.Pix, outputNRGBA.Pix)
	processingOptions.WatermarkOpacity = 1
	outputNRGBA, err = engine.applyWatermark(sourceNRGBA, processingOptions)
	require.NoError(t, err)
	require.Equal(t, watermarkNRGBA.Pix, outputNRGBA.Pix)
}

func TestImageSourceLimitsUnit(t *testing.T) {
	t.Parallel()
	configManager := NewConfigManager(core.NewTestKernel(nil))
	config := DefaultConfig()
	config.MaxSrcResolution = 1
	config.MaxAnimationFrames = 1
	configManager.SetMemoryConfig(config)
	engine := NewEngine(configManager)
	t.Run("resolution", func(t *testing.T) {
		_, _, err := engine.decodeImage(bytes.NewReader(createTestImagePNG(1001, 1000)))
		require.ErrorContains(t, err, "maximum resolution")
	})
	t.Run("animated GIF", func(t *testing.T) {
		framePaletted := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Black, color.White})
		var inputBuffer bytes.Buffer
		require.NoError(t, gif.EncodeAll(&inputBuffer, &gif.GIF{Image: []*image.Paletted{framePaletted, framePaletted}, Delay: []int{0, 0}}))
		_, _, err := engine.decodeImage(bytes.NewReader(inputBuffer.Bytes()))
		require.ErrorContains(t, err, "maximum animation frames")
	})
	t.Run("animated PNG", func(t *testing.T) {
		inputBytes := createTestImagePNG(1, 1)
		chunk := make([]byte, 20)
		binary.BigEndian.PutUint32(chunk, 8)
		copy(chunk[4:], "acTL")
		binary.BigEndian.PutUint32(chunk[8:], 2)
		inputBytes = append(append(append([]byte{}, inputBytes[:33]...), chunk...), inputBytes[33:]...)
		_, _, err := engine.decodeImage(bytes.NewReader(inputBytes))
		require.ErrorContains(t, err, "maximum animation frames")
	})
}

func TestImageMalformedSourcesUnit(t *testing.T) {
	t.Parallel()
	engine := NewEngine(NewConfigManager(core.NewTestKernel(nil)))
	gifHeader := []byte{'G', 'I', 'F', '8', '9', 'a', 1, 0, 1, 0, 0, 0, 0}
	for _, testCase := range []struct {
		name    string
		data    []byte
		message string
	}{
		{name: "descriptor", data: append(bytes.Clone(gifHeader), 0x2c), message: "truncated GIF frame descriptor"},
		{name: "block type", data: append(bytes.Clone(gifHeader), 0x7f), message: "invalid GIF block type"},
		{name: "data block", data: append(bytes.Clone(gifHeader), 0x21, 0xfe), message: "truncated GIF data block"},
		{name: "sub-block", data: append(bytes.Clone(gifHeader), 0x21, 0xfe, 4, 1), message: "truncated GIF sub-block"},
		{name: "missing pixels", data: append(bytes.Clone(gifHeader), 0x3b), message: "unsupported or corrupted image format"},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			_, _, err := engine.decodeImage(bytes.NewReader(testCase.data))
			require.ErrorContains(t, err, testCase.message)
		})
	}
	t.Run("global palette", func(t *testing.T) {
		framePaletted := image.NewPaletted(image.Rect(0, 0, 1, 1), color.Palette{color.Black, color.White})
		var inputBuffer bytes.Buffer
		require.NoError(t, gif.Encode(&inputBuffer, framePaletted, nil))
		_, _, err := engine.decodeImage(bytes.NewReader(inputBuffer.Bytes()))
		require.NoError(t, err)
	})
	t.Run("PNG chunk length", func(t *testing.T) {
		inputBytes := createTestImagePNG(1, 1)
		binary.BigEndian.PutUint32(inputBytes[33:], 0xffffffff)
		_, _, err := engine.decodeImage(bytes.NewReader(inputBytes))
		require.ErrorContains(t, err, "PNG chunk length")
	})
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

		getInfoResponse, inspectErr := engine.Inspect(reader, infoOptions)
		require.NoError(t, inspectErr)
		require.NotNil(t, getInfoResponse)
		require.Equal(t, 200, getInfoResponse.Width)
		require.Equal(t, 100, getInfoResponse.Height)
		require.Equal(t, FormatPNG, getInfoResponse.Format)
		require.Equal(t, "image/png", getInfoResponse.MIMEType)
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
		transformedGetInfoResponse, inspectErr := engine.Inspect(transformedReader, NewDefaultInfoOptions())
		require.NoError(t, inspectErr)
		require.Equal(t, 50, transformedGetInfoResponse.Width)
		require.Equal(t, 50, transformedGetInfoResponse.Height)
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
		transformedGetInfoResponse, inspectErr := engine.Inspect(transformedReader, NewDefaultInfoOptions())
		require.NoError(t, inspectErr)
		require.Equal(t, 100, transformedGetInfoResponse.Width)
		require.Equal(t, 200, transformedGetInfoResponse.Height)
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
			getInfoResponse, inspectErr := engine.Inspect(inspectReader, NewDefaultInfoOptions())
			require.NoError(t, inspectErr)
			require.NotNil(t, getInfoResponse)
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

	t.Run("watermark application and invalid data", func(t *testing.T) {
		t.Parallel()
		watermarkPNG := createTestImagePNG(20, 20)
		watermarkProcessingOptions := ProcessingOptions{
			WatermarkURL:     "https://example.com/watermark.png",
			watermarkData:    watermarkPNG,
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
		require.ErrorContains(t, badWatermarkErr, "failed to apply watermark")
		require.Empty(t, fallbackBytes)
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
		jpegGetInfoResponse, inspectErr := engine.Inspect(bytes.NewReader(jpegBytes), NewDefaultInfoOptions())
		require.NoError(t, inspectErr)
		require.NotNil(t, jpegGetInfoResponse.Alpha)
		require.False(t, *jpegGetInfoResponse.Alpha)

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
		sparseGetInfoResponse, sparseInspectErr := engine.Inspect(bytes.NewReader(pngBytes), sparseInfoOptions)
		require.NoError(t, sparseInspectErr)
		require.Nil(t, sparseGetInfoResponse.Alpha)
		require.Empty(t, sparseGetInfoResponse.Colorspace)
		require.Zero(t, sparseGetInfoResponse.Bands)
		require.Zero(t, sparseGetInfoResponse.Pages)
		require.Zero(t, sparseGetInfoResponse.Width)
		require.Zero(t, sparseGetInfoResponse.Height)
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

		webpGetInfoResponse, webpInspectErr := engine.Inspect(bytes.NewReader(webpBytes), NewDefaultInfoOptions())
		require.NoError(t, webpInspectErr)
		require.Equal(t, FormatWebP, webpGetInfoResponse.Format)

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

		// 5. Zero watermark opacity remains transparent.
		watermarkBytes := createTestImagePNG(20, 20)
		watermarkDefaultOpacityProcessingOptions := ProcessingOptions{
			WatermarkURL:     "https://example.com/watermark.png",
			watermarkData:    watermarkBytes,
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
		rawWebPGetInfoResponse, rawWebPErr := engine.Inspect(bytes.NewReader(rawWebPBytes), NewDefaultInfoOptions())
		require.NoError(t, rawWebPErr)
		require.Equal(t, 100, rawWebPGetInfoResponse.Width)
		require.Equal(t, 50, rawWebPGetInfoResponse.Height)
		require.Equal(t, FormatWebP, rawWebPGetInfoResponse.Format)

		rawBMPBytes := createTestImageBMP(80, 40)
		rawBMPGetInfoResponse, rawBMPErr := engine.Inspect(bytes.NewReader(rawBMPBytes), NewDefaultInfoOptions())
		require.NoError(t, rawBMPErr)
		require.Equal(t, 80, rawBMPGetInfoResponse.Width)
		require.Equal(t, 40, rawBMPGetInfoResponse.Height)
		require.Equal(t, FormatBMP, rawBMPGetInfoResponse.Format)

		rawTIFFBytes := createTestImageTIFF(60, 30)
		rawTIFFGetInfoResponse, rawTIFFErr := engine.Inspect(bytes.NewReader(rawTIFFBytes), NewDefaultInfoOptions())
		require.NoError(t, rawTIFFErr)
		require.Equal(t, 60, rawTIFFGetInfoResponse.Width)
		require.Equal(t, 30, rawTIFFGetInfoResponse.Height)
		require.Equal(t, FormatTIFF, rawTIFFGetInfoResponse.Format)

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
