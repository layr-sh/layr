package core

import (
	"bytes"
	"testing"
	"uuid"
)

func TestCoreJWTSignerCryptoKeyManagerDerivationIntegration(t *testing.T) {
	primaryMasterEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	primaryCryptoKeyManager, err := NewCryptoKeyManager(primaryMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create primary CryptoKeyManager: %v", err)
	}

	secondaryCryptoKeyManager, err := NewCryptoKeyManager(primaryMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create secondary CryptoKeyManager: %v", err)
	}

	primaryJWTSigner := NewJWTSigner(primaryCryptoKeyManager)
	secondaryJWTSigner := NewJWTSigner(secondaryCryptoKeyManager)

	// 1. Both signers derived from the same master key must have identical public keys
	if !bytes.Equal(primaryJWTSigner.PublicKey(), secondaryJWTSigner.PublicKey()) {
		t.Fatal("expected identical public keys from identical master keys")
	}

	// 2. Token signed by primary signer must verify on secondary signer
	applicationUserID := uuid.NewV7().String()
	userClaims := map[string]any{
		"tier":        "enterprise",
		"permissions": []any{"read:documents", "write:documents"},
	}
	signedToken := primaryJWTSigner.GenerateAccessToken(JWTClaims{
		Subject: applicationUserID,
		Email:   "alice@example.com",
		Phone:   "+10000000000",
		Claims:  userClaims,
	}, 600)

	jwtClaims, verifyErr := secondaryJWTSigner.VerifyAccessToken(signedToken)
	if verifyErr != nil {
		t.Fatalf("secondary signer failed to verify primary signer token: %v", verifyErr)
	}
	if jwtClaims.Subject != applicationUserID || jwtClaims.Email != "alice@example.com" {
		t.Fatalf("claims subject mismatch: expected %s, got %s", applicationUserID, jwtClaims.Subject)
	}

	// 3. HMAC signature signed by primary signer must verify on secondary signer
	verificationMessage := "verify-session-nonce-12345"
	hmacSignature := primaryJWTSigner.SignHMAC(verificationMessage)
	if !secondaryJWTSigner.VerifyHMAC(verificationMessage, hmacSignature) {
		t.Fatal("secondary signer failed to verify primary signer HMAC signature")
	}
}

func TestCoreJWTSignerCrossTenantIsolationIntegration(t *testing.T) {
	tenantOneCryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create tenant 1 key manager: %v", err)
	}

	tenantTwoCryptoKeyManager, err := NewCryptoKeyManager("fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210")
	if err != nil {
		t.Fatalf("failed to create tenant 2 key manager: %v", err)
	}

	tenantOneJWTSigner := NewJWTSigner(tenantOneCryptoKeyManager)
	tenantTwoJWTSigner := NewJWTSigner(tenantTwoCryptoKeyManager)

	// 1. Different master keys produce distinct public keys
	if bytes.Equal(tenantOneJWTSigner.PublicKey(), tenantTwoJWTSigner.PublicKey()) {
		t.Fatal("expected distinct public keys for distinct master keys")
	}

	// 2. Token signed by tenant 1 must fail verification on tenant 2
	applicationUserID := uuid.NewV7().String()
	tokenTenantOne := tenantOneJWTSigner.GenerateAccessToken(JWTClaims{
		Subject: applicationUserID,
		Email:   "bob@example.com",
	}, 600)

	if _, verifyErr := tenantTwoJWTSigner.VerifyAccessToken(tokenTenantOne); verifyErr == nil {
		t.Fatal("expected signature verification failure when tenant 2 verifies tenant 1 token")
	}

	// 3. HMAC signed by tenant 1 must fail verification on tenant 2
	verificationPayload := "cross-tenant-message"
	signatureTenantOne := tenantOneJWTSigner.SignHMAC(verificationPayload)
	if tenantTwoJWTSigner.VerifyHMAC(verificationPayload, signatureTenantOne) {
		t.Fatal("expected HMAC verification failure when tenant 2 verifies tenant 1 HMAC signature")
	}
}

func TestCoreJWTSignerClaimsRoundtripIntegration(t *testing.T) {
	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create CryptoKeyManager: %v", err)
	}
	jwtSigner := NewJWTSigner(cryptoKeyManager)

	applicationUserID := uuid.NewV7().String()
	userClaims := map[string]any{
		"is_active": true,
		"preferences": map[string]any{
			"theme": "dark",
		},
	}

	token := jwtSigner.GenerateAccessToken(JWTClaims{
		Subject: applicationUserID,
		Email:   "charlie@example.com",
		Phone:   "+19876543210",
		Role:    "developer",
		Claims:  userClaims,
	}, 1200)

	jwtClaims, verifyErr := jwtSigner.VerifyAccessToken(token)
	if verifyErr != nil {
		t.Fatalf("failed to verify token: %v", verifyErr)
	}

	if jwtClaims.Subject != applicationUserID {
		t.Fatalf("subject mismatch: expected %s, got %s", applicationUserID, jwtClaims.Subject)
	}
	if jwtClaims.Role != "developer" {
		t.Fatalf("role mismatch: expected developer, got %s", jwtClaims.Role)
	}
	if jwtClaims.Email != "charlie@example.com" {
		t.Fatalf("email mismatch: expected charlie@example.com, got %s", jwtClaims.Email)
	}
	if jwtClaims.Phone != "+19876543210" {
		t.Fatalf("phone mismatch: expected +19876543210, got %s", jwtClaims.Phone)
	}
	if jwtClaims.Claims["is_active"] != true {
		t.Fatalf("claims mismatch: expected is_active true, got %v", jwtClaims.Claims["is_active"])
	}

	// Refresh token uniqueness integration
	seenTokens := make(map[string]bool)
	seenHashes := make(map[string]bool)
	for i := 0; i < 50; i++ {
		refreshToken := jwtSigner.GenerateRefreshToken()
		if seenTokens[refreshToken] {
			t.Fatalf("duplicate refresh token generated: %s", refreshToken)
		}
		seenTokens[refreshToken] = true

		tokenHash := jwtSigner.HashRefreshToken(refreshToken)
		if seenHashes[tokenHash] {
			t.Fatalf("duplicate refresh token hash produced: %s", tokenHash)
		}
		seenHashes[tokenHash] = true
	}
}

