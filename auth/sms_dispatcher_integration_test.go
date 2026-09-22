package auth

import (
	"context"
	"testing"

	"layr.sh/core"
)

func TestAuthSMSDynamicPostgreSQLHookIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	db := kernel.DB()

	ctx := context.Background()

	// 1. Create public.auth_sms_template function in PostgreSQL
	hookSQL := `
		CREATE OR REPLACE FUNCTION public.auth_sms_template(kind text, recipient text, code text, user_id uuid)
		RETURNS jsonb
		LANGUAGE plpgsql
		AS $$
		BEGIN
			IF kind = 'password_reset' THEN
				RETURN json_build_object(
					'text', 'Custom DB SMS Password Reset for ' || recipient || ': ' || code
				)::jsonb;
			END IF;
			IF kind = 'sign_in_otp' THEN
				RETURN json_build_object(
					'text', 'Custom DB SMS Sign In OTP for ' || recipient || ': ' || code
				)::jsonb;
			END IF;
			IF kind = 'phone_verification' AND code != '000000' THEN
				RETURN to_jsonb('Custom Plain Text Phone Verification: ' || code);
			END IF;
			RETURN NULL;
		END;
		$$;
	`
	if _, err := db.Exec(ctx, hookSQL); err != nil {
		t.Fatalf("failed to create hook function: %v", err)
	}

	driverWebhook := "webhook"
	smsDispatcherConfig := &SMSDispatcherConfig{
		Driver: &driverWebhook,
		Webhook: SMSDispatcherWebhookConfig{
			URL: "http://localhost:9999/dummy",
		},
		Templates: SMSDispatcherTemplatesConfig{
			PasswordReset: SMSDispatcherTemplateConfig{
				Text: "Runtime Config SMS Password Reset: {{.Code}}",
			},
			SignInOTP: SMSDispatcherTemplateConfig{
				Text: "Runtime Config SMS OTP: {{.Code}}",
			},
			PhoneVerification: SMSDispatcherTemplateConfig{
				Text: "Runtime Config SMS Phone Verification: {{.Code}}",
			},
		},
	}
	smsDispatcher := NewSMSDispatcher(kernel, func() *SMSDispatcherConfig { return smsDispatcherConfig })

	// 1. Test template resolution using JSON object result with user_id (SQL hook takes priority over runtime config)
	text := smsDispatcher.resolvePasswordResetTemplate(ctx, "+15551234567", "321654", "01918a24-1234-7000-8000-000000000001", smsDispatcherConfig)
	if text != "Custom DB SMS Password Reset for +15551234567: 321654" {
		t.Fatalf("expected custom hook text to override runtime config for password reset, got: %s", text)
	}

	text = smsDispatcher.resolveSignInOTPTemplate(ctx, "+15551234567", "654321", "01918a24-1234-7000-8000-000000000001", smsDispatcherConfig)
	if text != "Custom DB SMS Sign In OTP for +15551234567: 654321" {
		t.Fatalf("expected custom hook text to override runtime config, got: %s", text)
	}

	// Test template resolution using plain string jsonb without user_id (SQL hook takes priority over runtime config)
	text = smsDispatcher.resolvePhoneVerificationTemplate(ctx, "+15559876543", "789123", "", smsDispatcherConfig)
	if text != "Custom Plain Text Phone Verification: 789123" {
		t.Fatalf("expected plain string hook text to override runtime config, got: %s", text)
	}

	// 2. Test hook returning NULL -> falls back to runtime config template
	text = smsDispatcher.resolvePhoneVerificationTemplate(ctx, "+15550001111", "000000", "", smsDispatcherConfig)
	if text != "Runtime Config SMS Phone Verification: 000000" {
		t.Fatalf("expected fallback to runtime config when hook returns null, got: %s", text)
	}

	// 3. Drop hook and test fallback to runtime config
	if _, err := db.Exec(ctx, "DROP FUNCTION public.auth_sms_template(text, text, text, uuid)"); err != nil {
		t.Fatalf("failed to drop hook function: %v", err)
	}

	text = smsDispatcher.resolvePasswordResetTemplate(ctx, "+15551234567", "321654", "", smsDispatcherConfig)
	if text != "Runtime Config SMS Password Reset: 321654" {
		t.Fatalf("expected runtime config text after hook dropped for password reset, got: %s", text)
	}

	text = smsDispatcher.resolveSignInOTPTemplate(ctx, "+15551234567", "654321", "", smsDispatcherConfig)
	if text != "Runtime Config SMS OTP: 654321" {
		t.Fatalf("expected runtime config text after hook dropped, got: %s", text)
	}

	// 4. Test built-in defaults when runtime config templates are empty
	emptyTemplateSMSDispatcherConfig := &SMSDispatcherConfig{Driver: &driverWebhook}
	text = smsDispatcher.resolvePasswordResetTemplate(ctx, "+15551234567", "321654", "", emptyTemplateSMSDispatcherConfig)
	if text != "Your layr-app password reset code is: 321654" {
		t.Fatalf("expected default text for password reset when runtime config empty, got: %s", text)
	}

	text = smsDispatcher.resolveSignInOTPTemplate(ctx, "+15551234567", "654321", "", emptyTemplateSMSDispatcherConfig)
	if text != "Your layr-app sign in verification code is: 654321" {
		t.Fatalf("expected default text when runtime config empty, got: %s", text)
	}
}
