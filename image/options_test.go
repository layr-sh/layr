package image

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/require"
	"layr.sh/core"
)

func TestImageOptionsUnit(t *testing.T) {
	t.Parallel()

	t.Run("parse url path plain format", func(t *testing.T) {
		t.Parallel()
		rawPath := "/rs:fill:300:200/g:sm/plain/local/bucket/avatar.jpg@webp"
		optionsString, sourceURL, extension, parseErr := ParseURLPath(rawPath)
		require.NoError(t, parseErr)
		require.Equal(t, "rs:fill:300:200/g:sm", optionsString)
		require.Equal(t, "local/bucket/avatar.jpg", sourceURL)
		require.Equal(t, "webp", extension)
	})

	t.Run("parse url path base64 encoded format", func(t *testing.T) {
		t.Parallel()
		originalSourceURL := "https://example.com/images/hero.png"
		encodedSourceURL := base64.RawURLEncoding.EncodeToString([]byte(originalSourceURL))

		rawPath := "/rs:fit:800:600/q:85/" + encodedSourceURL + ".webp"
		optionsString, sourceURL, extension, parseErr := ParseURLPath(rawPath)
		require.NoError(t, parseErr)
		require.Equal(t, "rs:fit:800:600/q:85", optionsString)
		require.Equal(t, originalSourceURL, sourceURL)
		require.Equal(t, "webp", extension)
	})

	t.Run("parse url path malformed errors", func(t *testing.T) {
		t.Parallel()
		_, _, _, emptyErr := ParseURLPath("")
		require.Error(t, emptyErr)

		_, _, _, rootErr := ParseURLPath("/")
		require.Error(t, rootErr)

		_, _, _, missingSourceErr := ParseURLPath("/plain/")
		require.Error(t, missingSourceErr)
	})

	t.Run("parse all processing options canonical and abbreviated", func(t *testing.T) {
		t.Parallel()
		encodedWatermarkURL := base64.RawURLEncoding.EncodeToString([]byte("local/bucket/mark.png"))
		optionsString := "rs:fill:400:300:1:1/algo:lanczos3/dpr:2/g:fp:0.25:0.75/c:200:150:noea/t:10:ffffff:1:0/pd:10:20:10:20/ar:1/rot:90/bg:ff0000/bga:0.5/bl:2.5/sh:1.2/pix:8/wm:0.8:soea:5:5:0.2:" + encodedWatermarkURL + "/f:webp/q:90/sm:1/kcr:1/scp:1/z:1.5:1.5/cb:v2/raw:1"

		kernel := core.NewTestKernel(nil)
		presetManager := NewPresetManager(kernel)

		processingOptions, parseErr := ParseProcessingOptions(optionsString, presetManager.ResolveOptions)
		require.NoError(t, parseErr)

		require.Equal(t, "fill", processingOptions.ResizeType)
		require.Equal(t, 400, processingOptions.Width)
		require.Equal(t, 300, processingOptions.Height)
		require.True(t, processingOptions.Enlarge)
		require.True(t, processingOptions.Extend)
		require.Equal(t, "lanczos3", processingOptions.ResizingAlgorithm)
		require.Equal(t, 2.0, processingOptions.DPR)

		require.Equal(t, "fp", processingOptions.Gravity.Type)
		require.Equal(t, 0.25, processingOptions.Gravity.XOffset)
		require.Equal(t, 0.75, processingOptions.Gravity.YOffset)

		require.Equal(t, 200.0, processingOptions.CropWidth)
		require.Equal(t, 150.0, processingOptions.CropHeight)
		require.Equal(t, "noea", processingOptions.CropGravity.Type)

		require.Equal(t, 10.0, processingOptions.TrimThreshold)
		require.Equal(t, "ffffff", processingOptions.TrimColor)
		require.True(t, processingOptions.TrimEqualHor)
		require.False(t, processingOptions.TrimEqualVer)

		require.Equal(t, 10, processingOptions.PaddingTop)
		require.Equal(t, 20, processingOptions.PaddingRight)
		require.Equal(t, 10, processingOptions.PaddingBottom)
		require.Equal(t, 20, processingOptions.PaddingLeft)

		require.True(t, processingOptions.AutoRotate)
		require.Equal(t, 90, processingOptions.Rotate)

		require.Equal(t, "ff0000", processingOptions.Background)
		require.Equal(t, 0.5, processingOptions.BackgroundAlpha)

		require.Equal(t, 2.5, processingOptions.Blur)
		require.Equal(t, 1.2, processingOptions.Sharpen)
		require.Equal(t, 8, processingOptions.Pixelate)

		require.Equal(t, 0.8, processingOptions.WatermarkOpacity)
		require.Equal(t, "soea", processingOptions.WatermarkGravity.Type)
		require.Equal(t, 5, processingOptions.WatermarkXOffset)
		require.Equal(t, 5, processingOptions.WatermarkYOffset)
		require.Equal(t, 0.2, processingOptions.WatermarkScale)
		require.Equal(t, "local/bucket/mark.png", processingOptions.WatermarkURL)

		require.Equal(t, FormatWebP, processingOptions.Format)
		require.Equal(t, 90, processingOptions.Quality)
		require.True(t, processingOptions.StripMetadata)
		require.True(t, processingOptions.KeepCopyright)
		require.True(t, processingOptions.StripColorProfile)
		require.Equal(t, 1.5, processingOptions.ZoomX)
		require.Equal(t, 1.5, processingOptions.ZoomY)
		require.Equal(t, "v2", processingOptions.Cachebuster)
		require.True(t, processingOptions.RawMode)
	})

	t.Run("parse size and extend options", func(t *testing.T) {
		t.Parallel()
		optionsString := "s:250:180:1:0/ex:1:soea/min-width:100/min-height:50"
		processingOptions, parseErr := ParseProcessingOptions(optionsString, nil)
		require.NoError(t, parseErr)
		require.Equal(t, 250, processingOptions.Width)
		require.Equal(t, 180, processingOptions.Height)
		require.True(t, processingOptions.Enlarge)
		require.True(t, processingOptions.Extend)
		require.Equal(t, "soea", processingOptions.ExtendGravity.Type)
		require.Equal(t, 100, processingOptions.MinWidth)
		require.Equal(t, 50, processingOptions.MinHeight)
	})

	t.Run("parse unsharpen and format quality options", func(t *testing.T) {
		t.Parallel()
		optionsString := "ush:0.5:1.0:2.0:0.1/fq:webp:75/aq:dssim:0.01:60:85/wmu:aHR0cHM6Ly9leGFtcGxlLmNvbS9sb2dvLnBuZw"
		processingOptions, parseErr := ParseProcessingOptions(optionsString, nil)
		require.NoError(t, parseErr)
		require.Equal(t, 0.5, processingOptions.UnsharpenMode)
		require.Equal(t, 1.0, processingOptions.UnsharpenWeight)
		require.Equal(t, 2.0, processingOptions.UnsharpenDividor)
		require.Equal(t, 0.1, processingOptions.UnsharpenThreshold)
		require.Equal(t, 75, processingOptions.FormatQuality[FormatWebP])
		require.Equal(t, "dssim", processingOptions.AutoQualityMethod)
		require.Equal(t, 0.01, processingOptions.AutoQualityTarget)
		require.Equal(t, 60, processingOptions.AutoQualityMin)
		require.Equal(t, 85, processingOptions.AutoQualityMax)
		require.Equal(t, "https://example.com/logo.png", processingOptions.WatermarkURL)
	})

	t.Run("expand presets in options", func(t *testing.T) {
		t.Parallel()
		presetResolver := func(name string) (string, error) {
			if name == "thumb" {
				return "rs:fill:120:120/q:80", nil
			}
			return "", ErrPresetNotFound
		}

		processingOptions, parseErr := ParseProcessingOptions("pr:thumb/f:webp", presetResolver)
		require.NoError(t, parseErr)
		require.Equal(t, 120, processingOptions.Width)
		require.Equal(t, 120, processingOptions.Height)
		require.Equal(t, "fill", processingOptions.ResizeType)
		require.Equal(t, 80, processingOptions.Quality)
		require.Equal(t, FormatWebP, processingOptions.Format)
	})

	t.Run("parse info options", func(t *testing.T) {
		t.Parallel()
		optionsString := "size:1/format:1/dimensions:1/exif:0/colorspace:1"
		infoOptions := ParseInfoOptions(optionsString)

		require.True(t, infoOptions.Size)
		require.True(t, infoOptions.Format)
		require.True(t, infoOptions.Dimensions)
		require.False(t, infoOptions.EXIF)
		require.True(t, infoOptions.Colorspace)
	})

	t.Run("parse remaining options and short abbreviations", func(t *testing.T) {
		t.Parallel()
		fallbackBase64 := base64.RawURLEncoding.EncodeToString([]byte("https://fallback.com/404.png"))
		optionsString := "rt:fill/ra:lanczos2/w:100/h:200/mw:50/mh:50/dpr:1.5/el:1/vts:10/cb:key123/kcr:1/scp:1/raw:1/skp:svg/fiu:" + fallbackBase64
		processingOptions, parseErr := ParseProcessingOptions(optionsString, nil)
		require.NoError(t, parseErr)
		require.Equal(t, "fill", processingOptions.ResizeType)
		require.Equal(t, "lanczos2", processingOptions.ResizingAlgorithm)
		require.Equal(t, 100, processingOptions.Width)
		require.Equal(t, 200, processingOptions.Height)
		require.Equal(t, 50, processingOptions.MinWidth)
		require.Equal(t, 50, processingOptions.MinHeight)
		require.Equal(t, 1.5, processingOptions.DPR)
		require.True(t, processingOptions.Enlarge)
		require.True(t, processingOptions.KeepCopyright)
		require.True(t, processingOptions.StripColorProfile)
		require.True(t, processingOptions.RawMode)
		require.Equal(t, 10.0, processingOptions.VideoThumbnailSecond)
		require.Equal(t, []string{"svg"}, processingOptions.SkipProcessing)
		require.Equal(t, "https://fallback.com/404.png", processingOptions.FallbackImageURL)

		// Test unencoded fallback URL
		unencodedProcessingOptions, unencodedErr := ParseProcessingOptions("fiu:local-asset.png", nil)
		require.NoError(t, unencodedErr)
		require.Equal(t, "local-asset.png", unencodedProcessingOptions.FallbackImageURL)

		// Expired option
		_, expiredErr := ParseProcessingOptions("exp:100", nil)
		require.Error(t, expiredErr)
		require.Contains(t, expiredErr.Error(), "image request expired")

		// Info options with extra flags
		extraInfoOptions := ParseInfoOptions("xmp:1/video_meta:1/exif:1:1/alpha:1/pages:1/bands:1")
		require.True(t, extraInfoOptions.XMP)
		require.True(t, extraInfoOptions.VideoMeta)
		require.True(t, extraInfoOptions.EXIF)
		require.True(t, extraInfoOptions.EXIFCanonicalNames)
		require.True(t, extraInfoOptions.Alpha)
		require.True(t, extraInfoOptions.Pages)
		require.True(t, extraInfoOptions.Bands)

		// Empty options and underscore
		emptyProcessingOptions, emptyDefaultErr := ParseProcessingOptions("", nil)
		require.NoError(t, emptyDefaultErr)
		require.Equal(t, 0, emptyProcessingOptions.Quality)

		underscoreProcessingOptions, underscoreDefaultErr := ParseProcessingOptions("_", nil)
		require.NoError(t, underscoreDefaultErr)
		require.Equal(t, 0, underscoreProcessingOptions.Quality)
	})

	t.Run("parse base64 variants and errors", func(t *testing.T) {
		t.Parallel()
		source := "https://example.com/pic.jpg"

		// Standard base64 with padding
		paddedBase64 := base64.URLEncoding.EncodeToString([]byte(source))
		_, parsedSource1, _, parsePaddedErr := ParseURLPath("/rs:fill:100:100/" + paddedBase64)
		require.NoError(t, parsePaddedErr)
		require.Equal(t, source, parsedSource1)

		// Standard StdEncoding base64 with + and /
		standardBase64 := base64.StdEncoding.EncodeToString([]byte(source))
		_, parsedSource2, _, parseStdErr := ParseURLPath("/rs:fill:100:100/" + standardBase64)
		require.NoError(t, parseStdErr)
		require.Equal(t, source, parsedSource2)

		// With @ext
		_, parsedSource3, extension, parseExtErr := ParseURLPath("/rs:fill:100:100/" + paddedBase64 + "@webp")
		require.NoError(t, parseExtErr)
		require.Equal(t, source, parsedSource3)
		require.Equal(t, "webp", extension)

		// Corrupt base64
		_, _, _, corruptErr := ParseURLPath("/rs:fill:100:100/!@#$%^&*()")
		require.Error(t, corruptErr)
	})

	t.Run("expand presets error paths", func(t *testing.T) {
		t.Parallel()
		// Unknown preset
		_, err := ParseProcessingOptions("pr:missing", func(name string) (string, error) {
			return "", ErrPresetNotFound
		})
		require.Error(t, err)

		// Recursion depth limit
		depthResolver := func(name string) (string, error) {
			return "pr:recurse", nil
		}
		_, recurseErr := ParseProcessingOptions("pr:recurse", depthResolver)
		require.Error(t, recurseErr)
		require.Contains(t, recurseErr.Error(), "maximum preset recursion depth exceeded")

		// Nil preset resolver
		_, nilResolverErr := ParseProcessingOptions("pr:thumb", nil)
		require.Error(t, nilResolverErr)
		require.Contains(t, nilResolverErr.Error(), "preset resolver unavailable")
	})

	t.Run("padding and gravity edge cases", func(t *testing.T) {
		t.Parallel()

		// Single padding
		singlePadProcessingOptions, singlePadErr := ParseProcessingOptions("pd:15", nil)
		require.NoError(t, singlePadErr)
		require.Equal(t, 15, singlePadProcessingOptions.PaddingTop)
		require.Equal(t, 15, singlePadProcessingOptions.PaddingRight)
		require.Equal(t, 15, singlePadProcessingOptions.PaddingBottom)
		require.Equal(t, 15, singlePadProcessingOptions.PaddingLeft)

		// Vertical and horizontal padding
		doublePadProcessingOptions, doublePadErr := ParseProcessingOptions("pd:20:40", nil)
		require.NoError(t, doublePadErr)
		require.Equal(t, 20, doublePadProcessingOptions.PaddingTop)
		require.Equal(t, 20, doublePadProcessingOptions.PaddingBottom)
		require.Equal(t, 40, doublePadProcessingOptions.PaddingRight)
		require.Equal(t, 40, doublePadProcessingOptions.PaddingLeft)

		// Non-focus gravity with offsets
		gravityProcessingOptions, gravityErr := ParseProcessingOptions("g:no:12.5:34.5", nil)
		require.NoError(t, gravityErr)
		require.Equal(t, "no", gravityProcessingOptions.Gravity.Type)
		require.Equal(t, 12.5, gravityProcessingOptions.Gravity.XOffset)
		require.Equal(t, 34.5, gravityProcessingOptions.Gravity.YOffset)

		// Empty gravity
		var emptyGravityOption GravityOption
		parseGravityOption(&emptyGravityOption, nil)
		require.Empty(t, emptyGravityOption.Type)

		// Unencoded watermark url fallback
		watermarkURLProcessingOptions, watermarkURLErr := ParseProcessingOptions("wmu:custom@watermark", nil)
		require.NoError(t, watermarkURLErr)
		require.Equal(t, "custom@watermark", watermarkURLProcessingOptions.WatermarkURL)

		// Watermark option with unencoded URL
		watermarkProcessingOptions, watermarkErr := ParseProcessingOptions("wm:0.75:so:10:20:1.5:raw_watermark_url", nil)
		require.NoError(t, watermarkErr)
		require.Equal(t, "raw_watermark_url", watermarkProcessingOptions.WatermarkURL)
	})

	t.Run("path parsing edge cases", func(t *testing.T) {
		t.Parallel()

		// Plain path with empty source
		_, _, _, missingPlainSourceErr := ParseURLPath("/rs:fill:100:100/plain/")
		require.Error(t, missingPlainSourceErr)
		require.Contains(t, missingPlainSourceErr.Error(), "missing source url in plain path")

		// Plain prefix with empty source
		_, _, _, missingPrefixSourceErr := ParseURLPath("plain/   ")
		require.Error(t, missingPrefixSourceErr)
		require.Contains(t, missingPrefixSourceErr.Error(), "missing source url in plain path")

		// Plain prefix with @ext and escaped URL
		escapedPlainURL := "plain/http%3A%2F%2Fexample.com%2Fimage.png@webp"
		_, plainSourceURL, plainExt, plainParseErr := ParseURLPath(escapedPlainURL)
		require.NoError(t, plainParseErr)
		require.Equal(t, "http://example.com/image.png", plainSourceURL)
		require.Equal(t, "webp", plainExt)

		// Empty path
		_, _, _, emptyPathErr := ParseURLPath("/")
		require.Error(t, emptyPathErr)

		// Encoded path with empty source
		_, _, _, emptyEncodedSourceErr := ParseURLPath("/rs:fill:100:100/")
		require.Error(t, emptyEncodedSourceErr)

		// Encoded path with standard base64 decoding
		standardBase64Payload := base64.StdEncoding.EncodeToString([]byte("local/bucket/photo@jpg"))
		_, standardParsedSource, standardExt, standardErr := ParseURLPath("/rs:fill:50:50/" + standardBase64Payload)
		require.NoError(t, standardErr)
		require.Equal(t, "local/bucket/photo", standardParsedSource)
		require.Equal(t, "jpg", standardExt)

		// Standard base64 containing '+' which fails RawURL and URLEncoding
		plusBytes := []byte{250, 0, 0}
		plusBase64Payload := base64.StdEncoding.EncodeToString(plusBytes)
		_, plusParsedSource, _, plusErr := ParseURLPath("/rs:fill:50:50/" + plusBase64Payload)
		require.NoError(t, plusErr)
		require.Equal(t, string(plusBytes), plusParsedSource)

		// Empty segments in options string
		emptySegmentsProcessingOptions, emptySegmentsErr := ParseProcessingOptions("rs:fill:100:100//q:80/_", nil)
		require.NoError(t, emptySegmentsErr)
		require.Equal(t, 80, emptySegmentsProcessingOptions.Quality)

		// Empty segment in ParseInfoOptions
		emptyInfoOptions := ParseInfoOptions("size/s//_")
		require.True(t, emptyInfoOptions.Size)
	})
}
