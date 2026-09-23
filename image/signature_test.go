package image

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestImageSignatureUnit(t *testing.T) {
	t.Parallel()

	signingKey := []byte("01234567890123456789012345678901")
	signingSalt := []byte("salt0123456789012345678901234567")
	samplePath := "/rs:fill:300:200/plain/local/bucket/image.jpg@webp"

	t.Run("valid signature generation and verification", func(t *testing.T) {
		t.Parallel()
		signature := SignPath(signingKey, signingSalt, samplePath)
		require.NotEmpty(t, signature)

		isValid := VerifySignature(signingKey, signingSalt, signature, samplePath, false)
		require.True(t, isValid)
	})

	t.Run("truncated signature verification", func(t *testing.T) {
		t.Parallel()
		hmacHash := hmac.New(sha256.New, signingKey)
		hmacHash.Write(signingSalt)
		hmacHash.Write([]byte(samplePath))
		fullDigest := hmacHash.Sum(nil)

		truncatedDigest := fullDigest[:16]
		truncatedSignature := base64.RawURLEncoding.EncodeToString(truncatedDigest)

		isValid := VerifySignature(signingKey, signingSalt, truncatedSignature, samplePath, false)
		require.True(t, isValid)
	})

	t.Run("hex encoded signature verification", func(t *testing.T) {
		t.Parallel()
		hmacHash := hmac.New(sha256.New, signingKey)
		hmacHash.Write(signingSalt)
		hmacHash.Write([]byte(samplePath))
		fullDigest := hmacHash.Sum(nil)

		hexSignature := hex.EncodeToString(fullDigest)

		isValid := VerifySignature(signingKey, signingSalt, hexSignature, samplePath, false)
		require.True(t, isValid)
	})

	t.Run("insecure modes with allow insecure true and false", func(t *testing.T) {
		t.Parallel()
		// When allowInsecure is true:
		require.True(t, VerifySignature(signingKey, signingSalt, "_", samplePath, true))
		require.True(t, VerifySignature(signingKey, signingSalt, "unsafe", samplePath, true))

		// When allowInsecure is false:
		require.False(t, VerifySignature(signingKey, signingSalt, "_", samplePath, false))
		require.False(t, VerifySignature(signingKey, signingSalt, "unsafe", samplePath, false))
	})

	t.Run("invalid signatures rejected", func(t *testing.T) {
		t.Parallel()
		require.False(t, VerifySignature(signingKey, signingSalt, "", samplePath, false))
		require.False(t, VerifySignature(signingKey, signingSalt, "   ", samplePath, false))
		require.False(t, VerifySignature(signingKey, signingSalt, "invalid-signature", samplePath, false))
		require.False(t, VerifySignature(nil, signingSalt, "abc", samplePath, false))

		// Wrong path
		validSignature := SignPath(signingKey, signingSalt, samplePath)
		require.False(t, VerifySignature(signingKey, signingSalt, validSignature, "/different/path", false))

		// Wrong key
		differentKey := []byte("different-key-012345678901234567")
		require.False(t, VerifySignature(differentKey, signingSalt, validSignature, samplePath, false))

		hmacHash := hmac.New(sha256.New, signingKey)
		hmacHash.Write(signingSalt)
		hmacHash.Write([]byte(samplePath))
		fullDigest := hmacHash.Sum(nil)

		// Padded standard base64 signature
		paddedSignature := base64.URLEncoding.EncodeToString(fullDigest)
		require.True(t, VerifySignature(signingKey, signingSalt, paddedSignature, samplePath, false))

		// Overlong signature (> 32 bytes)
		overlongBytes := make([]byte, 40)
		overlongSignature := base64.RawURLEncoding.EncodeToString(overlongBytes)
		require.False(t, VerifySignature(signingKey, signingSalt, overlongSignature, samplePath, false))

		// Malformed characters (not hex, not base64)
		require.False(t, VerifySignature(signingKey, signingSalt, "!@#$%%^&*()", samplePath, false))
	})
}
