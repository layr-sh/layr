package core

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
)

const (
	expectedTokenSegmentCount       = 3
	defaultAccessTokenExpirySeconds = 900
	defaultIDTokenExpirySeconds     = 3600
	defaultM2MTokenExpirySeconds    = 3600
)

// JWTClaims represents standard RFC 7519 and OIDC Core 1.0 JWT claims for platform users, service accounts, and tokens.
type JWTClaims struct {
	Subject       string         `json:"sub"`
	SessionID     string         `json:"sid,omitempty"`
	Email         string         `json:"email,omitempty"`
	EmailVerified bool           `json:"email_verified,omitempty"`
	Phone         string         `json:"phone,omitempty"`
	PhoneVerified bool           `json:"phone_verified,omitempty"`
	Role          string         `json:"role,omitempty"`
	IsAnonymous   bool           `json:"is_anonymous,omitempty"`
	Issuer        string         `json:"iss,omitempty"`
	Audience      string         `json:"aud,omitempty"`
	ExpiresAt     int64          `json:"exp,omitempty"`
	IssuedAt      int64          `json:"iat,omitempty"`
	NotBefore     int64          `json:"nbf,omitempty"`
	AuthTime      int64          `json:"auth_time,omitempty"`
	Nonce         string         `json:"nonce,omitempty"`
	JWTID         string         `json:"jti,omitempty"`
	Scope         string         `json:"scope,omitempty"`
	Claims        map[string]any `json:"claims,omitempty"`
}

// Scopes parses the space-separated scope claim into a slice of individual scopes.
func (jwtClaims JWTClaims) Scopes() []string {
	return strings.Fields(jwtClaims.Scope)
}

// HasScope checks if the JWT scope claim satisfies the required scope.
func (jwtClaims JWTClaims) HasScope(requiredScope string) bool {
	return HasScope(jwtClaims.Scopes(), requiredScope)
}

