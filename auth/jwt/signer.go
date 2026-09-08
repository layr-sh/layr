package jwt

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
	"uuid"

	"layr.sh/core"
)

// App user token issuer, audience, and key ID constants.
const (
	IssuerAppUser   = "layr"
	AudienceAppUser = "layr:user"
	KeyIDEd25519    = "layr-ed25519-v1"
)

const (
	expectedTokenSegmentCount = 3
)

// AppUserClaims represents standard RFC 7519 JWT claims for layr/auth end-users.
type AppUserClaims struct {
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
}

// NewSigner derives an Ed25519 keypair from KEY_JWT_SIGNING subkey.
func NewSigner(keyManager *core.CryptoKeyManager) (*Signer, error) {
	seed, _ := keyManager.DeriveSubkey(core.CryptoContextAuthJWTSigning)
	privateKey := ed25519.NewKeyFromSeed(seed)
	publicKey := privateKey.Public().(ed25519.PublicKey)

	return &Signer{
		privateKey: privateKey,
		publicKey:  publicKey,
		seed:       seed,
	}, nil
}

// PublicKey returns the Ed25519 public key.
func (signer *Signer) PublicKey() ed25519.PublicKey {
	return signer.publicKey
}

// GenerateAccessToken signs a standard Ed25519 JWT for an authenticated application user.
func (signer *Signer) GenerateAccessToken(userID, email, phone, role string, isAnonymous bool, claims map[string]any, expirySeconds int) (string, error) {
	if role == "" {
		role = "authenticated"
	}
	if expirySeconds == 0 {
		expirySeconds = 900
	}

	now := time.Now().UTC()
	userClaims := AppUserClaims{
		Subject:     userID,
		Email:       email,
		Phone:       phone,
		Role:        role,
		IsAnonymous: isAnonymous,
		Issuer:      IssuerAppUser,
		Audience:    AudienceAppUser,
		IssuedAt:    now.Unix(),
		NotBefore:   now.Unix(),
		ExpiresAt:   now.Add(time.Duration(expirySeconds) * time.Second).Unix(),
		JWTID:       uuid.NewV7().String(),
		Claims:      claims,
	}

	header := map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
		"kid": KeyIDEd25519,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(userClaims)

	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsBase64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerBase64 + "." + claimsBase64
	signature := ed25519.Sign(signer.privateKey, []byte(signingInput))
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureBase64, nil
}

// VerifyAccessToken validates an Ed25519 JWT, asserting signature, expiration, and audience.
func (signer *Signer) VerifyAccessToken(token string) (*AppUserClaims, error) {
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

	var claims AppUserClaims
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
	if claims.Audience != AudienceAppUser {
		return nil, fmt.Errorf("invalid jwt audience: expected %s, got %s", AudienceAppUser, claims.Audience)
	}
	if claims.Issuer != IssuerAppUser {
		return nil, fmt.Errorf("invalid jwt issuer: expected %s, got %s", IssuerAppUser, claims.Issuer)
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
func (signer *Signer) GenerateIDToken(issuer, clientID, userID, email, phone, role, nonce string, emailVerified, phoneVerified, isAnonymous bool, expirySeconds int) (string, error) {
	if role == "" {
		role = "authenticated"
	}
	if expirySeconds == 0 {
		expirySeconds = 3600
	}

	now := time.Now().UTC()
	idClaims := OIDCIDTokenClaims{
		Issuer:              issuer,
		Subject:             userID,
		Audience:            clientID,
		IssuedAt:            now.Unix(),
		AuthTime:            now.Unix(),
		ExpiresAt:           now.Add(time.Duration(expirySeconds) * time.Second).Unix(),
		Nonce:               nonce,
		Email:               email,
		EmailVerified:       emailVerified,
		PhoneNumber:         phone,
		PhoneNumberVerified: phoneVerified,
		Role:                role,
		IsAnonymous:         isAnonymous,
	}

	header := map[string]string{
		"alg": "EdDSA",
		"typ": "JWT",
		"kid": KeyIDEd25519,
	}

	headerJSON, _ := json.Marshal(header)
	claimsJSON, _ := json.Marshal(idClaims)

	headerBase64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	claimsBase64 := base64.RawURLEncoding.EncodeToString(claimsJSON)

	signingInput := headerBase64 + "." + claimsBase64
	signature := ed25519.Sign(signer.privateKey, []byte(signingInput))
	signatureBase64 := base64.RawURLEncoding.EncodeToString(signature)

	return signingInput + "." + signatureBase64, nil
}
