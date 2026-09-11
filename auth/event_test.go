package auth

import (
	"testing"
	"time"
)

func TestAuthEventsUnit(t *testing.T) {
	now := time.Now().UTC()
	passwordHash := "secret-hashed-password"
	userRecord := UserRecord{
		ID:            "usr_123",
		Email:         nil,
		Phone:         nil,
		PasswordHash:  &passwordHash,
		Role:          "authenticated",
		IsAnonymous:   false,
		Properties:    map[string]any{"plan": "pro"},
		CreatedAt:     now,
		LastUpdatedAt: now,
	}

	// 1. ConfigUpdated
	configEvent := NewConfigUpdatedEvent("auth_config", ConfigUpdatedEventData{})
	if configEvent.Type != "auth.config.updated" || configEvent.ResourceType != "auth.config" || configEvent.Action != "updated" {
		t.Fatalf("unexpected config event: %+v", configEvent)
	}
	if configEvent.ResourceID == nil || *configEvent.ResourceID != "auth_config" {
		t.Fatalf("unexpected config resource ID: %v", configEvent.ResourceID)
	}

	// 2. SessionCreated
	clientIP := "127.0.0.1"
	userAgent := "Go-Test"
	sessionCreatedEvent := NewSessionCreatedEvent("sess_123", SessionCreatedEventData{
		ID:        "sess_123",
		UserID:    "usr_123",
		IPAddress: &clientIP,
		UserAgent: &userAgent,
		ExpiresAt: now.Add(time.Hour),
		CreatedAt: now,
	})
	if sessionCreatedEvent.Type != "auth.session.created" || sessionCreatedEvent.ResourceType != "auth.session" || sessionCreatedEvent.Action != "created" {
		t.Fatalf("unexpected session created event: %+v", sessionCreatedEvent)
	}
	if sessionCreatedEvent.ResourceID == nil || *sessionCreatedEvent.ResourceID != "sess_123" {
		t.Fatalf("unexpected session resource ID: %v", sessionCreatedEvent.ResourceID)
	}
	if sessionCreatedEvent.Data["id"] != "sess_123" || sessionCreatedEvent.Data["user_id"] != "usr_123" {
		t.Fatalf("unexpected session data: %+v", sessionCreatedEvent.Data)
	}

	// 3. SessionDeleted
	sessionID := "sess_123"
	revokedCount := 3
	sessionDeletedEvent := NewSessionDeletedEvent("sess_123", SessionDeletedEventData{
		SessionID:    &sessionID,
		UserID:       "usr_123",
		RevokedCount: &revokedCount,
	})
	if sessionDeletedEvent.Type != "auth.session.deleted" || sessionDeletedEvent.Action != "deleted" {
		t.Fatalf("unexpected session deleted event: %+v", sessionDeletedEvent)
	}
	if sessionDeletedEvent.Data["session_id"] != "sess_123" || sessionDeletedEvent.Data["user_id"] != "usr_123" {
		t.Fatalf("unexpected session deleted data: %+v", sessionDeletedEvent.Data)
	}

	// 4. PasskeyCreated
	passkeyEvent := NewPasskeyCreatedEvent("passkey_123", PasskeyCreatedEventData{
		ID:           "passkey_123",
		UserID:       "usr_123",
		FriendlyName: "MacBook Passkey",
		Transports:   []string{"internal"},
	})
	if passkeyEvent.Type != "auth.passkey.created" || passkeyEvent.Action != "created" {
		t.Fatalf("unexpected passkey event: %+v", passkeyEvent)
	}
	if passkeyEvent.ResourceID == nil || *passkeyEvent.ResourceID != "passkey_123" {
		t.Fatalf("unexpected passkey resource ID: %v", passkeyEvent.ResourceID)
	}
	if passkeyEvent.Data["friendly_name"] != "MacBook Passkey" {
		t.Fatalf("unexpected passkey friendly_name: %v", passkeyEvent.Data["friendly_name"])
	}

	// 5. UserSignedUp
	signedUpEvent := NewUserSignedUpEvent("usr_123", UserSignedUpEventData(userRecord))
	if signedUpEvent.Type != "auth.user.signed_up" || signedUpEvent.Action != "signed_up" {
		t.Fatalf("unexpected user signed up event: %+v", signedUpEvent)
	}
	if signedUpEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected user signed up ID: %v", signedUpEvent.Data["id"])
	}
	if _, hasHash := signedUpEvent.Data["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from signed up event data")
	}

	// 6. UserConverted
	convertedEvent := NewUserConvertedEvent("usr_123", UserConvertedEventData(userRecord))
	if convertedEvent.Type != "auth.user.converted" || convertedEvent.Action != "converted" {
		t.Fatalf("unexpected user converted event: %+v", convertedEvent)
	}
	if convertedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected user converted ID: %v", convertedEvent.Data["id"])
	}
	if _, hasHash := convertedEvent.Data["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from converted event data")
	}

	// 7. UserEmailVerified
	emailVerifiedEvent := NewUserEmailVerifiedEvent("usr_123", UserEmailVerifiedEventData(userRecord))
	if emailVerifiedEvent.Type != "auth.user.email_verified" || emailVerifiedEvent.Action != "email_verified" {
		t.Fatalf("unexpected email verified event: %+v", emailVerifiedEvent)
	}
	if emailVerifiedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected email verified ID: %v", emailVerifiedEvent.Data["id"])
	}
	if _, hasHash := emailVerifiedEvent.Data["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from email verified event data")
	}

	// 8. UserPhoneVerified
	phoneVerifiedEvent := NewUserPhoneVerifiedEvent("usr_123", UserPhoneVerifiedEventData(userRecord))
	if phoneVerifiedEvent.Type != "auth.user.phone_verified" || phoneVerifiedEvent.Action != "phone_verified" {
		t.Fatalf("unexpected phone verified event: %+v", phoneVerifiedEvent)
	}
	if phoneVerifiedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected phone verified ID: %v", phoneVerifiedEvent.Data["id"])
	}
	if _, hasHash := phoneVerifiedEvent.Data["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from phone verified event data")
	}

	// 9. UserUpdated
	updatedEvent := NewUserUpdatedEvent("usr_123", UserUpdatedEventData(userRecord))
	if updatedEvent.Type != "auth.user.updated" || updatedEvent.Action != "updated" {
		t.Fatalf("unexpected user updated event: %+v", updatedEvent)
	}
	if updatedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected user updated ID: %v", updatedEvent.Data["id"])
	}
	if _, hasHash := updatedEvent.Data["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from user updated event data")
	}

	// 10. UserDeleted
	deletedEvent := NewUserDeletedEvent("usr_123", UserDeletedEventData(userRecord))
	if deletedEvent.Type != "auth.user.deleted" || deletedEvent.Action != "deleted" {
		t.Fatalf("unexpected user deleted event: %+v", deletedEvent)
	}
	if deletedEvent.Data["id"] != "usr_123" {
		t.Fatalf("unexpected user deleted ID: %v", deletedEvent.Data["id"])
	}
	if _, hasHash := deletedEvent.Data["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from user deleted event data")
	}

	// 11. PasswordResetRequested
	resetRequestedEvent := NewPasswordResetRequestedEvent("usr_123", PasswordResetRequestedEventData{
		UserID:    "usr_123",
		Recipient: "test@example.com",
	})
	if resetRequestedEvent.Type != "auth.password.reset_requested" || resetRequestedEvent.Action != "reset_requested" {
		t.Fatalf("unexpected password reset requested event: %+v", resetRequestedEvent)
	}
	if _, hasCode := resetRequestedEvent.Data["code"]; hasCode {
		t.Fatal("expected code to be omitted from reset requested event data")
	}

	// 12. PasswordReset
	resetEvent := NewPasswordResetEvent("usr_123", PasswordResetEventData{
		Recipient: "test@example.com",
		User:      userRecord,
	})
	if resetEvent.Type != "auth.password.reset" || resetEvent.Action != "reset" {
		t.Fatalf("unexpected password reset event: %+v", resetEvent)
	}
	userDataMap, isMap := resetEvent.Data["user"].(map[string]any)
	if !isMap {
		t.Fatalf("expected nested user in reset event data, got: %v", resetEvent.Data["user"])
	}
	if _, hasHash := userDataMap["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from password reset user data")
	}

	// 13. PasswordChanged
	changedEvent := NewPasswordChangedEvent("usr_123", PasswordChangedEventData{
		User: userRecord,
	})
	if changedEvent.Type != "auth.password.changed" || changedEvent.Action != "changed" {
		t.Fatalf("unexpected password changed event: %+v", changedEvent)
	}
	changedUserDataMap, isMap := changedEvent.Data["user"].(map[string]any)
	if !isMap {
		t.Fatalf("expected nested user in changed event data, got: %v", changedEvent.Data["user"])
	}
	if _, hasHash := changedUserDataMap["password_hash"]; hasHash {
		t.Fatal("expected password_hash to be omitted from password changed user data")
	}

	// 14. OTPSent
	otpEvent := NewOTPSentEvent("usr_123", OTPSentEventData{
		UserID:    "usr_123",
		Recipient: "test@example.com",
		Purpose:   "email_verification",
	})
	if otpEvent.Type != "auth.otp.sent" || otpEvent.Action != "sent" {
		t.Fatalf("unexpected otp sent event: %+v", otpEvent)
	}
	if otpEvent.Data["purpose"] != "email_verification" {
		t.Fatalf("unexpected purpose in otp sent event: %v", otpEvent.Data["purpose"])
	}
	if _, hasCode := otpEvent.Data["code"]; hasCode {
		t.Fatal("expected code to be omitted from otp sent event data")
	}
}
