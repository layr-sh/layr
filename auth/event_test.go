package auth

import (
	"testing"
	"time"
)

func TestAuthEventsUnit(t *testing.T) {
	now := time.Now().UTC()
	passwordHash := "secret-hashed-password"
	email := "test@example.com"
	phone := "+1234567890"
	encryptedMFASecret := "enc:secret:totp"
	user := User{
		ID:                 "usr_123",
		Email:              &email,
		Phone:              &phone,
		PasswordHash:       &passwordHash,
		EncryptedMFASecret: &encryptedMFASecret,
		MFAEnabled:         true,
		Role:               "authenticated",
		IsAnonymous:        false,
		Properties: map[string]any{
			"plan": "pro",
			"role": "custom_role",
		},
		CreatedAt:     now,
		LastUpdatedAt: now,
	}

	// Helper function to assert user properties and omission of sensitive fields
	assertSanitizedProps := func(data map[string]any, eventType string) {
		t.Helper()
		props, ok := data["properties"].(map[string]any)
		if !ok {
			t.Fatalf("[%s] expected properties map in data, got %v", eventType, data["properties"])
		}
		if _, hasSecret := data["encrypted_mfa_secret"]; hasSecret {
			t.Fatalf("[%s] expected encrypted_mfa_secret to be omitted from event data", eventType)
		}
		if _, hasHash := data["password_hash"]; hasHash {
			t.Fatalf("[%s] expected password_hash to be omitted from event data", eventType)
		}
		if props["plan"] != "pro" {
			t.Fatalf("[%s] expected plan=pro preserved, got %v", eventType, props["plan"])
		}
		if props["role"] != "custom_role" {
			t.Fatalf("[%s] expected role=custom_role preserved in properties, got %v", eventType, props["role"])
		}
	}

	// 1. ConfigUpdated
	configEvent := NewConfigUpdatedEvent("auth_config", ConfigUpdatedEventData{})
	if configEvent.Type != "auth.config.updated" || configEvent.ResourceType != "auth.config" || configEvent.Action != "updated" {
		t.Fatalf("unexpected config event: %+v", configEvent)
	}
	if configEvent.ResourceID == nil || *configEvent.ResourceID != "auth_config" {
		t.Fatalf("unexpected config resource ID: %v", configEvent.ResourceID)
	}

	// 2. SessionCreated (nested user)
	clientIP := "127.0.0.1"
	userAgent := "Go-Test"
	sessionCreatedEvent := NewSessionCreatedEvent("sess_123", SessionCreatedEventData{
		ID:         "sess_123",
		User:       user,
		AuthMethod: "password",
		Provider:   "google",
		IPAddress:  &clientIP,
		UserAgent:  &userAgent,
		ExpiresAt:  now.Add(time.Hour),
		CreatedAt:  now,
	})
	if sessionCreatedEvent.Type != "auth.session.created" || sessionCreatedEvent.ResourceType != "auth.session" || sessionCreatedEvent.Action != "created" {
		t.Fatalf("unexpected session created event: %+v", sessionCreatedEvent)
	}
	if sessionCreatedEvent.ResourceID == nil || *sessionCreatedEvent.ResourceID != "sess_123" {
		t.Fatalf("unexpected session resource ID: %v", sessionCreatedEvent.ResourceID)
	}
	if sessionCreatedEvent.Data["id"] != "sess_123" || sessionCreatedEvent.Data["auth_method"] != "password" || sessionCreatedEvent.Data["provider"] != "google" {
		t.Fatalf("unexpected session data: %+v", sessionCreatedEvent.Data)
	}
	userData, ok := sessionCreatedEvent.Data["user"].(map[string]any)
	if !ok || userData["id"] != "usr_123" {
		t.Fatalf("expected nested user with id in session created event, got: %v", sessionCreatedEvent.Data["user"])
	}
	assertSanitizedProps(userData, "SessionCreated")

	// 3. SessionDeleted (nested user)
	sessionID := "sess_123"
	revokedCount := 5
	sessionDeletedEvent := NewSessionDeletedEvent("sess_123", SessionDeletedEventData{
		User:         user,
		SessionID:    &sessionID,
		RevokedCount: &revokedCount,
	})
	if sessionDeletedEvent.Type != "auth.session.deleted" || sessionDeletedEvent.ResourceType != "auth.session" || sessionDeletedEvent.Action != "deleted" {
		t.Fatalf("unexpected session deleted event: %+v", sessionDeletedEvent)
	}
	if sessionDeletedEvent.Data["session_id"] != "sess_123" {
		t.Fatalf("unexpected session deleted data: %+v", sessionDeletedEvent.Data)
	}
	userDeletedSess, ok := sessionDeletedEvent.Data["user"].(map[string]any)
	if !ok || userDeletedSess["id"] != "usr_123" {
		t.Fatalf("expected nested user in session deleted event, got: %v", sessionDeletedEvent.Data["user"])
	}
	assertSanitizedProps(userDeletedSess, "SessionDeleted")

	// 4. PasskeyCreated (nested user)
	passkeyEvent := NewPasskeyCreatedEvent("passkey_123", PasskeyCreatedEventData{
		ID:           "passkey_123",
		User:         user,
		FriendlyName: "MacBook Touch ID",
		Transports:   []string{"internal"},
	})
	if passkeyEvent.Type != "auth.passkey.created" || passkeyEvent.ResourceType != "auth.passkey" || passkeyEvent.Action != "created" {
		t.Fatalf("unexpected passkey event: %+v", passkeyEvent)
	}
	if passkeyEvent.Data["friendly_name"] != "MacBook Touch ID" {
		t.Fatalf("unexpected friendly name: %v", passkeyEvent.Data["friendly_name"])
	}
	passkeyUser, ok := passkeyEvent.Data["user"].(map[string]any)
	if !ok || passkeyUser["id"] != "usr_123" {
		t.Fatalf("expected nested user in passkey created event, got: %v", passkeyEvent.Data["user"])
	}
	assertSanitizedProps(passkeyUser, "PasskeyCreated")

	// 4b. PasskeyDeleted (nested user)
	passkeyDeletedEvent := NewPasskeyDeletedEvent("passkey_123", PasskeyDeletedEventData{
		ID:   "passkey_123",
		User: user,
	})
	if passkeyDeletedEvent.Type != "auth.passkey.deleted" || passkeyDeletedEvent.ResourceType != "auth.passkey" || passkeyDeletedEvent.Action != "deleted" {
		t.Fatalf("unexpected passkey deleted event: %+v", passkeyDeletedEvent)
	}
	if passkeyDeletedEvent.Data["id"] != "passkey_123" {
		t.Fatalf("unexpected passkey deleted ID: %v", passkeyDeletedEvent.Data["id"])
	}
	passkeyDeletedUser, ok := passkeyDeletedEvent.Data["user"].(map[string]any)
	if !ok || passkeyDeletedUser["id"] != "usr_123" {
		t.Fatalf("expected nested user in passkey deleted event, got: %v", passkeyDeletedEvent.Data["user"])
	}
	assertSanitizedProps(passkeyDeletedUser, "PasskeyDeleted")

	// 5. UserSignedUp (flat user)
	signedUpEvent := NewUserSignedUpEvent("usr_123", UserSignedUpEventData(user))
	if signedUpEvent.Type != "auth.user.signed_up" || signedUpEvent.ResourceType != "auth.user" || signedUpEvent.Action != "signed_up" {
		t.Fatalf("unexpected user signed up event: %+v", signedUpEvent)
	}
	if signedUpEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected user signed up ID: %v", signedUpEvent.Data["id"])
	}
	if _, hasHash := signedUpEvent.Data["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from user signed up event data")
	}
	assertSanitizedProps(signedUpEvent.Data, "UserSignedUp")

	// 6. UserConverted (flat user)
	convertedEvent := NewUserConvertedEvent("usr_123", UserConvertedEventData(user))
	if convertedEvent.Type != "auth.user.converted" || convertedEvent.ResourceType != "auth.user" || convertedEvent.Action != "converted" {
		t.Fatalf("unexpected user converted event: %+v", convertedEvent)
	}
	if convertedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected user converted ID: %v", convertedEvent.Data["id"])
	}
	assertSanitizedProps(convertedEvent.Data, "UserConverted")

	// 7. UserEmailVerified (flat user)
	emailVerifiedEvent := NewUserEmailVerifiedEvent("usr_123", UserEmailVerifiedEventData(user))
	if emailVerifiedEvent.Type != "auth.user.email_verified" || emailVerifiedEvent.ResourceType != "auth.user" || emailVerifiedEvent.Action != "email_verified" {
		t.Fatalf("unexpected user email verified event: %+v", emailVerifiedEvent)
	}
	if emailVerifiedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected user email verified ID: %v", emailVerifiedEvent.Data["id"])
	}
	assertSanitizedProps(emailVerifiedEvent.Data, "UserEmailVerified")

	// 8. UserPhoneVerified (flat user)
	phoneVerifiedEvent := NewUserPhoneVerifiedEvent("usr_123", UserPhoneVerifiedEventData(user))
	if phoneVerifiedEvent.Type != "auth.user.phone_verified" || phoneVerifiedEvent.ResourceType != "auth.user" || phoneVerifiedEvent.Action != "phone_verified" {
		t.Fatalf("unexpected user phone verified event: %+v", phoneVerifiedEvent)
	}
	if phoneVerifiedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected user phone verified ID: %v", phoneVerifiedEvent.Data["id"])
	}
	assertSanitizedProps(phoneVerifiedEvent.Data, "UserPhoneVerified")

	// 9. UserUpdated (flat user)
	updatedEvent := NewUserUpdatedEvent("usr_123", UserUpdatedEventData(user))
	if updatedEvent.Type != "auth.user.updated" || updatedEvent.ResourceType != "auth.user" || updatedEvent.Action != "updated" {
		t.Fatalf("unexpected user updated event: %+v", updatedEvent)
	}
	if updatedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected user updated ID: %v", updatedEvent.Data["id"])
	}
	assertSanitizedProps(updatedEvent.Data, "UserUpdated")

	// 10. UserDeleted (flat user)
	deletedEvent := NewUserDeletedEvent("usr_123", UserDeletedEventData(user))
	if deletedEvent.Type != "auth.user.deleted" || deletedEvent.ResourceType != "auth.user" || deletedEvent.Action != "deleted" {
		t.Fatalf("unexpected user deleted event: %+v", deletedEvent)
	}
	if deletedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected user deleted ID: %v", deletedEvent.Data["id"])
	}
	if _, hasHash := deletedEvent.Data["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from user deleted event data")
	}
	assertSanitizedProps(deletedEvent.Data, "UserDeleted")

	// 11. PasswordResetRequested (nested user)
	resetRequestedEvent := NewPasswordResetRequestedEvent("usr_123", PasswordResetRequestedEventData{
		Recipient: "test@example.com",
		User:      user,
	})
	if resetRequestedEvent.Type != "auth.password.reset_requested" || resetRequestedEvent.Action != "reset_requested" {
		t.Fatalf("unexpected password reset requested event: %+v", resetRequestedEvent)
	}
	if resetRequestedEvent.Data["recipient"] != "test@example.com" {
		t.Fatalf("unexpected recipient: %v", resetRequestedEvent.Data["recipient"])
	}
	resetRequestedUser, ok := resetRequestedEvent.Data["user"].(map[string]any)
	if !ok || resetRequestedUser["id"] != "usr_123" {
		t.Fatalf("expected nested user in password reset requested event, got: %v", resetRequestedEvent.Data["user"])
	}
	assertSanitizedProps(resetRequestedUser, "PasswordResetRequested")

	// 12. PasswordReset (nested user)
	resetEvent := NewPasswordResetEvent("usr_123", PasswordResetEventData{
		Recipient: "test@example.com",
		User:      user,
	})
	if resetEvent.Type != "auth.password.reset" || resetEvent.Action != "reset" {
		t.Fatalf("unexpected password reset event: %+v", resetEvent)
	}
	if resetEvent.Data["recipient"] != "test@example.com" {
		t.Fatalf("expected recipient in reset event data, got: %v", resetEvent.Data["recipient"])
	}
	resetUser, ok := resetEvent.Data["user"].(map[string]any)
	if !ok || resetUser["id"] != "usr_123" {
		t.Fatalf("expected nested user in reset event data, got: %v", resetEvent.Data["user"])
	}
	if _, hasHash := resetUser["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from password reset user data")
	}
	assertSanitizedProps(resetUser, "PasswordReset")

	// 13. PasswordChanged (flat user)
	changedEvent := NewPasswordChangedEvent("usr_123", PasswordChangedEventData(user))
	if changedEvent.Type != "auth.password.changed" || changedEvent.Action != "changed" {
		t.Fatalf("unexpected password changed event: %+v", changedEvent)
	}
	if changedEvent.Data["id"] != "usr_123" {
		t.Fatalf("expected flat user id in changed event data, got: %v", changedEvent.Data["id"])
	}
	if _, hasHash := changedEvent.Data["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from password changed user data")
	}
	assertSanitizedProps(changedEvent.Data, "PasswordChanged")

	// 14. MFAEnabled & MFADisabled (flat user)
	mfaEnabledEvent := NewMFAEnabledEvent("usr_123", MFAEnabledEventData(user))
	if mfaEnabledEvent.Type != "auth.mfa.enabled" || mfaEnabledEvent.Action != "enabled" || mfaEnabledEvent.ResourceType != "auth.mfa" {
		t.Fatalf("unexpected mfa enabled event: %+v", mfaEnabledEvent)
	}
	if mfaEnabledEvent.Data["id"] != "usr_123" {
		t.Fatalf("expected flat user id in mfa enabled event: %v", mfaEnabledEvent.Data["id"])
	}
	assertSanitizedProps(mfaEnabledEvent.Data, "MFAEnabled")

	mfaDisabledEvent := NewMFADisabledEvent("usr_123", MFADisabledEventData(user))
	if mfaDisabledEvent.Type != "auth.mfa.disabled" || mfaDisabledEvent.Action != "disabled" || mfaDisabledEvent.ResourceType != "auth.mfa" {
		t.Fatalf("unexpected mfa disabled event: %+v", mfaDisabledEvent)
	}
	if mfaDisabledEvent.Data["id"] != "usr_123" {
		t.Fatalf("expected flat user id in mfa disabled event: %v", mfaDisabledEvent.Data["id"])
	}
	assertSanitizedProps(mfaDisabledEvent.Data, "MFADisabled")

	// 15. OTPSent (nested user when present, and with nil user)
	otpWithUserEvent := NewOTPSentEvent("test@example.com", OTPSentEventData{
		Recipient: "test@example.com",
		Purpose:   "email_verification",
		Channel:   "email",
		User:      &user,
	})
	if otpWithUserEvent.Type != "auth.otp.sent" || otpWithUserEvent.Action != "sent" {
		t.Fatalf("unexpected otp sent event: %+v", otpWithUserEvent)
	}
	if otpWithUserEvent.Data["purpose"] != "email_verification" || otpWithUserEvent.Data["channel"] != "email" {
		t.Fatalf("unexpected data in otp sent event: %+v", otpWithUserEvent.Data)
	}
	otpUser, ok := otpWithUserEvent.Data["user"].(map[string]any)
	if !ok || otpUser["id"] != "usr_123" {
		t.Fatalf("expected nested user in otp sent event, got: %v", otpWithUserEvent.Data["user"])
	}
	assertSanitizedProps(otpUser, "OTPSentWithUser")

	otpNilUserEvent := NewOTPSentEvent("test@example.com", OTPSentEventData{
		Recipient: "test@example.com",
		Purpose:   "sign_in",
		Channel:   "email",
		User:      nil,
	})
	if _, hasUser := otpNilUserEvent.Data["user"]; hasUser {
		t.Fatalf("expected user to be omitted when nil in otp sent event: %v", otpNilUserEvent.Data)
	}

	// 16. OTPVerified (nested user when present, and with nil user)
	otpVerifiedEvent := NewOTPVerifiedEvent("test@example.com", OTPVerifiedEventData{
		Recipient: "test@example.com",
		Purpose:   "sign_in",
		Channel:   "email",
		User:      &user,
	})
	if otpVerifiedEvent.Type != "auth.otp.verified" || otpVerifiedEvent.Action != "verified" {
		t.Fatalf("unexpected otp verified event: %+v", otpVerifiedEvent)
	}
	if otpVerifiedEvent.Data["purpose"] != "sign_in" || otpVerifiedEvent.Data["channel"] != "email" {
		t.Fatalf("unexpected data in otp verified event: %+v", otpVerifiedEvent.Data)
	}
	otpVerifiedUser, ok := otpVerifiedEvent.Data["user"].(map[string]any)
	if !ok || otpVerifiedUser["id"] != "usr_123" {
		t.Fatalf("expected nested user in otp verified event, got: %v", otpVerifiedEvent.Data["user"])
	}
	assertSanitizedProps(otpVerifiedUser, "OTPVerifiedWithUser")

	otpVerifiedNilUserEvent := NewOTPVerifiedEvent("test@example.com", OTPVerifiedEventData{
		Recipient: "test@example.com",
		Purpose:   "sign_in",
		Channel:   "email",
		User:      nil,
	})
	if _, hasUser := otpVerifiedNilUserEvent.Data["user"]; hasUser {
		t.Fatalf("expected user to be omitted when nil in otp verified event: %v", otpVerifiedNilUserEvent.Data)
	}

	// 17. UserCreated (flat user)
	userCreatedEvent := NewUserCreatedEvent("usr_123", UserCreatedEventData(user))
	if userCreatedEvent.Type != "auth.user.created" || userCreatedEvent.Action != "created" {
		t.Fatalf("unexpected user created event: %+v", userCreatedEvent)
	}
	if userCreatedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected id in user created event: %v", userCreatedEvent.Data["id"])
	}
	assertSanitizedProps(userCreatedEvent.Data, "UserCreated")

	// 18. UserLocked (nested user)
	lockedUntilTime := now.Add(24 * time.Hour)
	userLockedEvent := NewUserLockedEvent("usr_123", UserLockedEventData{
		User:        user,
		LockedUntil: &lockedUntilTime,
	})
	if userLockedEvent.Type != "auth.user.locked" || userLockedEvent.Action != "locked" {
		t.Fatalf("unexpected user locked event: %+v", userLockedEvent)
	}
	userLockedData, ok := userLockedEvent.Data["user"].(map[string]any)
	if !ok || userLockedData["id"] != "usr_123" {
		t.Fatalf("expected nested user in user locked event, got: %v", userLockedEvent.Data["user"])
	}
	assertSanitizedProps(userLockedData, "UserLocked")

	// 19. UserUnlocked (flat user)
	userUnlockedEvent := NewUserUnlockedEvent("usr_123", UserUnlockedEventData(user))
	if userUnlockedEvent.Type != "auth.user.unlocked" || userUnlockedEvent.Action != "unlocked" {
		t.Fatalf("unexpected user unlocked event: %+v", userUnlockedEvent)
	}
	if userUnlockedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected flat user id in user unlocked event: %v", userUnlockedEvent.Data["id"])
	}
	assertSanitizedProps(userUnlockedEvent.Data, "UserUnlocked")

	// 20. Event with nil properties preserved
	nilPropsUser := User{ID: "nil_props", Properties: nil}
	nilPropsEvent := NewUserUnlockedEvent("nil_props", UserUnlockedEventData(nilPropsUser))
	if nilPropsEvent.Data["properties"] != nil {
		t.Fatalf("expected nil properties preserved in event, got %v", nilPropsEvent.Data["properties"])
	}

	// 21. PasswordBreachBlocked
	passwordBreachBlockedEvent := NewPasswordBreachBlockedEvent("user@example.com", PasswordBreachBlockedEventData{
		Email: "user@example.com",
		Count: 42,
	})
	if passwordBreachBlockedEvent.Type != "auth.threat.password_breach_blocked" {
		t.Fatalf("unexpected password breach event type: %s", passwordBreachBlockedEvent.Type)
	}
	if passwordBreachBlockedEvent.ResourceID == nil || *passwordBreachBlockedEvent.ResourceID != "user@example.com" {
		t.Fatalf("unexpected resource ID in breach event: %v", passwordBreachBlockedEvent.ResourceID)
	}
	if passwordBreachBlockedEvent.Data["count"] != float64(42) {
		t.Fatalf("unexpected breach count in event: %v", passwordBreachBlockedEvent.Data["count"])
	}

	// 22. BotChallengeFailed
	botChallengeFailedEvent := NewBotChallengeFailedEvent("192.168.1.1", BotChallengeFailedEventData{
		IPAddress: "192.168.1.1",
		Provider:  "turnstile",
		Endpoint:  "/api/v1/auth/sign-in",
	})
	if botChallengeFailedEvent.Type != "auth.threat.bot_challenge_failed" {
		t.Fatalf("unexpected bot challenge event type: %s", botChallengeFailedEvent.Type)
	}
	if botChallengeFailedEvent.ResourceID == nil || *botChallengeFailedEvent.ResourceID != "192.168.1.1" {
		t.Fatalf("unexpected resource ID in bot challenge event: %v", botChallengeFailedEvent.ResourceID)
	}
	if botChallengeFailedEvent.Data["provider"] != "turnstile" {
		t.Fatalf("unexpected provider in bot event: %v", botChallengeFailedEvent.Data["provider"])
	}

	// 23. SuspiciousSignIn
	suspiciousSignInEvent := NewSuspiciousSignInEvent("usr_123", SuspiciousSignInEventData{
		User:      user,
		IPAddress: "203.0.113.195",
		UserAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 16_0 like Mac OS X)",
		RiskScore: 75,
		RiskLevel: "high",
		Reasons:   []string{"new_device", "new_ip"},
	})
	if suspiciousSignInEvent.Type != "auth.user.suspicious_sign_in" {
		t.Fatalf("unexpected suspicious sign in event type: %s", suspiciousSignInEvent.Type)
	}
	if suspiciousSignInEvent.ResourceID == nil || *suspiciousSignInEvent.ResourceID != "usr_123" {
		t.Fatalf("unexpected resource ID in suspicious sign in event: %v", suspiciousSignInEvent.ResourceID)
	}
	suspiciousData, ok := suspiciousSignInEvent.Data["user"].(map[string]any)
	if !ok || suspiciousData["id"] != "usr_123" {
		t.Fatalf("expected sanitized user in suspicious sign-in event, got: %v", suspiciousSignInEvent.Data["user"])
	}
	assertSanitizedProps(suspiciousData, "SuspiciousSignIn")

	// 24. UserSignInFailed
	userSignInFailedEvent := NewUserSignInFailedEvent("usr_123", UserSignInFailedEventData{
		Identifier: "test@example.com",
		AuthMethod: "password",
		Reason:     "invalid_credentials",
		IPAddress:  "192.168.1.50",
		UserAgent:  "Mozilla/5.0",
		User:       &user,
	})
	if userSignInFailedEvent.Type != "auth.user.sign_in_failed" {
		t.Fatalf("unexpected sign in failed event type: %s", userSignInFailedEvent.Type)
	}
	if userSignInFailedEvent.ResourceID == nil || *userSignInFailedEvent.ResourceID != "usr_123" {
		t.Fatalf("unexpected resource ID in sign in failed event: %v", userSignInFailedEvent.ResourceID)
	}
	if userSignInFailedEvent.Data["reason"] != "invalid_credentials" {
		t.Fatalf("unexpected reason in sign in failed event: %v", userSignInFailedEvent.Data["reason"])
	}

	// 25. MFAChallengeFailed
	mfaChallengeFailedEvent := NewMFAChallengeFailedEvent("usr_123", MFAChallengeFailedEventData{
		UserID:    "usr_123",
		Reason:    "invalid_code",
		IPAddress: "192.168.1.50",
		UserAgent: "Mozilla/5.0",
		User:      &user,
	})
	if mfaChallengeFailedEvent.Type != "auth.mfa.challenge_failed" {
		t.Fatalf("unexpected MFA challenge failed event type: %s", mfaChallengeFailedEvent.Type)
	}
	if mfaChallengeFailedEvent.ResourceID == nil || *mfaChallengeFailedEvent.ResourceID != "usr_123" {
		t.Fatalf("unexpected resource ID in MFA challenge failed event: %v", mfaChallengeFailedEvent.ResourceID)
	}

	// 26. OTPVerificationFailed
	otpVerificationFailedEvent := NewOTPVerificationFailedEvent("+15551234567", OTPVerificationFailedEventData{
		Recipient: "+15551234567",
		Purpose:   "sign_in",
		Channel:   "sms",
		Reason:    "invalid_code",
		IPAddress: "192.168.1.50",
		UserAgent: "Mozilla/5.0",
	})
	if otpVerificationFailedEvent.Type != "auth.otp.verification_failed" {
		t.Fatalf("unexpected OTP verification failed event type: %s", otpVerificationFailedEvent.Type)
	}
	if otpVerificationFailedEvent.ResourceID == nil || *otpVerificationFailedEvent.ResourceID != "+15551234567" {
		t.Fatalf("unexpected resource ID in OTP verification failed event: %v", otpVerificationFailedEvent.ResourceID)
	}

	// 27. RateLimitExceeded
	rateLimitExceededEvent := NewRateLimitExceededEvent("test@example.com", RateLimitExceededEventData{
		Identifier:   "test@example.com",
		Endpoint:     "/api/v1/auth/sign-in",
		AttemptCount: 6,
		IPAddress:    "192.168.1.50",
		UserAgent:    "Mozilla/5.0",
	})
	if rateLimitExceededEvent.Type != "auth.threat.rate_limit_exceeded" {
		t.Fatalf("unexpected rate limit exceeded event type: %s", rateLimitExceededEvent.Type)
	}
	if rateLimitExceededEvent.ResourceID == nil || *rateLimitExceededEvent.ResourceID != "test@example.com" {
		t.Fatalf("unexpected resource ID in rate limit exceeded event: %v", rateLimitExceededEvent.ResourceID)
	}

	// 28. UserExported
	userExportedEvent := NewUserExportedEvent("usr_123", UserExportedEventData(user))
	if userExportedEvent.Type != "auth.user.exported" {
		t.Fatalf("unexpected user exported event type: %s", userExportedEvent.Type)
	}
	if userExportedEvent.ResourceID == nil || *userExportedEvent.ResourceID != "usr_123" {
		t.Fatalf("unexpected resource ID in user exported event: %v", userExportedEvent.ResourceID)
	}
}
