package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"layr.sh/core"
)

// Named constants for timeouts and HTTP limits to satisfy linter checks.
const (
	defaultSMSHTTPTimeout    = 15 * time.Second
	defaultSMSWebhookTimeout = 10 * time.Second
)

var (
	// ErrSMSDispatcherNotConfigured indicates that SMS delivery has not been configured.
	ErrSMSDispatcherNotConfigured = errors.New("sms delivery is not configured")
	// ErrSMSDispatcherInvalidRecipient indicates an invalid recipient phone number.
	ErrSMSDispatcherInvalidRecipient = errors.New("invalid phone recipient")
	// ErrSMSDispatcherEmptyBody indicates that the message body was empty.
	ErrSMSDispatcherEmptyBody = errors.New("sms body cannot be empty")
	// ErrSMSDispatcherUnsupportedDriver indicates an unrecognized delivery driver.
	ErrSMSDispatcherUnsupportedDriver = errors.New("unsupported sms driver")
)

// SMSDispatcherMessageKind represents the category of transactional SMS.
type SMSDispatcherMessageKind string

const (
	// SMSDispatcherMessageKindPasswordReset identifies password reset SMS.
	SMSDispatcherMessageKindPasswordReset SMSDispatcherMessageKind = "password_reset" //nolint:namingclarity
	// SMSDispatcherMessageKindSignInOTP identifies one-time-password sign-in SMS.
	SMSDispatcherMessageKindSignInOTP SMSDispatcherMessageKind = "sign_in_otp" //nolint:namingclarity
	// SMSDispatcherMessageKindPhoneVerification identifies phone number verification SMS.
	SMSDispatcherMessageKindPhoneVerification SMSDispatcherMessageKind = "phone_verification" //nolint:namingclarity
)

// SMSDispatcherConfig contains configuration for SMS delivery.
type SMSDispatcherConfig struct {
	Driver    *string                      `json:"driver"` // "twilio" | "webhook"
	Twilio    SMSDispatcherTwilioConfig    `json:"twilio"`
	Webhook   SMSDispatcherWebhookConfig   `json:"webhook"`
	Templates SMSDispatcherTemplatesConfig `json:"templates"`
}

// SMSDispatcherTwilioConfig defines Twilio Programmable Messaging credentials.
type SMSDispatcherTwilioConfig struct {
	AccountSID          string `json:"account_sid"`
	AuthToken           string `json:"auth_token,omitempty"`
	AuthTokenConfigured bool   `json:"auth_token_configured,omitempty"`
	FromNumber          string `json:"from_number"`
}

// SMSDispatcherWebhookConfig defines webhook delivery parameters.
type SMSDispatcherWebhookConfig struct {
	URL                     string `json:"url"`
	SigningSecret           string `json:"signing_secret,omitempty"`
	SigningSecretConfigured bool   `json:"signing_secret_configured,omitempty"`
	TimeoutSeconds          int    `json:"timeout_seconds,omitempty"`
}

// SMSDispatcherTemplateConfig defines templates for text messages.
type SMSDispatcherTemplateConfig struct {
	Text string `json:"text"`
}

// SMSDispatcherTemplatesConfig defines message-kind-specific templates.
type SMSDispatcherTemplatesConfig struct {
	PasswordReset     SMSDispatcherTemplateConfig `json:"password_reset"`
	SignInOTP         SMSDispatcherTemplateConfig `json:"sign_in_otp"`
	PhoneVerification SMSDispatcherTemplateConfig `json:"phone_verification"`
}

// SMSDispatcherDefaultConfig returns canonical default configuration for outbound SMS delivery with default templates.
func SMSDispatcherDefaultConfig() SMSDispatcherConfig {
	return SMSDispatcherConfig{
		Driver: nil,
		Templates: SMSDispatcherTemplatesConfig{
			PasswordReset: SMSDispatcherTemplateConfig{
				Text: "Your {{.AppName}} password reset code is: {{.Code}}",
			},
			PhoneVerification: SMSDispatcherTemplateConfig{
				Text: "Your {{.AppName}} phone verification code is: {{.Code}}",
			},
			SignInOTP: SMSDispatcherTemplateConfig{
				Text: "Your {{.AppName}} sign in verification code is: {{.Code}}",
			},
		},
	}
}

// SMSDispatcherMessage represents an outbound transactional SMS message.
type SMSDispatcherMessage struct {
	Kind   SMSDispatcherMessageKind `json:"kind"`
	To     string                   `json:"to"`
	UserID string                   `json:"user_id,omitempty"`
	Code   string                   `json:"code,omitempty"`
	Text   string                   `json:"text"`
}

