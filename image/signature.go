package image

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"strings"
)

// SignPath computes an imgproxy-compatible HMAC-SHA256 signature for a path.
func SignPath(signingKey []byte, signingSalt []byte, path string) string {
	hmacHash := hmac.New(sha256.New, signingKey)
	hmacHash.Write(signingSalt)
	hmacHash.Write([]byte(path))
	digest := hmacHash.Sum(nil)
	return base64.RawURLEncoding.EncodeToString(digest)
}

// VerifySignature validates a provided signature against the expected HMAC-SHA256 digest of the path.
// It supports URL-safe Base64 (unpadded or padded), Hex, truncated signatures (1-32 bytes),
// and insecure placeholders ("_" or "unsafe") when allowed by configuration.
func VerifySignature(signingKey []byte, signingSalt []byte, providedSignature string, path string, allowInsecure bool) bool {
	trimmedSignature := strings.TrimSpace(providedSignature)
	if trimmedSignature == "" {
		return false
	}

	// 1. Insecure Mode Evaluation
	if allowInsecure && (trimmedSignature == "_" || trimmedSignature == "unsafe") {
		return true
	}

	if len(signingKey) == 0 {
		return false
	}

	// 2. Compute Expected Digest
	hmacHash := hmac.New(sha256.New, signingKey)
	hmacHash.Write(signingSalt)
	hmacHash.Write([]byte(path))
	expectedBytes := hmacHash.Sum(nil)

	// 3. Decode Provided Signature (Hex or Base64URL)
	var providedBytes []byte
	const sha256HexLength = 64
	if len(trimmedSignature) == sha256HexLength {
		hexBytes, hexErr := hex.DecodeString(trimmedSignature)
		if hexErr == nil {
			providedBytes = hexBytes
		}
	}

	if len(providedBytes) == 0 {
		decodedBytes, base64Err := base64.RawURLEncoding.DecodeString(trimmedSignature)
		if base64Err == nil {
			providedBytes = decodedBytes
		} else {
			decodedStandardBytes, standardBase64Err := base64.URLEncoding.DecodeString(trimmedSignature)
			if standardBase64Err == nil {
				providedBytes = decodedStandardBytes
			} else {
				return false
			}
		}
	}

	// 4. Truncated Signature Comparison (1 to 32 bytes)
	providedLength := len(providedBytes)
	if providedLength == 0 || providedLength > len(expectedBytes) {
		return false
	}

	return subtle.ConstantTimeCompare(providedBytes, expectedBytes[:providedLength]) == 1
}