func TestCoreJWTProjectIsolationIntegration(t *testing.T) {
	cryptoKeyManager, err := NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create CryptoKeyManager: %v", err)
	}

	appJWTSigner := NewJWTSigner(cryptoKeyManager, "portal-key-v1")
	consoleUserJWTSigner := NewJWTSigner(cryptoKeyManager, "admin-key-v1")

	// 1. Both share the same underlying key derivation
	if !bytes.Equal(appJWTSigner.PublicKey(), consoleUserJWTSigner.PublicKey()) {
		t.Fatal("expected identical public keys for identical crypto key managers")
	}

	// 2. Generate token with appSigner for Customer Portal audience and issuer
	userID := uuid.NewV7().String()
	appToken := appJWTSigner.GenerateAccessToken(JWTClaims{
		Subject:  userID,
		Email:    "user@portal.com",
		Audience: "customer-portal:user",
		Issuer:   "customer-portal",
	}, 600)

	// 3. appSigner successfully verifies token with matching audience and issuer
	verifiedJWTClaims, verifyPortalTokenErr := appJWTSigner.VerifyAccessToken(appToken)
	if verifyPortalTokenErr != nil {
		t.Fatalf("appSigner failed to verify own token: %v", verifyPortalTokenErr)
	}
	if assertErr := verifiedJWTClaims.Assert("aud", "customer-portal:user"); assertErr != nil {
		t.Fatalf("failed to assert portal aud: %v", assertErr)
	}
	if assertErr := verifiedJWTClaims.Assert("iss", "customer-portal"); assertErr != nil {
		t.Fatalf("failed to assert portal iss: %v", assertErr)
	}

	// 4. Verifier expecting Console audience fails assertion
	consoleUserJWTClaims, crossVerifyErr := consoleUserJWTSigner.VerifyAccessToken(appToken)
	if crossVerifyErr != nil {
		t.Fatalf("unexpected signature verification failure: %v", crossVerifyErr)
	}
	if assertErr := consoleUserJWTClaims.Assert("aud", "admin-console:user"); assertErr == nil {
		t.Fatal("expected audience assertion failure when verifying with different audience")
	}
}

func TestCoreJWTM2MTokenIntegration(t *testing.T) {
	masterKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	cryptoKeyManager, err := NewCryptoKeyManager(masterKeyHex)
	if err != nil {
		t.Fatalf("failed to create crypto key manager: %v", err)
	}

	jwtSigner := NewJWTSigner(cryptoKeyManager, "layr-ed25519-v1")

	serviceAccountID := uuid.NewV7().String()
	scopes := []string{"auth:read", "data:write"}
	slugifier := NewSlugifier()
	expectedHandle := slugifier.Slugify(GetConfig().Project.Name)
	if expectedHandle == "" {
		expectedHandle = "layr"
	}
	expectedAudience := expectedHandle + ":service_account"
	m2mToken, err := jwtSigner.GenerateM2MToken(serviceAccountID, scopes, 1800, expectedAudience)
	if err != nil {
		t.Fatalf("failed to generate M2M token: %v", err)
	}

	m2mJWTClaims, err := jwtSigner.VerifyM2MToken(m2mToken)
	if err != nil {
		t.Fatalf("failed to verify M2M token: %v", err)
	}

	if m2mJWTClaims.Subject != serviceAccountID {
		t.Errorf("expected subject %s, got %s", serviceAccountID, m2mJWTClaims.Subject)
	}
	if m2mJWTClaims.Issuer != expectedHandle {
		t.Errorf("expected issuer %s, got %s", expectedHandle, m2mJWTClaims.Issuer)
	}
	if m2mJWTClaims.Audience != expectedAudience {
		t.Errorf("expected audience %s, got %s", expectedAudience, m2mJWTClaims.Audience)
	}
	if len(m2mJWTClaims.Scopes()) != 2 || m2mJWTClaims.Scopes()[0] != "auth:read" || m2mJWTClaims.Scopes()[1] != "data:write" {
		t.Errorf("expected scopes %v, got %v", scopes, m2mJWTClaims.Scopes())
	}
	if m2mJWTClaims.Scope != "auth:read data:write" {
		t.Errorf("expected scope 'auth:read data:write', got %q", m2mJWTClaims.Scope)
	}
}