// SMSDispatcher handles sending transactional SMS messages via Twilio or Webhook.
type SMSDispatcher struct {
	db               *core.DatabasePool
	resolveConfig    func() *SMSDispatcherConfig
	cryptoKeyManager *core.CryptoKeyManager
	httpClient       *http.Client
}

// NewSMSDispatcher initializes an SMSDispatcher with the provided database pool, configuration resolver, and key manager.
func NewSMSDispatcher(db *core.DatabasePool, resolveConfig func() *SMSDispatcherConfig, cryptoKeyManager *core.CryptoKeyManager) *SMSDispatcher {
	return &SMSDispatcher{
		db:               db,
		resolveConfig:    resolveConfig,
		cryptoKeyManager: cryptoKeyManager,
		httpClient:       &http.Client{Timeout: defaultSMSHTTPTimeout},
	}
}

// IsSMSDeliveryReady checks if SMS delivery configuration is valid and active.
func IsSMSDeliveryReady(smsDispatcherConfig SMSDispatcherConfig) bool {
	if smsDispatcherConfig.Driver == nil {
		return false
	}
	switch *smsDispatcherConfig.Driver {
	case "twilio":
		smsDispatcherTwilioConfig := smsDispatcherConfig.Twilio
		if strings.TrimSpace(smsDispatcherTwilioConfig.AccountSID) == "" || strings.TrimSpace(smsDispatcherTwilioConfig.FromNumber) == "" {
			return false
		}
		if strings.TrimSpace(smsDispatcherTwilioConfig.AuthToken) == "" && !smsDispatcherTwilioConfig.AuthTokenConfigured {
			return false
		}
		return true
	case "webhook":
		return strings.TrimSpace(smsDispatcherConfig.Webhook.URL) != ""
	default:
		return false
	}
}

// IsConfigured returns whether SMS delivery is ready.
func (dispatcher *SMSDispatcher) IsConfigured() bool {
	if dispatcher.resolveConfig == nil {
		return false
	}
	smsDispatcherConfig := dispatcher.resolveConfig()
	if smsDispatcherConfig == nil {
		return false
	}
	return IsSMSDeliveryReady(*smsDispatcherConfig)
}

// SendSignInOTP dispatches a sign-in one-time-password SMS.
func (dispatcher *SMSDispatcher) SendSignInOTP(ctx context.Context, toPhone, code, userID string) error {
	return dispatcher.sendWithTemplate(ctx, SMSDispatcherMessageKindSignInOTP, toPhone, code, userID)
}

// SendPhoneVerification dispatches a phone number verification SMS.
func (dispatcher *SMSDispatcher) SendPhoneVerification(ctx context.Context, toPhone, code, userID string) error {
	return dispatcher.sendWithTemplate(ctx, SMSDispatcherMessageKindPhoneVerification, toPhone, code, userID)
}

// SendPasswordReset dispatches a password reset SMS.
func (dispatcher *SMSDispatcher) SendPasswordReset(ctx context.Context, toPhone, code, userID string) error {
	return dispatcher.sendWithTemplate(ctx, SMSDispatcherMessageKindPasswordReset, toPhone, code, userID)
}

func (dispatcher *SMSDispatcher) sendWithTemplate(ctx context.Context, smsDispatcherMessageKind SMSDispatcherMessageKind, toPhone, code, userID string) error {
	log.Tracef("dispatching templated sms kind=%s to=%s userID=%s", smsDispatcherMessageKind, toPhone, userID)

	if !dispatcher.IsConfigured() {
		log.Debugf("sms dispatch rejected: sms dispatcher is not configured")
		return ErrSMSDispatcherNotConfigured
	}
	smsDispatcherConfig := dispatcher.resolveConfig()

	text := dispatcher.resolveTemplate(ctx, smsDispatcherMessageKind, toPhone, code, userID, smsDispatcherConfig)

	return dispatcher.Send(ctx, SMSDispatcherMessage{
		Kind:   smsDispatcherMessageKind,
		To:     toPhone,
		UserID: userID,
		Code:   code,
		Text:   text,
	})
}

