package jwt

import (
	"bytes"
	"testing"
	"uuid"

	"layr.sh/core"
)

func TestJWTSignerCryptoKeyManagerDerivationIntegration(t *testing.T) {
	primaryMasterEncryptionKeyHex := "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	primaryCryptoKeyManager, err := core.NewCryptoKeyManager(primaryMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create primary CryptoKeyManager: %v", err)
	}

	secondaryCryptoKeyManager, err := core.NewCryptoKeyManager(primaryMasterEncryptionKeyHex)
	if err != nil {
		t.Fatalf("failed to create secondary CryptoKeyManager: %v", err)
	}

	primarySigner, err := NewSigner(primaryCryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create primary Signer: %v", err)
	}

	secondarySigner, err := NewSigner(secondaryCryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create secondary Signer: %v", err)
	}

	// 1. Both signers derived from the same master key must have identical public keys
	if !bytes.Equal(primarySigner.PublicKey(), secondarySigner.PublicKey()) {
		t.Fatal("expected identical public keys from identical master keys")
	}

	// 2. Token signed by primary signer must verify on secondary signer
	applicationUserID := uuid.NewV7().String()
	userClaims := map[string]any{
		"tier":        "enterprise",
		"permissions": []any{"read:documents", "write:documents"},
	}
	signedToken, generateErr := primarySigner.GenerateAccessToken(Claims{
		Subject: applicationUserID,
		Email:   "alice@example.com",
		Phone:   "+10000000000",
		Claims:  userClaims,
	}, 600)
	if generateErr != nil {
		t.Fatalf("failed to generate access token: %v", generateErr)
	}

	claims, verifyErr := secondarySigner.VerifyAccessToken(signedToken)
	if verifyErr != nil {
		t.Fatalf("secondary signer failed to verify primary signer token: %v", verifyErr)
	}
	if claims.Subject != applicationUserID || claims.Email != "alice@example.com" {
		t.Fatalf("claims subject mismatch: expected %s, got %s", applicationUserID, claims.Subject)
	}

	// 3. HMAC signature signed by primary signer must verify on secondary signer
	verificationMessage := "verify-session-nonce-12345"
	hmacSignature := primarySigner.SignHMAC(verificationMessage)
	if !secondarySigner.VerifyHMAC(verificationMessage, hmacSignature) {
		t.Fatal("secondary signer failed to verify primary signer HMAC signature")
	}
}

func TestJWTSignerCrossTenantIsolationIntegration(t *testing.T) {
	tenantOneCryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create tenant 1 key manager: %v", err)
	}

	tenantTwoCryptoKeyManager, err := core.NewCryptoKeyManager("fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210")
	if err != nil {
		t.Fatalf("failed to create tenant 2 key manager: %v", err)
	}

	tenantOneSigner, err := NewSigner(tenantOneCryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create tenant 1 signer: %v", err)
	}

	tenantTwoSigner, err := NewSigner(tenantTwoCryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create tenant 2 signer: %v", err)
	}

	// 1. Different master keys produce distinct public keys
	if bytes.Equal(tenantOneSigner.PublicKey(), tenantTwoSigner.PublicKey()) {
		t.Fatal("expected distinct public keys for distinct master keys")
	}

	// 2. Token signed by tenant 1 must fail verification on tenant 2
	applicationUserID := uuid.NewV7().String()
	tokenTenantOne, generateErr := tenantOneSigner.GenerateAccessToken(Claims{
		Subject: applicationUserID,
		Email:   "bob@example.com",
	}, 600)
	if generateErr != nil {
		t.Fatalf("failed to generate token: %v", generateErr)
	}

	if _, verifyErr := tenantTwoSigner.VerifyAccessToken(tokenTenantOne); verifyErr == nil {
		t.Fatal("expected signature verification failure when tenant 2 verifies tenant 1 token")
	}

	// 3. HMAC signed by tenant 1 must fail verification on tenant 2
	verificationPayload := "cross-tenant-message"
	signatureTenantOne := tenantOneSigner.SignHMAC(verificationPayload)
	if tenantTwoSigner.VerifyHMAC(verificationPayload, signatureTenantOne) {
		t.Fatal("expected HMAC verification failure when tenant 2 verifies tenant 1 HMAC signature")
	}
}

