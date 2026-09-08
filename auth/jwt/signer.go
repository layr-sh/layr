package jwt

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"
	"uuid"

	"layr.sh/core"
)

const (
	expectedTokenSegmentCount       = 3
	defaultAccessTokenExpirySeconds = 900
	defaultIDTokenExpirySeconds     = 3600
)

// Claims represents standard RFC 7519 JWT claims for layr/auth end-users.
type Claims struct {
	Subject     string         `json:"sub"`
	Email       string         `json:"email,omitempty"`
	Phone       string         `json:"phone,omitempty"`
	Role        string         `json:"role"`
	IsAnonymous bool           `json:"is_anonymous"`
	Issuer      string         `json:"iss"`
	Audience    string         `json:"aud"`
	ExpiresAt   int64          `json:"exp"`
	IssuedAt    int64          `json:"iat"`
	NotBefore   int64          `json:"nbf"`
	JWTID       string         `json:"jti"`
	Claims      map[string]any `json:"claims,omitempty"`
}

// Assert checks a single standard or custom claim against the expected value.
// Supported standard claim aliases:
// - "aud", "audience" -> claims.Audience
// - "iss", "issuer" -> claims.Issuer
// - "sub", "subject" -> claims.Subject
// - "role" -> claims.Role
// - "email" -> claims.Email
// - "phone" -> claims.Phone
// - "is_anonymous" -> claims.IsAnonymous
// - "jti" -> claims.JWTID
// All other keys are verified against custom claims.Claims.
func (claims *Claims) Assert(key string, expected any) error {
	var actual any
	switch key {
	case "aud", "audience":
		actual = claims.Audience
	case "iss", "issuer":
		actual = claims.Issuer
	case "sub", "subject":
		actual = claims.Subject
	case "role":
		actual = claims.Role
	case "email":
		actual = claims.Email
	case "phone":
		actual = claims.Phone
	case "is_anonymous":
		actual = claims.IsAnonymous
	case "jti":
		actual = claims.JWTID
	default:
		if claims.Claims == nil {
			return fmt.Errorf("jwt claim %q not found", key)
		}
		val, exists := claims.Claims[key]
		if !exists {
			return fmt.Errorf("jwt claim %q not found", key)
		}
		actual = val
	}

	if !assertValueEqual(actual, expected) {
		return fmt.Errorf("jwt claim %q mismatch: expected %v, got %v", key, expected, actual)
	}
	return nil
}

func assertValueEqual(actual, expected any) bool {
	if reflect.DeepEqual(actual, expected) {
		return true
	}

	actualNum, actualValid := toFloat64(actual)
	expectedNum, expectedValid := toFloat64(expected)
	if actualValid && expectedValid {
		return actualNum == expectedNum
	}

	return false
}

func toFloat64(val any) (float64, bool) {
	switch num := val.(type) {
	case int:
		return float64(num), true
	case int64:
		return float64(num), true
	case float64:
		return num, true
	default:
		return 0, false
	}
}

// TokenPair contains an access token and a refresh token.
type TokenPair struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// Signer handles Ed25519 token issuance and verification using subkey KEY_JWT_SIGNING.
type Signer struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	seed       []byte
	keyID      string
}

// NewSigner derives an Ed25519 keypair from KEY_JWT_SIGNING subkey.
// If customKeyID is provided, it uses it; otherwise it defaults to "<projectSlug>-ed25519-v1".
func NewSigner(keyManager *core.CryptoKeyManager, customKeyID ...string) (*Signer, error) {
	seed, _ := keyManager.DeriveSubkey(core.CryptoContextAuthJWTSigning)
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	resolvedKeyID := core.GetConfig().Project.Slug() + "-ed25519-v1"
	if len(customKeyID) > 0 && customKeyID[0] != "" {
		resolvedKeyID = customKeyID[0]
	}

	return &Signer{
		privateKey: privateKey,
		publicKey:  publicKey,
		seed:       seed,
		keyID:      resolvedKeyID,
	}, nil
}

// KeyID returns the configured Ed25519 key ID.
func (signer *Signer) KeyID() string {
	return signer.keyID
}

// PublicKey returns the Ed25519 public key.
func (signer *Signer) PublicKey() ed25519.PublicKey {
	return signer.publicKey
}

// GenerateAccessToken signs a standard Ed25519 JWT for an authenticated application user.
// Unset claims are populated with sensible defaults:
// - Role: "authenticated"
// - Issuer: core.GetConfig().Project.Slug()
// - Audience: core.GetConfig().Project.Slug() + ":user"
// - IssuedAt / NotBefore: current UTC timestamp
// - ExpiresAt: current UTC timestamp + expirySeconds (default 900s)
// - JWTID: UUIDv7 string
func (signer *Signer) GenerateAccessToken(claims Claims, expirySeconds ...int) (string, error) {
	if claims.Role == "" {
		claims.Role = "authenticated"
	}
	projectSlug := core.GetConfig().Project.Slug()
	if claims.Issuer == "" {
		claims.Issuer = projectSlug
	}
	if claims.Audience == "" {
		claims.Audience = projectSlug + ":user"
	}

	now := time.Now().UTC()
	if claims.IssuedAt == 0 {
		claims.IssuedAt = now.Unix()
	}
	if claims.NotBefore == 0 {
		claims.NotBefore = now.Unix()
	}
	if claims.ExpiresAt == 0 {
		expiry := defaultAccessTokenExpirySeconds
		if len(expirySeconds) > 0 && expirySeconds[0] != 0 {
			expiry = expirySeconds[0]
		}
		claims.ExpiresAt = now.Add(time.Duration(expiry) * time.Second).Unix()
	}
	if claims.JWTID == "" {
		claims.JWTID = uuid.NewV7().String()
	}

	header := map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
		"kid": signer.keyID,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsBase64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerBase64 + "." + claimsBase64
	signature := ed25519.Sign(signer.privateKey, []byte(signingInput))
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureBase64, nil
}