func (dispatcher *SMSDispatcher) resolveTemplate(ctx context.Context, smsDispatcherMessageKind SMSDispatcherMessageKind, recipient, code, userID string, smsDispatcherConfig *SMSDispatcherConfig) string {
	log.Tracef("resolving sms template for kind=%s recipient=%s", smsDispatcherMessageKind, recipient)

	// 1. PostgreSQL dynamic hook: public.auth_sms_template(kind, recipient, code, user_id)
	if dispatcher.db != nil {
		var procedureName *string
		_ = dispatcher.db.QueryRow(ctx, "SELECT to_regprocedure('public.auth_sms_template(text,text,text,uuid)')::text").Scan(&procedureName)
		if procedureName != nil && *procedureName != "" {
			var userIDParam any
			if userID != "" {
				userIDParam = userID
			}
			var templateJSON []byte
			err := dispatcher.db.QueryRow(ctx, "SELECT public.auth_sms_template($1, $2, $3, $4::uuid)", string(smsDispatcherMessageKind), recipient, code, userIDParam).Scan(&templateJSON)
			if err == nil && len(templateJSON) > 0 && !bytes.Equal(templateJSON, []byte("null")) {
				var hookResult struct {
					Text string `json:"text"`
				}
				if json.Unmarshal(templateJSON, &hookResult) == nil && hookResult.Text != "" {
					log.Debugf("sms template resolved via PostgreSQL dynamic hook struct for kind=%s", smsDispatcherMessageKind)
					return hookResult.Text
				}
				var hookString string
				if json.Unmarshal(templateJSON, &hookString) == nil && hookString != "" {
					log.Debugf("sms template resolved via PostgreSQL dynamic hook string for kind=%s", smsDispatcherMessageKind)
					return hookString
				}
			}
		}
	}

	// 2. Layr Console / Configured template
	var smsDispatcherTemplateConfig SMSDispatcherTemplateConfig
	switch smsDispatcherMessageKind {
	case SMSDispatcherMessageKindPasswordReset:
		smsDispatcherTemplateConfig = smsDispatcherConfig.Templates.PasswordReset
	case SMSDispatcherMessageKindSignInOTP:
		smsDispatcherTemplateConfig = smsDispatcherConfig.Templates.SignInOTP
	case SMSDispatcherMessageKindPhoneVerification:
		smsDispatcherTemplateConfig = smsDispatcherConfig.Templates.PhoneVerification
	}

	appName := core.GetConfig().Project.Name
	if appName == "" {
		appName = "Layr"
	}

	if smsDispatcherTemplateConfig.Text != "" {
		text := smsDispatcherTemplateConfig.Text
		text = strings.ReplaceAll(text, "{{.Code}}", code)
		text = strings.ReplaceAll(text, "{{.Recipient}}", recipient)
		text = strings.ReplaceAll(text, "{{.To}}", recipient)
		text = strings.ReplaceAll(text, "{{.AppName}}", appName)
		log.Debugf("sms template resolved via configured runtime templates for kind=%s", smsDispatcherMessageKind)
		return text
	}

	// 3. Built-in defaults
	log.Debugf("sms template resolved via built-in default templates for kind=%s", smsDispatcherMessageKind)
	switch smsDispatcherMessageKind {
	case SMSDispatcherMessageKindPasswordReset:
		return fmt.Sprintf("Your %s password reset code is: %s", appName, code)
	case SMSDispatcherMessageKindSignInOTP:
		return fmt.Sprintf("Your %s sign in verification code is: %s", appName, code)
	case SMSDispatcherMessageKindPhoneVerification:
		return fmt.Sprintf("Your %s phone verification code is: %s", appName, code)
	}

	return code
}

// Send dispatches an outbound SMS message according to the configured driver.
func (dispatcher *SMSDispatcher) Send(ctx context.Context, smsDispatcherMessage SMSDispatcherMessage) error {
	log.Tracef("dispatching sms to=%s kind=%s", smsDispatcherMessage.To, smsDispatcherMessage.Kind)

	if !dispatcher.IsConfigured() {
		log.Debugf("sms dispatch rejected: not configured")
		return ErrSMSDispatcherNotConfigured
	}
	smsDispatcherConfig := dispatcher.resolveConfig()

	smsDispatcherMessage.To = strings.TrimSpace(smsDispatcherMessage.To)
	if smsDispatcherMessage.To == "" {
		log.Debugf("sms dispatch rejected: empty recipient")
		return ErrSMSDispatcherInvalidRecipient
	}
	if strings.TrimSpace(smsDispatcherMessage.Text) == "" {
		log.Debugf("sms dispatch rejected: empty text")
		return ErrSMSDispatcherEmptyBody
	}

	var driver string
	if smsDispatcherConfig.Driver != nil {
		driver = strings.ToLower(strings.TrimSpace(*smsDispatcherConfig.Driver))
	}

	if driver == "webhook" {
		return dispatcher.sendViaWebhook(ctx, smsDispatcherMessage, smsDispatcherConfig)
	}
	return dispatcher.sendViaTwilio(ctx, smsDispatcherMessage, smsDispatcherConfig)
}