// Assert checks a single standard or custom claim against the expected value.
// Supported standard claim aliases:
// - "aud", "audience" -> claims.Audience
// - "iss", "issuer" -> claims.Issuer
// - "sub", "subject" -> claims.Subject
// - "sid", "session_id" -> claims.SessionID
// - "role" -> claims.Role
// - "email" -> claims.Email
// - "phone" -> claims.Phone
// - "is_anonymous" -> claims.IsAnonymous
// - "jti" -> claims.JWTID
// - "scopes" -> claims.Scopes
// All other keys are verified against custom claims.Claims.
func (jwtClaims *JWTClaims) Assert(key string, expected any) error {
	var actual any
	switch key {
	case "aud", "audience":
		actual = jwtClaims.Audience
	case "iss", "issuer":
		actual = jwtClaims.Issuer
	case "sub", "subject":
		actual = jwtClaims.Subject
	case "sid", "session_id":
		actual = jwtClaims.SessionID
	case "role":
		actual = jwtClaims.Role
	case "email":
		actual = jwtClaims.Email
	case "phone":
		actual = jwtClaims.Phone
	case "is_anonymous":
		actual = jwtClaims.IsAnonymous
	case "jti":
		actual = jwtClaims.JWTID
	case "scope":
		actual = jwtClaims.Scope
	case "scopes":
		actual = jwtClaims.Scopes()
	default:
		if jwtClaims.Claims == nil {
			return fmt.Errorf("jwt claim %q not found", key)
		}
		value, exists := jwtClaims.Claims[key]
		if !exists {
			return fmt.Errorf("jwt claim %q not found", key)
		}
		actual = value
	}

	log.Tracef("asserting claim key %q (expected %v)", key, expected)
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

func toFloat64(value any) (float64, bool) {
	switch num := value.(type) {
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

// JWTSigner handles Ed25519 token issuance and verification using subkey KEY_JWT_SIGNING.
type JWTSigner struct {
	privateKey ed25519.PrivateKey
	publicKey  ed25519.PublicKey
	seed       []byte
	keyID      string
}

// NewJWTSigner derives an Ed25519 keypair from KEY_JWT_SIGNING subkey.
// If customKeyID is provided, it uses it; otherwise it defaults to "<handle>-ed25519-v1".
func NewJWTSigner(cryptoKeyManager *CryptoKeyManager, customKeyID ...string) (*JWTSigner, error) {
	seed, _ := cryptoKeyManager.DeriveSubkey(CryptoContextAuthJWTSigning)
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	slugifier := NewSlugifier()
	handle := slugifier.Slugify(GetConfig().Project.Name)
	if handle == "" {
		handle = "layr-app"
	}

	resolvedKeyID := handle + "-ed25519-v1"
	if len(customKeyID) > 0 && customKeyID[0] != "" {
		resolvedKeyID = customKeyID[0]
	}

	log.Debugf("derived Ed25519 signer keypair with keyID %s", resolvedKeyID)
	return &JWTSigner{
		privateKey: privateKey,
		publicKey:  publicKey,
		seed:       seed,
		keyID:      resolvedKeyID,
	}, nil
}

// KeyID returns the configured Ed25519 key ID.
func (signer *JWTSigner) KeyID() string {
	return signer.keyID
}

// PublicKey returns the Ed25519 public key.
func (signer *JWTSigner) PublicKey() ed25519.PublicKey {
	return signer.publicKey
}

// GenerateAccessToken signs a standard Ed25519 JWT for an authenticated application user.
// Unset claims are populated with sensible defaults:
// - Role: "authenticated"
// - Issuer: handle (slugified project name)
// - Audience: <handle>:user
// - IssuedAt / NotBefore: current UTC timestamp
// - ExpiresAt: current UTC timestamp + expirySeconds (default 900s)
// - JWTID: UUIDv7 string
func (signer *JWTSigner) GenerateAccessToken(jwtClaims JWTClaims, expirySeconds ...int) (string, error) {
	if jwtClaims.Role == "" {
		jwtClaims.Role = "authenticated"
	}
	slugifier := NewSlugifier()
	handle := slugifier.Slugify(GetConfig().Project.Name)
	if handle == "" {
		handle = "layr-app"
	}
	if jwtClaims.Issuer == "" {
		jwtClaims.Issuer = handle
	}
	if jwtClaims.Audience == "" {
		jwtClaims.Audience = handle + ":user"
	}

	now := time.Now().UTC()
	if jwtClaims.IssuedAt == 0 {
		jwtClaims.IssuedAt = now.Unix()
	}
	if jwtClaims.NotBefore == 0 {
		jwtClaims.NotBefore = now.Unix()
	}
	if jwtClaims.ExpiresAt == 0 {
		effectiveExpirySeconds := defaultAccessTokenExpirySeconds
		if len(expirySeconds) > 0 && expirySeconds[0] > 0 {
			effectiveExpirySeconds = expirySeconds[0]
		}
		jwtClaims.ExpiresAt = now.Add(time.Duration(effectiveExpirySeconds) * time.Second).Unix()
	}
	if jwtClaims.JWTID == "" {
		jwtClaims.JWTID = uuid.NewV7().String()
	}

	keyID := signer.keyID
	if keyID == "" {
		keyID = "layr-ed25519-v1"
	}

	header := map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
		"kid": keyID,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(jwtClaims)

	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsBase64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerBase64 + "." + claimsBase64
	log.Tracef("signing access token for subject %s with keyID %s", jwtClaims.Subject, signer.keyID)
	signature := ed25519.Sign(signer.privateKey, []byte(signingInput))
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	log.Debugf("issued access token for subject %s (kid: %s)", jwtClaims.Subject, signer.keyID)
	return signingInput + "." + signatureBase64, nil
}

// VerifyAccessToken validates an Ed25519 JWT, asserting signature and expiration/nbf timestamps.
func (signer *JWTSigner) VerifyAccessToken(token string) (*JWTClaims, error) {
	log.Tracef("verifying access token with keyID %s", signer.keyID)
	tokenSegments := strings.Split(token, ".")
	if len(tokenSegments) != expectedTokenSegmentCount {
		log.Debugf("jwt verification failed: invalid segment count %d", len(tokenSegments))
		return nil, errors.New("invalid token format: must contain 3 segments")
	}

	// 1. Verify signature
	signingInput := tokenSegments[0] + "." + tokenSegments[1]
	signature, err := base64.RawURLEncoding.DecodeString(tokenSegments[2])
	if err != nil {
		log.Debugf("jwt verification failed: invalid signature encoding: %v", err)
		return nil, errors.New("invalid signature encoding")
	}

	if !ed25519.Verify(signer.publicKey, []byte(signingInput), signature) {
		log.Debugf("jwt verification failed: Ed25519 signature mismatch for keyID %s", signer.keyID)
		return nil, errors.New("jwt signature verification failed")
	}

	// 2. Decode claims
	claimsJSON, err := base64.RawURLEncoding.DecodeString(tokenSegments[1])
	if err != nil {
		log.Debugf("jwt verification failed: invalid claims base64: %v", err)
		return nil, fmt.Errorf("invalid claims base64: %w", err)
	}

	var jwtClaims JWTClaims
	if err := json.Unmarshal(claimsJSON, &jwtClaims); err != nil {
		log.Debugf("jwt verification failed: invalid claims json: %v", err)
		return nil, fmt.Errorf("invalid claims json: %w", err)
	}

	now := time.Now().UTC().Unix()
	if jwtClaims.ExpiresAt < now {
		log.Debugf("jwt verification failed: token expired at %d (current: %d)", jwtClaims.ExpiresAt, now)
		return nil, errors.New("jwt token expired")
	}
	if jwtClaims.NotBefore > now {
		log.Debugf("jwt verification failed: token not valid until %d (current: %d)", jwtClaims.NotBefore, now)
		return nil, errors.New("jwt token not valid yet")
	}

	log.Debugf("verified access token for subject %s (kid: %s)", jwtClaims.Subject, signer.keyID)
	return &jwtClaims, nil
}

// GenerateRefreshToken generates an opaque random 32-byte hex refresh token.
func (signer *JWTSigner) GenerateRefreshToken() string {
	rawUUIDs := uuid.NewV7().String() + uuid.NewV7().String()
	return strings.ReplaceAll(rawUUIDs, "-", "")
}

// HashRefreshToken produces SHA-256 hex string for database storage and index lookup.
func (signer *JWTSigner) HashRefreshToken(token string) string {
	digest := sha256.Sum256([]byte(token))
	return fmt.Sprintf("%x", digest)
}

// SignHMAC signs input string with seed using HMAC-SHA256 for one-time verification tokens.
func (signer *JWTSigner) SignHMAC(message string) string {
	log.Tracef("signing message with HMAC-SHA256")
	mac := hmac.New(sha256.New, signer.seed) //nolint:namingclarity
	mac.Write([]byte(message))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// VerifyHMAC verifies HMAC-SHA256 signature in constant time.
func (signer *JWTSigner) VerifyHMAC(message, expectedSignature string) bool {
	log.Tracef("verifying HMAC-SHA256 signature")
	actualSignature := signer.SignHMAC(message)
	return hmac.Equal([]byte(actualSignature), []byte(expectedSignature))
}

// GenerateIDToken signs an OpenID Connect Core 1.0 ID token using Ed25519.
// Unset claims are populated with sensible defaults:
// - Role: "authenticated"
// - Issuer: handle (slugified project name)
// - IssuedAt / AuthTime: current UTC timestamp
// - ExpiresAt: current UTC timestamp + expirySeconds (default 3600s)
func (signer *JWTSigner) GenerateIDToken(jwtClaims JWTClaims, expirySeconds ...int) (string, error) {
	if jwtClaims.Role == "" {
		jwtClaims.Role = "authenticated"
	}
	if jwtClaims.Issuer == "" {
		slugifier := NewSlugifier()
		handle := slugifier.Slugify(GetConfig().Project.Name)
		if handle == "" {
			handle = "layr-app"
		}
		jwtClaims.Issuer = handle
	}

	now := time.Now().UTC()
	if jwtClaims.IssuedAt == 0 {
		jwtClaims.IssuedAt = now.Unix()
	}
	if jwtClaims.AuthTime == 0 {
		jwtClaims.AuthTime = now.Unix()
	}
	if jwtClaims.ExpiresAt == 0 {
		expiry := defaultIDTokenExpirySeconds
		if len(expirySeconds) > 0 && expirySeconds[0] != 0 {
			expiry = expirySeconds[0]
		}
		jwtClaims.ExpiresAt = now.Add(time.Duration(expiry) * time.Second).Unix()
	}

	header := map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
		"kid": signer.keyID,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(jwtClaims)

	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsBase64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerBase64 + "." + claimsBase64
	log.Tracef("signing ID token for subject %s with keyID %s", jwtClaims.Subject, signer.keyID)
	signature := ed25519.Sign(signer.privateKey, []byte(signingInput))
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	log.Debugf("issued ID token for subject %s (kid: %s)", jwtClaims.Subject, signer.keyID)
	return signingInput + "." + signatureBase64, nil
}

// GenerateM2MToken signs a machine-to-machine OAuth 2.0 Client Credentials token.
// Headers: {"alg": "EdDSA", "typ": "JWT", "kid": <keyID>}
// Claims:
// - sub: serviceAccountID
// - role: "service_role"
// - iss: handle (slugified project name)
// - aud: audience
// - scope: space-separated granted scopes
// - exp: current UTC timestamp + expirySeconds (default 3600s)
// - iat: current UTC timestamp
// - jti: UUIDv7 string
func (signer *JWTSigner) GenerateM2MToken(serviceAccountID string, scopes []string, expirySeconds int, audience string) (string, error) {
	if serviceAccountID == "" {
		return "", errors.New("service account ID is required")
	}
	if len(signer.privateKey) == 0 {
		return "", errors.New("signer private key is not initialized")
	}
	if scopes == nil {
		scopes = []string{}
	}
	if expirySeconds <= 0 {
		expirySeconds = defaultM2MTokenExpirySeconds
	}

	slugifier := NewSlugifier()
	handle := slugifier.Slugify(GetConfig().Project.Name)
	if handle == "" {
		handle = "layr"
	}

	now := time.Now().UTC()
	m2mJWTClaims := JWTClaims{
		Subject:   serviceAccountID,
		Role:      "service_role",
		Issuer:    handle,
		Audience:  audience,
		Scope:     strings.Join(scopes, " "),
		ExpiresAt: now.Add(time.Duration(expirySeconds) * time.Second).Unix(),
		IssuedAt:  now.Unix(),
		JWTID:     uuid.NewV7().String(),
	}

	keyID := signer.keyID
	if keyID == "" {
		keyID = "layr-ed25519-v1"
	}

	header := map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
		"kid": keyID,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(m2mJWTClaims)

	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsBase64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerBase64 + "." + claimsBase64
	log.Tracef("signing M2M token for service account %s with keyID %s", serviceAccountID, keyID)
	signature := ed25519.Sign(signer.privateKey, []byte(signingInput))
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	log.Debugf("issued M2M token for service account %s (kid: %s)", serviceAccountID, keyID)
	return signingInput + "." + signatureBase64, nil
}

// VerifyM2MToken validates an Ed25519 M2M JWT, asserting signature and expiration timestamps.
func (signer *JWTSigner) VerifyM2MToken(token string) (*JWTClaims, error) {
	log.Tracef("verifying M2M token with keyID %s", signer.keyID)
	tokenSegments := strings.Split(token, ".")
	if len(tokenSegments) != expectedTokenSegmentCount {
		log.Debugf("m2m jwt verification failed: invalid segment count %d", len(tokenSegments))
		return nil, errors.New("invalid token format: must contain 3 segments")
	}

	// 1. Verify signature
	signingInput := tokenSegments[0] + "." + tokenSegments[1]
	signature, signatureDecodeErr := base64.RawURLEncoding.DecodeString(tokenSegments[2])
	if signatureDecodeErr != nil {
		log.Debugf("m2m jwt verification failed: invalid signature encoding: %v", signatureDecodeErr)
		return nil, errors.New("invalid signature encoding")
	}

	if !ed25519.Verify(signer.publicKey, []byte(signingInput), signature) {
		log.Debugf("m2m jwt verification failed: Ed25519 signature mismatch for keyID %s", signer.keyID)
		return nil, errors.New("jwt signature verification failed")
	}

	// 2. Decode claims
	claimsJSON, claimsDecodeErr := base64.RawURLEncoding.DecodeString(tokenSegments[1])
	if claimsDecodeErr != nil {
		log.Debugf("m2m jwt verification failed: invalid claims base64: %v", claimsDecodeErr)
		return nil, fmt.Errorf("invalid claims base64: %w", claimsDecodeErr)
	}

	var m2mJWTClaims JWTClaims
	if claimsUnmarshalErr := json.Unmarshal(claimsJSON, &m2mJWTClaims); claimsUnmarshalErr != nil {
		log.Debugf("m2m jwt verification failed: invalid claims json: %v", claimsUnmarshalErr)
		return nil, fmt.Errorf("invalid claims json: %w", claimsUnmarshalErr)
	}

	now := time.Now().UTC().Unix()
	if m2mJWTClaims.ExpiresAt < now {
		log.Debugf("m2m jwt verification failed: token expired at %d (current: %d)", m2mJWTClaims.ExpiresAt, now)
		return nil, errors.New("jwt token expired")
	}

	log.Debugf("verified M2M token for subject %s (kid: %s)", m2mJWTClaims.Subject, signer.keyID)
	return &m2mJWTClaims, nil
}