// VerifyAccessToken validates an Ed25519 JWT, asserting signature and expiration/nbf timestamps.
func (signer *Signer) VerifyAccessToken(token string) (*Claims, error) {
	tokenSegments := strings.Split(token, ".")
	if len(tokenSegments) != expectedTokenSegmentCount {
		return nil, errors.New("invalid token format: must contain 3 segments")
	}

	// 1. Verify signature
	signingInput := tokenSegments[0] + "." + tokenSegments[1]
	signature, err := base64.RawURLEncoding.DecodeString(tokenSegments[2])
	if err != nil {
		return nil, errors.New("invalid signature encoding")
	}

	if !ed25519.Verify(signer.publicKey, []byte(signingInput), signature) {
		return nil, errors.New("jwt signature verification failed")
	}

	// 2. Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(tokenSegments[1])
	if err != nil {
		return nil, fmt.Errorf("invalid claims base64: %w", err)
	}

	var claims Claims
	if err := json.Unmarshal(claimsJSON, &claims); err != nil {
		return nil, fmt.Errorf("invalid claims json: %w", err)
	}

	now := time.Now().UTC().Unix()
	if claims.ExpiresAt < now {
		return nil, errors.New("jwt token expired")
	}
	if claims.NotBefore > now {
		return nil, errors.New("jwt token not valid yet")
	}

	return &claims, nil
}

// GenerateRefreshToken generates an opaque random 32-byte hex refresh token.
func GenerateRefreshToken() string {
	rawUUIDs := uuid.NewV7().String() + uuid.NewV7().String()
	return strings.ReplaceAll(rawUUIDs, "-", "")
}

// HashRefreshToken produces SHA-256 hex string for database storage and index lookup.
func HashRefreshToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", digest)
}

// SignHMAC signs input string with seed using HMAC-SHA256 for one-time verification tokens.
func (signer *Signer) SignHMAC(message string) string {
	mac := hmac.New(sha256.New, signer.seed)
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC verifies HMAC-SHA256 signature in constant time.
func (signer *Signer) VerifyHMAC(message, expectedSignature string) bool {
	actualSignature := signer.SignHMAC(message)
	return hmac.Equal([]byte(actualSignature), []byte(expectedSignature))
}

// OIDCIDTokenClaims represents standard OpenID Connect Core 1.0 ID token claims.
type OIDCIDTokenClaims struct {
	Issuer              string `json:"iss"`
	Subject             string `json:"sub"`
	Audience            string `json:"aud"`
	ExpiresAt           int64  `json:"exp"`
	IssuedAt            int64  `json:"iat"`
	AuthTime            int64  `json:"auth_time,omitempty"`
	Nonce               string `json:"nonce,omitempty"`
	Email               string `json:"email,omitempty"`
	EmailVerified       bool   `json:"email_verified,omitempty"`
	PhoneNumber         string `json:"phone_number,omitempty"`
	PhoneNumberVerified bool   `json:"phone_number_verified,omitempty"`
	Role                string `json:"role,omitempty"`
	IsAnonymous         bool   `json:"is_anonymous,omitempty"`
}

// GenerateIDToken signs an OpenID Connect Core 1.0 ID token using Ed25519.
// Unset claims are populated with sensible defaults:
// - Role: "authenticated"
// - Issuer: core.GetConfig().Project.Slug()
// - IssuedAt / AuthTime: current UTC timestamp
// - ExpiresAt: current UTC timestamp + expirySeconds (default 3600s)
func (signer *Signer) GenerateIDToken(claims OIDCIDTokenClaims, expirySeconds ...int) (string, error) {
	if claims.Role == "" {
		claims.Role = "authenticated"
	}
	if claims.Issuer == "" {
		claims.Issuer = core.GetConfig().Project.Slug()
	}

	now := time.Now().UTC()
	if claims.IssuedAt == 0 {
		claims.IssuedAt = now.Unix()
	}
	if claims.AuthTime == 0 {
		claims.AuthTime = now.Unix()
	}
	if claims.ExpiresAt == 0 {
		expiry := defaultIDTokenExpirySeconds
		if len(expirySeconds) > 0 && expirySeconds[0] != 0 {
			expiry = expirySeconds[0]
		}
		claims.ExpiresAt = now.Add(time.Duration(expiry) * time.Second).Unix()
	}

	header := map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
		"kid": signer.keyID,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(claims)

	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsBase64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerBase64 + "." + claimsBase64
	signature := ed25519.Sign(signer.privateKey, []byte(signingInput))
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureBase64, nil
}