// sendViaTwilio dispatches an SMS message using Twilio's Programmable Messaging REST API.
// Reference: https://www.twilio.com/docs/messaging/api/message-resource#create-a-message-resource
func (dispatcher *SMSDispatcher) sendViaTwilio(ctx context.Context, smsDispatcherMessage SMSDispatcherMessage, smsDispatcherConfig *SMSDispatcherConfig) error {
	smsDispatcherTwilioConfig := smsDispatcherConfig.Twilio

	authToken := smsDispatcherTwilioConfig.AuthToken
	if authToken != "" && strings.HasPrefix(authToken, "enc:v1:") && dispatcher.cryptoKeyManager != nil {
		decrypted, err := dispatcher.cryptoKeyManager.DecryptField(authToken)
		if err != nil {
			return fmt.Errorf("failed to decrypt twilio auth token: %w", err)
		}
		authToken = string(decrypted)
	}

	apiURL := fmt.Sprintf("https://api.twilio.com/2010-04-01/Accounts/%s/Messages.json", smsDispatcherTwilioConfig.AccountSID)
	values := url.Values{}
	values.Set("To", smsDispatcherMessage.To)
	values.Set("From", smsDispatcherTwilioConfig.FromNumber)
	values.Set("Body", smsDispatcherMessage.Text)

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, apiURL, strings.NewReader(values.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create twilio request: %w", err)
	}
	request.SetBasicAuth(smsDispatcherTwilioConfig.AccountSID, authToken)
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	client := dispatcher.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	log.Debugf("dispatching sms via Twilio to=%s from=%s", smsDispatcherMessage.To, smsDispatcherTwilioConfig.FromNumber)
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("twilio request failed: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("twilio returned error status code: %d", response.StatusCode)
	}

	log.Debugf("twilio sms delivered successfully with status %d", response.StatusCode)
	return nil
}

func (dispatcher *SMSDispatcher) sendViaWebhook(ctx context.Context, smsDispatcherMessage SMSDispatcherMessage, smsDispatcherConfig *SMSDispatcherConfig) error {
	smsDispatcherWebhookConfig := smsDispatcherConfig.Webhook

	timeout := time.Duration(smsDispatcherWebhookConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultSMSWebhookTimeout
	}

	signingSecret := smsDispatcherWebhookConfig.SigningSecret
	if signingSecret != "" && strings.HasPrefix(signingSecret, "enc:v1:") && dispatcher.cryptoKeyManager != nil {
		decrypted, err := dispatcher.cryptoKeyManager.DecryptField(signingSecret)
		if err != nil {
			return fmt.Errorf("failed to decrypt webhook signing secret: %w", err)
		}
		signingSecret = string(decrypted)
	}

	payloadMap := map[string]any{
		"kind":         "sms",
		"message_kind": string(smsDispatcherMessage.Kind),
		"to":           smsDispatcherMessage.To,
		"user_id":      smsDispatcherMessage.UserID,
		"code":         smsDispatcherMessage.Code,
		"text":         smsDispatcherMessage.Text,
	}

	payloadBytes, _ := json.Marshal(payloadMap)

	requestCtx, requestCancel := context.WithTimeout(ctx, timeout)
	defer requestCancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, smsDispatcherWebhookConfig.URL, bytes.NewReader(payloadBytes))
	if err != nil {
		return fmt.Errorf("failed to construct webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	if signingSecret != "" {
		timestamp := fmt.Sprintf("%d", time.Now().Unix())
		signaturePayload := timestamp + "." + string(payloadBytes)
		mac := hmac.New(sha256.New, []byte(signingSecret)) //nolint:namingclarity
		mac.Write([]byte(signaturePayload))
		signature := hex.EncodeToString(mac.Sum(nil))
		request.Header.Set("X-Layr-Signature", fmt.Sprintf("t=%s,v1=%s", timestamp, signature))
	}

	client := dispatcher.httpClient
	if client == nil {
		client = http.DefaultClient
	}

	log.Debugf("dispatching sms webhook to %s", smsDispatcherWebhookConfig.URL)
	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("webhook request failed: %w", err)
	}
	defer func() {
		_ = response.Body.Close()
	}()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("webhook returned non-2xx status code: %d", response.StatusCode)
	}

	log.Debugf("sms webhook delivered successfully with status %d", response.StatusCode)
	return nil
}
