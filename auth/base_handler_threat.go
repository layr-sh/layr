package auth

import (
	"fmt"
	"net/http"
	"strings"
	"uuid"

	"layr.sh/auth/threat"
	"layr.sh/core"
)

// checkCaptcha verifies the CAPTCHA token when Bot Protection is enabled or adaptive challenge triggers.
// Returns true if verification passed or was bypassed, and false if verification failed and an error was written.
func (handler *BaseHandler) checkCaptcha(responseWriter http.ResponseWriter, request *http.Request, clientIP string, token string, endpoint string) bool {
	botProtectionConfig := handler.configManager.Get().Threat.BotProtection
	if !botProtectionConfig.Enabled {
		return true
	}

	ctx := request.Context()
	requiresChallenge := false
	if botProtectionConfig.Mode == "always" {
		requiresChallenge = true
	} else {
		// Adaptive mode: challenge if failed attempts exceed configured threshold
		threshold := int64(botProtectionConfig.AdaptiveFailedAttempts)
		if threat.GetFailedAttempts(ctx, handler.kernel.KVStore(), clientIP) >= threshold {
			requiresChallenge = true
		}
	}

	if !requiresChallenge {
		return true
	}

	trimmedToken := strings.TrimSpace(token)
	if trimmedToken == "" {
		handler.kernel.EventBus().Publish(ctx, NewBotChallengeFailedEvent(clientIP, BotChallengeFailedEventData{
			IPAddress: clientIP,
			Provider:  botProtectionConfig.Provider,
			Endpoint:  endpoint,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "CAPTCHA verification required")
		return false
	}

	secretKey, decryptErr := handler.configManager.DecryptSecret(botProtectionConfig.SecretKey)
	if decryptErr != nil || secretKey == "" {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to decrypt bot protection secret key: %v", decryptErr))
		return false
	}

	captchaVerifier, verifierErr := threat.NewCaptchaVerifier(botProtectionConfig.Provider, secretKey, handler.httpClient)
	if verifierErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Service temporarily unavailable", fmt.Sprintf("failed to initialize captcha verifier: %v", verifierErr))
		return false
	}

	isValid, verifyErr := captchaVerifier.Verify(ctx, trimmedToken, clientIP)
	if verifyErr != nil || !isValid {
		handler.kernel.EventBus().Publish(ctx, NewBotChallengeFailedEvent(clientIP, BotChallengeFailedEventData{
			IPAddress: clientIP,
			Provider:  botProtectionConfig.Provider,
			Endpoint:  endpoint,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "CAPTCHA verification failed")
		return false
	}

	return true
}

// checkPasswordBreach verifies whether the prospective password exists in public credential leak databases.
// Returns true if password is safe or breach check is unconfigured/fail-open, and false if breached with error written.
func (handler *BaseHandler) checkPasswordBreach(responseWriter http.ResponseWriter, request *http.Request, prospectivePassword string, email string) bool {
	passwordBreachConfig := handler.configManager.Get().Password.BreachCheck
	if !passwordBreachConfig.Enabled {
		return true
	}

	ctx := request.Context()
	isBreached, breachCount, breachErr := threat.CheckPwnedPassword(ctx, handler.httpClient, handler.kernel.KVStore(), prospectivePassword, passwordBreachConfig.FailOpen)
	if breachErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusServiceUnavailable, "Service temporarily unavailable", fmt.Sprintf("password breach check failed: %v", breachErr))
		return false
	}

	if isBreached {
		handler.kernel.EventBus().Publish(ctx, NewPasswordBreachBlockedEvent(email, PasswordBreachBlockedEventData{
			Email: email,
			Count: breachCount,
		}))
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "This password has appeared in a known data breach. Please choose a different, more secure password.")
		return false
	}

	return true
}

// completeSignInFlow processes post-authentication threat mitigation, risk evaluation, suspicious activity alerts,
// adaptive MFA enforcement, and session token issuance across all authentication mechanisms.
func (handler *BaseHandler) completeSignInFlow(responseWriter http.ResponseWriter, request *http.Request, user User, sessionMeta ...string) {
	ctx := request.Context()
	clientIP := core.ExtractRequestClientIP(request)
	userAgent := request.UserAgent()

	_ = threat.ResetFailedAttempts(ctx, handler.kernel.KVStore(), clientIP)
	_ = threat.ResetFailedAttempts(ctx, handler.kernel.KVStore(), user.ID)

	riskAssessment := threat.EvaluateSignInRisk(ctx, handler.kernel.DB(), handler.kernel.KVStore(), user.ID, clientIP, userAgent, handler.configManager.Get().Threat.KnownDevicesMaxDays)
	if riskAssessment != nil && riskAssessment.IsNewDevice && handler.configManager.Get().Threat.NotifyOnNewDevice {
		if user.Email != nil && *user.Email != "" {
			_ = handler.emailDispatcher.SendSuspiciousActivity(ctx, *user.Email, user.ID, clientIP, userAgent)
		}
		handler.kernel.EventBus().Publish(ctx, NewSuspiciousSignInEvent(user.ID, SuspiciousSignInEventData{
			User:      user,
			IPAddress: clientIP,
			UserAgent: userAgent,
			RiskScore: riskAssessment.Score,
			RiskLevel: riskAssessment.Level,
			Reasons:   riskAssessment.Reasons,
		}))
	}

	mfaConfig := handler.configManager.Get().MFA
	if user.MFAEnabled && mfaConfig.Enabled {
		shouldTriggerMFA := false
		if mfaConfig.Policy == "always" || mfaConfig.Policy == "" {
			shouldTriggerMFA = true
		} else if mfaConfig.Policy == "adaptive" && riskAssessment != nil {
			if riskAssessment.Level == threat.RiskLevelMedium || riskAssessment.Level == threat.RiskLevelHigh {
				shouldTriggerMFA = true
			} else {
				for _, configuredTrigger := range mfaConfig.RiskTriggers {
					for _, assessedReason := range riskAssessment.Reasons {
						if configuredTrigger == assessedReason {
							shouldTriggerMFA = true
							break
						}
					}
					if shouldTriggerMFA {
						break
					}
				}
			}
		}

		if shouldTriggerMFA {
			mfaTicket := "mfa_tk_" + uuid.NewV7().String()
			_ = handler.kernel.KVStore().Set(ctx, "auth:mfa_ticket:"+mfaTicket, user.ID, mfaTicketTTL)
			core.WriteJSONResponse(responseWriter, http.StatusOK, SignInResponse{
				MFARequired: true,
				MFATicket:   mfaTicket,
				Factor:      "totp",
			})
			return
		}
	}

	handler.issueSessionResponse(responseWriter, request, user, sessionMeta...)
}
