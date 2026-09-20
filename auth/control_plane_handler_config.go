package auth

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"layr.sh/core"
)

// handleGetConfig handles GET /api/v1/_/auth/config returning sanitized config without raw secrets.
func (controlPlaneHandler *ControlPlaneHandler) handleGetConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Tracef("handleGetConfig invoked")

	if !controlPlaneHandler.checkScope(request, "auth:config.read") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:config.read required")
		return
	}

	if controlPlaneHandler.configManager == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Config manager not initialized")
		return
	}

	sanitizedConfig := controlPlaneHandler.configManager.GetUnencrypted()
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(sanitizedConfig)
}

// handleUpdateConfig handles PUT /api/v1/_/auth/config updating runtime config with envelope encryption.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateConfig(responseWriter http.ResponseWriter, request *http.Request) {
	log.Tracef("handleUpdateConfig invoked")

	if !controlPlaneHandler.checkScope(request, "auth:config.write") {
		core.WriteErrorResponse(responseWriter, request, http.StatusForbidden, "Forbidden: scope auth:config.write required")
		return
	}

	if controlPlaneHandler.configManager == nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, "Config manager not initialized")
		return
	}

	currentConfig := controlPlaneHandler.configManager.Get()
	inputConfig := controlPlaneHandler.configManager.Get()
	bodyBytes, readErr := io.ReadAll(request.Body)
	if readErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid JSON payload")
		return
	}
	defer func() {
		_ = request.Body.Close()
	}()

	if err := json.Unmarshal(bodyBytes, &inputConfig); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("Invalid JSON payload: %v", err))
		return
	}

	var rawPut struct {
		OIDC struct {
			UI struct {
				ShowPassword *bool `json:"show_password"`
				ShowSignUp   *bool `json:"show_sign_up"`
				ShowPasskeys *bool `json:"show_passkeys"`
				ShowEmailOTP *bool `json:"show_email_otp"`
				ShowSMSOTP   *bool `json:"show_sms_otp"`
				ShowOAuth    *bool `json:"show_oauth"`
			} `json:"ui"`
		} `json:"oidc"`
	}
	_ = json.Unmarshal(bodyBytes, &rawPut)

	// Handle OAuth secrets: if new plaintext provided, envelope-encrypt; if omitted, preserve current
	if inputConfig.OAuthProviders != nil {
		for providerKey, provider := range inputConfig.OAuthProviders {
			currentOAuthProviderConfig := currentConfig.OAuthProviders[providerKey]
			secret := strings.TrimSpace(provider.ClientSecret)

			if secret != "" && !strings.HasPrefix(secret, "enc:v1:") {
				if controlPlaneHandler.configManager.cryptoKeyManager != nil {
					encryptedSecret, _ := controlPlaneHandler.configManager.cryptoKeyManager.EncryptField([]byte(secret))
					provider.ClientSecret = encryptedSecret
				}
			} else if secret == "" {
				provider.ClientSecret = currentOAuthProviderConfig.ClientSecret
			}
			inputConfig.OAuthProviders[providerKey] = provider
		}
	} else {
		inputConfig.OAuthProviders = currentConfig.OAuthProviders
	}

	// Handle OIDC clients secrets: if new plaintext provided, envelope-encrypt; if omitted, preserve current
	if inputConfig.OIDC.Clients != nil {
		currentOIDCClientConfigsByID := make(map[string]OIDCClientConfig, len(currentConfig.OIDC.Clients))
		for _, currentOIDCClientConfig := range currentConfig.OIDC.Clients {
			currentOIDCClientConfigsByID[currentOIDCClientConfig.ClientID] = currentOIDCClientConfig
		}

		for index, client := range inputConfig.OIDC.Clients {
			currentOIDCClientConfig := currentOIDCClientConfigsByID[client.ClientID]
			secret := strings.TrimSpace(client.ClientSecret)

			if secret != "" && !strings.HasPrefix(secret, "enc:v1:") {
				if controlPlaneHandler.configManager.cryptoKeyManager != nil {
					encryptedSecret, _ := controlPlaneHandler.configManager.cryptoKeyManager.EncryptField([]byte(secret))
					client.ClientSecret = encryptedSecret
				}
			} else if secret == "" {
				client.ClientSecret = currentOIDCClientConfig.ClientSecret
			}
			inputConfig.OIDC.Clients[index] = client
		}
	} else {
		inputConfig.OIDC.Clients = currentConfig.OIDC.Clients
	}

	if inputConfig.OIDC.ResourceServers == nil {
		inputConfig.OIDC.ResourceServers = currentConfig.OIDC.ResourceServers
	}

	// Handle Email secrets: if new plaintext provided, envelope-encrypt; if omitted, preserve current
	if inputConfig.EmailDispatcher.Driver != nil && strings.TrimSpace(*inputConfig.EmailDispatcher.Driver) == "" {
		inputConfig.EmailDispatcher.Driver = nil
	}
	emailSMTPPassword := strings.TrimSpace(inputConfig.EmailDispatcher.SMTP.Password)
	if emailSMTPPassword != "" && !strings.HasPrefix(emailSMTPPassword, "enc:v1:") {
		if controlPlaneHandler.configManager.cryptoKeyManager != nil {
			encryptedSecret, _ := controlPlaneHandler.configManager.cryptoKeyManager.EncryptField([]byte(emailSMTPPassword))
			inputConfig.EmailDispatcher.SMTP.Password = encryptedSecret
		}
	} else if emailSMTPPassword == "" {
		inputConfig.EmailDispatcher.SMTP.Password = currentConfig.EmailDispatcher.SMTP.Password
	}

	emailWebhookSigningSecret := strings.TrimSpace(inputConfig.EmailDispatcher.Webhook.SigningSecret)
	if emailWebhookSigningSecret != "" && !strings.HasPrefix(emailWebhookSigningSecret, "enc:v1:") {
		if controlPlaneHandler.configManager.cryptoKeyManager != nil {
			encryptedSecret, _ := controlPlaneHandler.configManager.cryptoKeyManager.EncryptField([]byte(emailWebhookSigningSecret))
			inputConfig.EmailDispatcher.Webhook.SigningSecret = encryptedSecret
		}
	} else if emailWebhookSigningSecret == "" {
		inputConfig.EmailDispatcher.Webhook.SigningSecret = currentConfig.EmailDispatcher.Webhook.SigningSecret
	}

	// Handle SMS secrets: if new plaintext provided, envelope-encrypt; if omitted, preserve current
	if inputConfig.SMSDispatcher.Driver != nil && strings.TrimSpace(*inputConfig.SMSDispatcher.Driver) == "" {
		inputConfig.SMSDispatcher.Driver = nil
	}
	smsTwilioAuthToken := strings.TrimSpace(inputConfig.SMSDispatcher.Twilio.AuthToken)
	if smsTwilioAuthToken != "" && !strings.HasPrefix(smsTwilioAuthToken, "enc:v1:") {
		if controlPlaneHandler.configManager.cryptoKeyManager != nil {
			encryptedSecret, _ := controlPlaneHandler.configManager.cryptoKeyManager.EncryptField([]byte(smsTwilioAuthToken))
			inputConfig.SMSDispatcher.Twilio.AuthToken = encryptedSecret
		}
	} else if smsTwilioAuthToken == "" {
		inputConfig.SMSDispatcher.Twilio.AuthToken = currentConfig.SMSDispatcher.Twilio.AuthToken
	}

	smsWebhookSigningSecret := strings.TrimSpace(inputConfig.SMSDispatcher.Webhook.SigningSecret)
	if smsWebhookSigningSecret != "" && !strings.HasPrefix(smsWebhookSigningSecret, "enc:v1:") {
		if controlPlaneHandler.configManager.cryptoKeyManager != nil {
			encryptedSecret, _ := controlPlaneHandler.configManager.cryptoKeyManager.EncryptField([]byte(smsWebhookSigningSecret))
			inputConfig.SMSDispatcher.Webhook.SigningSecret = encryptedSecret
		}
	} else if smsWebhookSigningSecret == "" {
		inputConfig.SMSDispatcher.Webhook.SigningSecret = currentConfig.SMSDispatcher.Webhook.SigningSecret
	}

	// Handle CAPTCHA secrets: if new plaintext provided, envelope-encrypt; if omitted, preserve current
	captchaSecret := strings.TrimSpace(inputConfig.Threat.BotProtection.SecretKey)
	if captchaSecret != "" && !strings.HasPrefix(captchaSecret, "enc:v1:") {
		if controlPlaneHandler.configManager.cryptoKeyManager != nil {
			encryptedSecret, _ := controlPlaneHandler.configManager.cryptoKeyManager.EncryptField([]byte(captchaSecret))
			inputConfig.Threat.BotProtection.SecretKey = encryptedSecret
		}
	} else if captchaSecret == "" {
		inputConfig.Threat.BotProtection.SecretKey = currentConfig.Threat.BotProtection.SecretKey
	}

	if inputConfig.Threat.BotProtection.Enabled {
		providerName := strings.ToLower(strings.TrimSpace(inputConfig.Threat.BotProtection.Provider))
		switch providerName {
		case "turnstile", "cloudflare", "recaptcha", "google", "hcaptcha":
			// valid
		default:
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Unsupported CAPTCHA provider")
			return
		}
		if inputConfig.Threat.BotProtection.SecretKey == "" {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "CAPTCHA secret key is required when bot protection is enabled")
			return
		}
		if inputConfig.Threat.BotProtection.Mode != "" && inputConfig.Threat.BotProtection.Mode != "always" && inputConfig.Threat.BotProtection.Mode != "adaptive" {
			core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid bot protection mode; must be always or adaptive")
			return
		}
	}

	if inputConfig.EmailOTP.Enabled && !IsEmailDeliveryReady(inputConfig.EmailDispatcher) {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, "Email delivery is unavailable because SMTP is not configured by the console user")
		return
	}
	if inputConfig.SMSOTP.Enabled && !IsSMSDeliveryReady(inputConfig.SMSDispatcher) {
		core.WriteErrorResponse(responseWriter, request, http.StatusUnprocessableEntity, "SMS delivery is unavailable because an SMS provider is not configured")
		return
	}

	hasOAuthInput := false
	for _, provider := range inputConfig.OAuthProviders {
		if provider.Enabled {
			hasOAuthInput = true
			break
		}
	}
	hasOAuthCurrent := false
	for _, provider := range currentConfig.OAuthProviders {
		if provider.Enabled {
			hasOAuthCurrent = true
			break
		}
	}

	// 1. Prevent user to update showXxx method to true when the method isn't enabled in the config
	if rawPut.OIDC.UI.ShowPassword != nil && *rawPut.OIDC.UI.ShowPassword && !inputConfig.Password.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_password in UI config when password authentication is disabled")
		return
	}
	if rawPut.OIDC.UI.ShowSignUp != nil && *rawPut.OIDC.UI.ShowSignUp && !inputConfig.Password.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_sign_up in UI config when password authentication is disabled")
		return
	}
	if rawPut.OIDC.UI.ShowPasskeys != nil && *rawPut.OIDC.UI.ShowPasskeys && !inputConfig.Passkeys.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_passkeys in UI config when passkey authentication is disabled")
		return
	}
	if rawPut.OIDC.UI.ShowEmailOTP != nil && *rawPut.OIDC.UI.ShowEmailOTP && !inputConfig.EmailOTP.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_email_otp in UI config when email OTP authentication is disabled")
		return
	}
	if rawPut.OIDC.UI.ShowSMSOTP != nil && *rawPut.OIDC.UI.ShowSMSOTP && !inputConfig.SMSOTP.Enabled {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_sms_otp in UI config when SMS OTP authentication is disabled")
		return
	}
	if rawPut.OIDC.UI.ShowOAuth != nil && *rawPut.OIDC.UI.ShowOAuth && !hasOAuthInput {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Cannot enable show_oauth in UI config when no OAuth providers are enabled")
		return
	}

	// 2. Updating an auth method enabled to true, automatically set showXxx to true in ui config
	if !currentConfig.Password.Enabled && inputConfig.Password.Enabled && rawPut.OIDC.UI.ShowPassword == nil {
		inputConfig.OIDC.UI.ShowPassword = true
	}
	if !currentConfig.Passkeys.Enabled && inputConfig.Passkeys.Enabled && rawPut.OIDC.UI.ShowPasskeys == nil {
		inputConfig.OIDC.UI.ShowPasskeys = true
	}
	if !currentConfig.EmailOTP.Enabled && inputConfig.EmailOTP.Enabled && rawPut.OIDC.UI.ShowEmailOTP == nil {
		inputConfig.OIDC.UI.ShowEmailOTP = true
	}
	if !currentConfig.SMSOTP.Enabled && inputConfig.SMSOTP.Enabled && rawPut.OIDC.UI.ShowSMSOTP == nil {
		inputConfig.OIDC.UI.ShowSMSOTP = true
	}
	if !hasOAuthCurrent && hasOAuthInput && rawPut.OIDC.UI.ShowOAuth == nil {
		inputConfig.OIDC.UI.ShowOAuth = true
	}

	// 3. Setting config auth method to false, automatically make ui config showXxx to false
	if !inputConfig.Password.Enabled {
		inputConfig.OIDC.UI.ShowPassword = false
		inputConfig.OIDC.UI.ShowSignUp = false
	}
	if !inputConfig.Passkeys.Enabled {
		inputConfig.OIDC.UI.ShowPasskeys = false
	}
	if !inputConfig.EmailOTP.Enabled {
		inputConfig.OIDC.UI.ShowEmailOTP = false
	}
	if !inputConfig.SMSOTP.Enabled {
		inputConfig.OIDC.UI.ShowSMSOTP = false
	}
	if !hasOAuthInput {
		inputConfig.OIDC.UI.ShowOAuth = false
	}

	if err := controlPlaneHandler.configManager.Save(request.Context(), inputConfig); err != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, err.Error())
		return
	}

	if controlPlaneHandler.eventBus != nil {
		controlPlaneHandler.eventBus.Publish(request.Context(), NewConfigUpdatedEvent(ConfigKey, ConfigUpdatedEventData(controlPlaneHandler.configManager.GetUnencrypted())))
	}

	log.Debugf("handleUpdateConfig successfully saved and sanitized configuration")
	responseWriter.Header().Set("Content-Type", "application/json")
	responseWriter.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(responseWriter).Encode(controlPlaneHandler.configManager.GetUnencrypted())
}
