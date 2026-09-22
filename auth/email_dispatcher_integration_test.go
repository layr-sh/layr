package auth

import (
	"context"
	"testing"

	"layr.sh/core"
)

func TestAuthEmailDynamicPostgreSQLHookIntegration(t *testing.T) {
	kernel, cleanup := core.SetupTestKernel(t, Migrations)
	defer cleanup()
	db := kernel.DB()

	ctx := context.Background()

	// 1. Create public.auth_email_template function in PostgreSQL
	hookSQL := `
		CREATE OR REPLACE FUNCTION public.auth_email_template(kind text, recipient text, code text, user_id uuid)
		RETURNS jsonb
		LANGUAGE plpgsql
		AS $$
		BEGIN
			IF kind = 'password_reset' THEN
				RETURN json_build_object(
					'subject', 'Custom DB Reset for ' || recipient,
					'html', '<h1>Your custom code is ' || code || '</h1>',
					'text', 'Your custom code is ' || code
				)::jsonb;
			END IF;
			IF kind = 'sign_in_otp' AND code != 'null_fallback' THEN
				RETURN json_build_object(
					'subject', 'Custom DB Sign In OTP for ' || recipient,
					'html', '<h1>Your custom OTP is ' || code || '</h1>',
					'text', 'Your custom OTP is ' || code
				)::jsonb;
			END IF;
			IF kind = 'email_verification' THEN
				RETURN json_build_object(
					'subject', 'Custom DB Email Verification for ' || recipient,
					'html', '<h1>Your verification code is ' || code || '</h1>',
					'text', 'Your verification code is ' || code
				)::jsonb;
			END IF;
			IF kind = 'suspicious_activity' THEN
				RETURN json_build_object(
					'subject', 'Custom DB Security Alert for ' || recipient,
					'html', '<p>Suspicious login from IP: ' || code || '</p>',
					'text', 'Suspicious login from IP: ' || code
				)::jsonb;
			END IF;
			RETURN NULL;
		END;
		$$;
	`
	if _, err := db.Exec(ctx, hookSQL); err != nil {
		t.Fatalf("failed to create hook function: %v", err)
	}

	driverWebhook := "webhook"
	emailDispatcherConfig := &EmailDispatcherConfig{
		Driver: &driverWebhook,
		Webhook: EmailDispatcherWebhookConfig{
			URL: "http://localhost:9999/dummy",
		},
		Templates: EmailDispatcherTemplatesConfig{
			PasswordReset: EmailDispatcherTemplateConfig{
				Subject: "Runtime Config Reset: {{.AppName}}",
				HTML:    "<p>Runtime Config HTML: {{.Code}}</p>",
				Text:    "Runtime Config Text: {{.Code}}",
			},
			SignInOTP: EmailDispatcherTemplateConfig{
				Subject: "Runtime Config Sign In OTP: {{.AppName}}",
				HTML:    "<p>Runtime OTP HTML: {{.Code}}</p>",
				Text:    "Runtime OTP Text: {{.Code}}",
			},
		},
	}
	emailDispatcher := NewEmailDispatcher(kernel, func() *EmailDispatcherConfig { return emailDispatcherConfig })

	// 1. Test template resolution using the hook with user_id (SQL hook takes priority over runtime config)
	subject, html, text := emailDispatcher.resolvePasswordResetTemplate(ctx, "alice@example.com", "456789", "01918a24-1234-7000-8000-000000000001", emailDispatcherConfig)
	if subject != "Custom DB Reset for alice@example.com" {
		t.Fatalf("expected custom hook subject to override runtime config, got: %s", subject)
	}
	if html != "<h1>Your custom code is 456789</h1>" {
		t.Fatalf("expected custom hook html to override runtime config, got: %s", html)
	}
	if text != "Your custom code is 456789" {
		t.Fatalf("expected custom hook text to override runtime config, got: %s", text)
	}

	// Test template resolution using the hook with empty user_id (NULL passed to Postgres)
	subject, _, _ = emailDispatcher.resolvePasswordResetTemplate(ctx, "bob@example.com", "123123", "", emailDispatcherConfig)
	if subject != "Custom DB Reset for bob@example.com" {
		t.Fatalf("expected custom hook subject without userID, got: %s", subject)
	}

	// Test sign in otp template resolution using the hook
	subject, html, text = emailDispatcher.resolveSignInOTPTemplate(ctx, "bob@example.com", "456789", "01918a24-1234-7000-8000-000000000001", emailDispatcherConfig)
	if subject != "Custom DB Sign In OTP for bob@example.com" || html != "<h1>Your custom OTP is 456789</h1>" || text != "Your custom OTP is 456789" {
		t.Fatalf("expected custom hook for sign in otp, got: %s, %s, %s", subject, html, text)
	}

	// Test email verification template resolution using the hook
	subject, html, text = emailDispatcher.resolveEmailVerificationTemplate(ctx, "charlie@example.com", "987654", "01918a24-1234-7000-8000-000000000001", emailDispatcherConfig)
	if subject != "Custom DB Email Verification for charlie@example.com" || html != "<h1>Your verification code is 987654</h1>" || text != "Your verification code is 987654" {
		t.Fatalf("expected custom hook for email verification, got: %s, %s, %s", subject, html, text)
	}

	// Test suspicious activity template resolution using the hook
	subject, html, text = emailDispatcher.resolveSuspiciousActivityTemplate(ctx, "charlie@example.com", "01918a24-1234-7000-8000-000000000001", "10.0.0.1", "curl/7.68.0", emailDispatcherConfig)
	if subject != "Custom DB Security Alert for charlie@example.com" {
		t.Fatalf("expected custom hook subject for suspicious activity, got: %s", subject)
	}
	if html != "<p>Suspicious login from IP: 10.0.0.1</p>" {
		t.Fatalf("expected custom hook html for suspicious activity, got: %s", html)
	}
	if text != "Suspicious login from IP: 10.0.0.1" {
		t.Fatalf("expected custom hook text for suspicious activity, got: %s", text)
	}

	// 2. Test hook returning NULL -> falls back to runtime config templates
	subject, _, text = emailDispatcher.resolveSignInOTPTemplate(ctx, "bob@example.com", "null_fallback", "", emailDispatcherConfig)
	if subject != "Runtime Config Sign In OTP: layr-app" {
		t.Fatalf("expected runtime config template subject when hook returns NULL, got: %s", subject)
	}
	if text != "Runtime OTP Text: null_fallback" {
		t.Fatalf("expected runtime config template text when hook returns NULL, got: %s", text)
	}

	// 3. Drop hook and test fallback to runtime config
	if _, err := db.Exec(ctx, "DROP FUNCTION public.auth_email_template(text, text, text, uuid)"); err != nil {
		t.Fatalf("failed to drop hook function: %v", err)
	}

	subject, _, _ = emailDispatcher.resolvePasswordResetTemplate(ctx, "alice@example.com", "456789", "", emailDispatcherConfig)
	if subject != "Runtime Config Reset: layr-app" {
		t.Fatalf("expected runtime config subject after hook dropped, got: %s", subject)
	}

	// 4. Test built-in defaults when runtime config templates are empty
	emptyTemplateEmailDispatcherConfig := &EmailDispatcherConfig{Driver: &driverWebhook}
	subject, html, text = emailDispatcher.resolvePasswordResetTemplate(ctx, "alice@example.com", "456789", "", emptyTemplateEmailDispatcherConfig)
	if subject != "Reset your password" {
		t.Fatalf("expected built-in default subject, got: %s", subject)
	}
	if html != "<p>Your password reset code is: <strong>456789</strong></p>" {
		t.Fatalf("expected built-in default html, got: %s", html)
	}
	if text != "Your password reset code is: 456789" {
		t.Fatalf("expected built-in default text, got: %s", text)
	}
}