func TestJWTSignerClaimsRoundtripIntegration(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create CryptoKeyManager: %v", err)
	}
	signer, err := NewSigner(cryptoKeyManager)
	if err != nil {
		t.Fatalf("failed to create Signer: %v", err)
	}

	applicationUserID := uuid.NewV7().String()
	userClaims := map[string]any{
		"is_active": true,
		"preferences": map[string]any{
			"theme": "dark",
		},
	}

	token, generateErr := signer.GenerateAccessToken(Claims{
		Subject: applicationUserID,
		Email:   "charlie@example.com",
		Phone:   "+19876543210",
		Role:    "developer",
		Claims:  userClaims,
	}, 1200)
	if generateErr != nil {
		t.Fatalf("failed to generate token: %v", generateErr)
	}

	claims, verifyErr := signer.VerifyAccessToken(token)
	if verifyErr != nil {
		t.Fatalf("failed to verify token: %v", verifyErr)
	}

	if claims.Subject != applicationUserID {
		t.Fatalf("subject mismatch: expected %s, got %s", applicationUserID, claims.Subject)
	}
	if claims.Role != "developer" {
		t.Fatalf("role mismatch: expected developer, got %s", claims.Role)
	}
	if claims.Email != "charlie@example.com" {
		t.Fatalf("email mismatch: expected charlie@example.com, got %s", claims.Email)
	}
	if claims.Phone != "+19876543210" {
		t.Fatalf("phone mismatch: expected +19876543210, got %s", claims.Phone)
	}
	if claims.Claims["is_active"] != true {
		t.Fatalf("claims mismatch: expected is_active true, got %v", claims.Claims["is_active"])
	}

	// Refresh token uniqueness integration
	seenTokens := make(map[string]bool)
	seenHashes := make(map[string]bool)
	for i := 0; i < 50; i++ {
		refreshToken := GenerateRefreshToken()
		if seenTokens[refreshToken] {
			t.Fatalf("duplicate refresh token generated: %s", refreshToken)
		}
		seenTokens[refreshToken] = true

		tokenHash := HashRefreshToken(refreshToken)
		if seenHashes[tokenHash] {
			t.Fatalf("duplicate refresh token hash produced: %s", tokenHash)
		}
		seenHashes[tokenHash] = true
	}
}

func TestJWTProjectIsolationIntegration(t *testing.T) {
	cryptoKeyManager, err := core.NewCryptoKeyManager("0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef")
	if err != nil {
		t.Fatalf("failed to create CryptoKeyManager: %v", err)
	}

	appSigner, appSignerErr := NewSigner(cryptoKeyManager, "portal-key-v1")
	if appSignerErr != nil {
		t.Fatalf("failed to create app signer: %v", appSignerErr)
	}
	consoleUserSigner, consoleUserSignerErr := NewSigner(cryptoKeyManager, "admin-key-v1")
	if consoleUserSignerErr != nil {
		t.Fatalf("failed to create admin signer: %v", consoleUserSignerErr)
	}

	// 1. Both share the same underlying key derivation
	if !bytes.Equal(appSigner.PublicKey(), consoleUserSigner.PublicKey()) {
		t.Fatal("expected identical public keys for identical crypto key managers")
	}

	// 2. Generate token with appSigner for Customer Portal audience and issuer
	userID := uuid.NewV7().String()
	appToken, portalTokenErr := appSigner.GenerateAccessToken(Claims{
		Subject:  userID,
		Email:    "user@portal.com",
		Audience: "customer-portal:user",
		Issuer:   "customer-portal",
	}, 600)
	if portalTokenErr != nil {
		t.Fatalf("failed to generate portal token: %v", portalTokenErr)
	}

	// 3. appSigner successfully verifies token with matching audience and issuer
	verifiedClaims, verifyPortalTokenErr := appSigner.VerifyAccessToken(appToken)
	if verifyPortalTokenErr != nil {
		t.Fatalf("appSigner failed to verify own token: %v", verifyPortalTokenErr)
	}
	if assertErr := verifiedClaims.Assert("aud", "customer-portal:user"); assertErr != nil {
		t.Fatalf("failed to assert portal aud: %v", assertErr)
	}
	if assertErr := verifiedClaims.Assert("iss", "customer-portal"); assertErr != nil {
		t.Fatalf("failed to assert portal iss: %v", assertErr)
	}

	// 4. Verifier expecting Console audience fails assertion
	consoleUserClaims, crossVerifyErr := consoleUserSigner.VerifyAccessToken(appToken)
	if crossVerifyErr != nil {
		t.Fatalf("unexpected signature verification failure: %v", crossVerifyErr)
	}
	if assertErr := consoleUserClaims.Assert("aud", "admin-console:user"); assertErr == nil {
		t.Fatal("expected audience assertion failure when verifying with different audience")
	}
}
