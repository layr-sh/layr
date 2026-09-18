package auth

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"net"
	"net/http"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"

	"layr.sh/core"
)

// Named constants for timeouts and network defaults to satisfy linter checks.
const (
	defaultEmailHTTPTimeout    = 15 * time.Second
	defaultEmailDialTimeout    = 10 * time.Second
	defaultEmailWebhookTimeout = 10 * time.Second
	smtpSecurePort             = 465
)

var (
	// ErrEmailDispatcherNotConfigured indicates that email delivery has not been configured.
	ErrEmailDispatcherNotConfigured = errors.New("email delivery is not configured")
	// ErrEmailDispatcherInvalidRecipient indicates an invalid recipient email address.
	ErrEmailDispatcherInvalidRecipient = errors.New("invalid email recipient")
	// ErrEmailDispatcherEmptySubject indicates that the email subject was empty.
	ErrEmailDispatcherEmptySubject = errors.New("email subject cannot be empty")
	// ErrEmailDispatcherEmptyBody indicates that neither HTML nor text body was provided.
	ErrEmailDispatcherEmptyBody = errors.New("email body cannot be empty")
	// ErrEmailDispatcherUnsupportedDriver indicates an unrecognized delivery driver.
	ErrEmailDispatcherUnsupportedDriver = errors.New("unsupported email driver")
)

// EmailDispatcherMessageKind represents the category of transactional email.
type EmailDispatcherMessageKind string

const (
	// EmailDispatcherMessageKindPasswordReset identifies password reset emails.
	EmailDispatcherMessageKindPasswordReset EmailDispatcherMessageKind = "password_reset" //nolint:namingclarity
	// EmailDispatcherMessageKindSignInOTP identifies one-time-password sign-in emails.
	EmailDispatcherMessageKindSignInOTP EmailDispatcherMessageKind = "sign_in_otp" //nolint:namingclarity
	// EmailDispatcherMessageKindEmailVerification identifies email address verification emails.
	EmailDispatcherMessageKindEmailVerification EmailDispatcherMessageKind = "email_verification" //nolint:namingclarity
	// EmailDispatcherMessageKindSuspiciousActivity identifies suspicious sign-in alert emails.
	EmailDispatcherMessageKindSuspiciousActivity EmailDispatcherMessageKind = "suspicious_activity" //nolint:namingclarity
)

// EmailDispatcherConfig contains configuration for email delivery.
type EmailDispatcherConfig struct {
	Driver      *string                        `json:"driver"` // "smtp" | "webhook"
	SenderEmail string                         `json:"sender_email"`
	SenderName  string                         `json:"sender_name"`
	SMTP        EmailDispatcherSMTPConfig      `json:"smtp"`
	Webhook     EmailDispatcherWebhookConfig   `json:"webhook"`
	Templates   EmailDispatcherTemplatesConfig `json:"templates"`
}

// EmailDispatcherSMTPConfig defines SMTP connection parameters.
type EmailDispatcherSMTPConfig struct {
	Host               string `json:"host"`
	Port               int    `json:"port"`
	Username           string `json:"username"`
	Password           string `json:"password,omitempty"`
	PasswordConfigured bool   `json:"password_configured,omitempty"`
	TLSMode            string `json:"tls_mode,omitempty"` // "auto" | "starttls" | "tls" | "none"
	InsecureSkipVerify bool   `json:"insecure_skip_verify,omitempty"`
}

// EmailDispatcherWebhookConfig defines webhook delivery parameters.
type EmailDispatcherWebhookConfig struct {
	URL                     string `json:"url"`
	SigningSecret           string `json:"signing_secret,omitempty"`
	SigningSecretConfigured bool   `json:"signing_secret_configured,omitempty"`
	TimeoutSeconds          int    `json:"timeout_seconds,omitempty"`
}

// EmailDispatcherTemplateConfig defines templates for subject, HTML, and plain text.
type EmailDispatcherTemplateConfig struct {
	Subject string `json:"subject"`
	HTML    string `json:"html"`
	Text    string `json:"text"`
}

// EmailDispatcherTemplatesConfig defines message-kind-specific templates.
type EmailDispatcherTemplatesConfig struct {
	PasswordReset      EmailDispatcherTemplateConfig `json:"password_reset"`
	SignInOTP          EmailDispatcherTemplateConfig `json:"sign_in_otp"`
	EmailVerification  EmailDispatcherTemplateConfig `json:"email_verification"`
	SuspiciousActivity EmailDispatcherTemplateConfig `json:"suspicious_activity"`
}

// EmailDispatcherDefaultConfig returns canonical default configuration for outbound email delivery with default templates.
func EmailDispatcherDefaultConfig() EmailDispatcherConfig {
	return EmailDispatcherConfig{
		Driver: nil,
		Templates: EmailDispatcherTemplatesConfig{
			EmailVerification: EmailDispatcherTemplateConfig{
				Subject: "Verify your email address",
				HTML:    "<p>Your email verification code for {{.AppName}} is: <strong>{{.Code}}</strong></p>",
				Text:    "Your email verification code for {{.AppName}} is: {{.Code}}",
			},
			PasswordReset: EmailDispatcherTemplateConfig{
				Subject: "Reset your password",
				HTML:    "<p>Your password reset code for {{.AppName}} is: <strong>{{.Code}}</strong></p>",
				Text:    "Your password reset code for {{.AppName}} is: {{.Code}}",
			},
			SignInOTP: EmailDispatcherTemplateConfig{
				Subject: "Your sign in code",
				HTML:    "<p>Your sign in verification code for {{.AppName}} is: <strong>{{.Code}}</strong></p>",
				Text:    "Your sign in verification code for {{.AppName}} is: {{.Code}}",
			},
			SuspiciousActivity: EmailDispatcherTemplateConfig{
				Subject: "New sign-in detected on your account",
				HTML:    "<p>A new sign-in was detected on your {{.AppName}} account from <strong>{{.Device}}</strong> (IP: {{.IP}}).</p><p>If this was not you, please secure your account immediately.</p>",
				Text:    "A new sign-in was detected on your {{.AppName}} account from {{.Device}} (IP: {{.IP}}). If this was not you, please secure your account immediately.",
			},
		},
	}
}

// EmailDispatcherMessage represents an outbound transactional email message.
type EmailDispatcherMessage struct {
	Kind        EmailDispatcherMessageKind `json:"kind"`
	To          string                     `json:"to"`
	UserID      string                     `json:"user_id,omitempty"`
	Code        string                     `json:"code,omitempty"`
	Subject     string                     `json:"subject"`
	HTML        string                     `json:"html"`
	Text        string                     `json:"text"`
	SenderEmail string                     `json:"sender_email,omitempty"`
	SenderName  string                     `json:"sender_name,omitempty"`
}

// EmailDispatcher handles sending transactional emails via SMTP or Webhook.
type EmailDispatcher struct {
	db               *core.DatabasePool
	resolveConfig    func() *EmailDispatcherConfig
	cryptoKeyManager *core.CryptoKeyManager
	httpClient       *http.Client
	dialTCP          func(ctx context.Context, network, address string) (net.Conn, error)
	dialTLS          func(ctx context.Context, network, address string, tlsConfig *tls.Config) (*tls.Conn, error)
}

// NewEmailDispatcher initializes an EmailDispatcher with the provided database pool, configuration resolver, and key manager.
func NewEmailDispatcher(db *core.DatabasePool, resolveConfig func() *EmailDispatcherConfig, cryptoKeyManager *core.CryptoKeyManager) *EmailDispatcher {
	return &EmailDispatcher{
		db:               db,
		resolveConfig:    resolveConfig,
		cryptoKeyManager: cryptoKeyManager,
		httpClient:       &http.Client{Timeout: defaultEmailHTTPTimeout},
	}
}

// IsEmailDeliveryReady checks if email delivery configuration is valid and active.
func IsEmailDeliveryReady(emailDispatcherConfig EmailDispatcherConfig) bool {
	if emailDispatcherConfig.Driver == nil {
		return false
	}
	switch *emailDispatcherConfig.Driver {
	case "smtp":
		emailDispatcherSMTPConfig := emailDispatcherConfig.SMTP
		if strings.TrimSpace(emailDispatcherSMTPConfig.Host) == "" || emailDispatcherSMTPConfig.Port <= 0 || strings.TrimSpace(emailDispatcherSMTPConfig.Username) == "" {
			return false
		}
		if strings.TrimSpace(emailDispatcherSMTPConfig.Password) == "" && !emailDispatcherSMTPConfig.PasswordConfigured {
			return false
		}
		return true
	case "webhook":
		return strings.TrimSpace(emailDispatcherConfig.Webhook.URL) != ""
	default:
		return false
	}
}

// IsConfigured returns whether email delivery is ready.
func (dispatcher *EmailDispatcher) IsConfigured() bool {
	if dispatcher.resolveConfig == nil {
		return false
	}
	emailDispatcherConfig := dispatcher.resolveConfig()
	if emailDispatcherConfig == nil {
		return false
	}
	return IsEmailDeliveryReady(*emailDispatcherConfig)
}

// SendPasswordReset dispatches a password reset email.
func (dispatcher *EmailDispatcher) SendPasswordReset(ctx context.Context, toEmail, code, userID string) error {
	log.Tracef("dispatching password reset email to=%s userID=%s", toEmail, userID)

	if !dispatcher.IsConfigured() {
		log.Debugf("email dispatch rejected: email dispatcher is not configured")
		return ErrEmailDispatcherNotConfigured
	}
	emailDispatcherConfig := dispatcher.resolveConfig()

	subject, htmlBody, textBody := dispatcher.resolvePasswordResetTemplate(ctx, toEmail, code, userID, emailDispatcherConfig)

	senderEmail := emailDispatcherConfig.SenderEmail
	if senderEmail == "" {
		senderEmail = "no-reply@layr.sh"
	}

	return dispatcher.Send(ctx, EmailDispatcherMessage{
		Kind:        EmailDispatcherMessageKindPasswordReset,
		To:          toEmail,
		UserID:      userID,
		Code:        code,
		Subject:     subject,
		HTML:        htmlBody,
		Text:        textBody,
		SenderEmail: senderEmail,
		SenderName:  emailDispatcherConfig.SenderName,
	})
}

// SendSignInOTP dispatches a sign-in one-time-password email.
func (dispatcher *EmailDispatcher) SendSignInOTP(ctx context.Context, toEmail, code, userID string) error {
	log.Tracef("dispatching sign in otp email to=%s userID=%s", toEmail, userID)

	if !dispatcher.IsConfigured() {
		log.Debugf("email dispatch rejected: email dispatcher is not configured")
		return ErrEmailDispatcherNotConfigured
	}
	emailDispatcherConfig := dispatcher.resolveConfig()

	subject, htmlBody, textBody := dispatcher.resolveSignInOTPTemplate(ctx, toEmail, code, userID, emailDispatcherConfig)

	senderEmail := emailDispatcherConfig.SenderEmail
	if senderEmail == "" {
		senderEmail = "no-reply@layr.sh"
	}

	return dispatcher.Send(ctx, EmailDispatcherMessage{
		Kind:        EmailDispatcherMessageKindSignInOTP,
		To:          toEmail,
		UserID:      userID,
		Code:        code,
		Subject:     subject,
		HTML:        htmlBody,
		Text:        textBody,
		SenderEmail: senderEmail,
		SenderName:  emailDispatcherConfig.SenderName,
	})
}

// SendEmailVerification dispatches an email verification email.
func (dispatcher *EmailDispatcher) SendEmailVerification(ctx context.Context, toEmail, code, userID string) error {
	log.Tracef("dispatching email verification email to=%s userID=%s", toEmail, userID)

	if !dispatcher.IsConfigured() {
		log.Debugf("email dispatch rejected: email dispatcher is not configured")
		return ErrEmailDispatcherNotConfigured
	}
	emailDispatcherConfig := dispatcher.resolveConfig()

	subject, htmlBody, textBody := dispatcher.resolveEmailVerificationTemplate(ctx, toEmail, code, userID, emailDispatcherConfig)

	senderEmail := emailDispatcherConfig.SenderEmail
	if senderEmail == "" {
		senderEmail = "no-reply@layr.sh"
	}

	return dispatcher.Send(ctx, EmailDispatcherMessage{
		Kind:        EmailDispatcherMessageKindEmailVerification,
		To:          toEmail,
		UserID:      userID,
		Code:        code,
		Subject:     subject,
		HTML:        htmlBody,
		Text:        textBody,
		SenderEmail: senderEmail,
		SenderName:  emailDispatcherConfig.SenderName,
	})
}

// SendSuspiciousActivity dispatches a security alert email when a sign-in from a new device/IP is detected.
func (dispatcher *EmailDispatcher) SendSuspiciousActivity(ctx context.Context, toEmail, userID string, clientIP, userAgent string) error {
	log.Tracef("dispatching suspicious activity alert to=%s userID=%s ip=%s", toEmail, userID, clientIP)

	if !dispatcher.IsConfigured() {
		log.Debugf("email dispatch rejected: email dispatcher is not configured")
		return ErrEmailDispatcherNotConfigured
	}
	emailDispatcherConfig := dispatcher.resolveConfig()

	subject, htmlBody, textBody := dispatcher.resolveSuspiciousActivityTemplate(ctx, toEmail, userID, clientIP, userAgent, emailDispatcherConfig)

	senderEmail := emailDispatcherConfig.SenderEmail
	if senderEmail == "" {
		senderEmail = "no-reply@layr.sh"
	}

	return dispatcher.Send(ctx, EmailDispatcherMessage{
		Kind:        EmailDispatcherMessageKindSuspiciousActivity,
		To:          toEmail,
		UserID:      userID,
		Subject:     subject,
		HTML:        htmlBody,
		Text:        textBody,
		SenderEmail: senderEmail,
		SenderName:  emailDispatcherConfig.SenderName,
	})
}

func (dispatcher *EmailDispatcher) queryDBEmailTemplate(ctx context.Context, emailDispatcherMessageKind EmailDispatcherMessageKind, recipient, code, userID string) (string, string, string, bool) {
	if dispatcher.db == nil {
		return "", "", "", false
	}
	var procedureName *string
	_ = dispatcher.db.QueryRow(ctx, "SELECT to_regprocedure('public.auth_email_template(text,text,text,uuid)')::text").Scan(&procedureName)
	if procedureName == nil || *procedureName == "" {
		return "", "", "", false
	}
	var userIDParam any
	if userID != "" {
		userIDParam = userID
	}
	var templateJSON []byte
	err := dispatcher.db.QueryRow(ctx, "SELECT public.auth_email_template($1, $2, $3, $4::uuid)", string(emailDispatcherMessageKind), recipient, code, userIDParam).Scan(&templateJSON)
	if err == nil && len(templateJSON) > 0 && !bytes.Equal(templateJSON, []byte("null")) {
		var hookResult struct {
			Subject string `json:"subject"`
			HTML    string `json:"html"`
			Text    string `json:"text"`
		}
		if json.Unmarshal(templateJSON, &hookResult) == nil {
			if hookResult.Subject != "" && (hookResult.HTML != "" || hookResult.Text != "") {
				log.Debugf("email template resolved via PostgreSQL dynamic hook for kind=%s", emailDispatcherMessageKind)
				return hookResult.Subject, hookResult.HTML, hookResult.Text, true
			}
		}
	}
	return "", "", "", false
}

func (dispatcher *EmailDispatcher) resolvePasswordResetTemplate(ctx context.Context, recipient, code, userID string, emailDispatcherConfig *EmailDispatcherConfig) (string, string, string) {
	log.Tracef("resolving email template for kind=%s recipient=%s", EmailDispatcherMessageKindPasswordReset, recipient)

	// 1. PostgreSQL dynamic hook: public.auth_email_template(kind, recipient, code, user_id)
	if subject, htmlBody, textBody, ok := dispatcher.queryDBEmailTemplate(ctx, EmailDispatcherMessageKindPasswordReset, recipient, code, userID); ok {
		return subject, htmlBody, textBody
	}

	// 2. Layr Console / Configured template
	appName := core.GetConfig().Project.Name
	if appName == "" {
		appName = "layr-app"
	}

	interpolate := func(templateText string) string {
		templateText = strings.ReplaceAll(templateText, "{{.Code}}", code)
		templateText = strings.ReplaceAll(templateText, "{{.Recipient}}", recipient)
		templateText = strings.ReplaceAll(templateText, "{{.To}}", recipient)
		templateText = strings.ReplaceAll(templateText, "{{.AppName}}", appName)
		return templateText
	}

	emailDispatcherTemplateConfig := emailDispatcherConfig.Templates.PasswordReset
	subject := interpolate(emailDispatcherTemplateConfig.Subject)
	htmlBody := interpolate(emailDispatcherTemplateConfig.HTML)
	textBody := interpolate(emailDispatcherTemplateConfig.Text)

	if subject != "" && (htmlBody != "" || textBody != "") {
		log.Debugf("email template resolved via configured runtime templates for kind=%s", EmailDispatcherMessageKindPasswordReset)
		return subject, htmlBody, textBody
	}

	// 3. Built-in defaults
	log.Debugf("email template resolved via built-in default templates for kind=%s", EmailDispatcherMessageKindPasswordReset)
	return "Reset your password",
		fmt.Sprintf("<p>Your password reset code is: <strong>%s</strong></p>", code),
		fmt.Sprintf("Your password reset code is: %s", code)
}

func (dispatcher *EmailDispatcher) resolveSignInOTPTemplate(ctx context.Context, recipient, code, userID string, emailDispatcherConfig *EmailDispatcherConfig) (string, string, string) {
	log.Tracef("resolving email template for kind=%s recipient=%s", EmailDispatcherMessageKindSignInOTP, recipient)

	// 1. PostgreSQL dynamic hook: public.auth_email_template(kind, recipient, code, user_id)
	if subject, htmlBody, textBody, ok := dispatcher.queryDBEmailTemplate(ctx, EmailDispatcherMessageKindSignInOTP, recipient, code, userID); ok {
		return subject, htmlBody, textBody
	}

	// 2. Layr Console / Configured template
	appName := core.GetConfig().Project.Name
	if appName == "" {
		appName = "layr-app"
	}

	interpolate := func(templateText string) string {
		templateText = strings.ReplaceAll(templateText, "{{.Code}}", code)
		templateText = strings.ReplaceAll(templateText, "{{.Recipient}}", recipient)
		templateText = strings.ReplaceAll(templateText, "{{.To}}", recipient)
		templateText = strings.ReplaceAll(templateText, "{{.AppName}}", appName)
		return templateText
	}

	emailDispatcherTemplateConfig := emailDispatcherConfig.Templates.SignInOTP
	subject := interpolate(emailDispatcherTemplateConfig.Subject)
	htmlBody := interpolate(emailDispatcherTemplateConfig.HTML)
	textBody := interpolate(emailDispatcherTemplateConfig.Text)

	if subject != "" && (htmlBody != "" || textBody != "") {
		log.Debugf("email template resolved via configured runtime templates for kind=%s", EmailDispatcherMessageKindSignInOTP)
		return subject, htmlBody, textBody
	}

	// 3. Built-in defaults
	log.Debugf("email template resolved via built-in default templates for kind=%s", EmailDispatcherMessageKindSignInOTP)
	return "Your sign in code",
		fmt.Sprintf("<p>Your sign in verification code is: <strong>%s</strong></p>", code),
		fmt.Sprintf("Your sign in verification code is: %s", code)
}

func (dispatcher *EmailDispatcher) resolveEmailVerificationTemplate(ctx context.Context, recipient, code, userID string, emailDispatcherConfig *EmailDispatcherConfig) (string, string, string) {
	log.Tracef("resolving email template for kind=%s recipient=%s", EmailDispatcherMessageKindEmailVerification, recipient)

	// 1. PostgreSQL dynamic hook: public.auth_email_template(kind, recipient, code, user_id)
	if subject, htmlBody, textBody, ok := dispatcher.queryDBEmailTemplate(ctx, EmailDispatcherMessageKindEmailVerification, recipient, code, userID); ok {
		return subject, htmlBody, textBody
	}

	// 2. Layr Console / Configured template
	appName := core.GetConfig().Project.Name
	if appName == "" {
		appName = "layr-app"
	}

	interpolate := func(templateText string) string {
		templateText = strings.ReplaceAll(templateText, "{{.Code}}", code)
		templateText = strings.ReplaceAll(templateText, "{{.Recipient}}", recipient)
		templateText = strings.ReplaceAll(templateText, "{{.To}}", recipient)
		templateText = strings.ReplaceAll(templateText, "{{.AppName}}", appName)
		return templateText
	}

	emailDispatcherTemplateConfig := emailDispatcherConfig.Templates.EmailVerification
	subject := interpolate(emailDispatcherTemplateConfig.Subject)
	htmlBody := interpolate(emailDispatcherTemplateConfig.HTML)
	textBody := interpolate(emailDispatcherTemplateConfig.Text)

	if subject != "" && (htmlBody != "" || textBody != "") {
		log.Debugf("email template resolved via configured runtime templates for kind=%s", EmailDispatcherMessageKindEmailVerification)
		return subject, htmlBody, textBody
	}

	// 3. Built-in defaults
	log.Debugf("email template resolved via built-in default templates for kind=%s", EmailDispatcherMessageKindEmailVerification)
	return "Verify your email address",
		fmt.Sprintf("<p>Your email verification code is: <strong>%s</strong></p>", code),
		fmt.Sprintf("Your email verification code is: %s", code)
}

func (dispatcher *EmailDispatcher) resolveSuspiciousActivityTemplate(ctx context.Context, recipient, userID, clientIP, userAgent string, emailDispatcherConfig *EmailDispatcherConfig) (string, string, string) {
	log.Tracef("resolving email template for kind=%s recipient=%s", EmailDispatcherMessageKindSuspiciousActivity, recipient)

	// 1. PostgreSQL dynamic hook: public.auth_email_template(kind, recipient, code, user_id)
	if subject, htmlBody, textBody, ok := dispatcher.queryDBEmailTemplate(ctx, EmailDispatcherMessageKindSuspiciousActivity, recipient, clientIP, userID); ok {
		return subject, htmlBody, textBody
	}

	// 2. Layr Console / Configured template
	appName := core.GetConfig().Project.Name
	if appName == "" {
		appName = "layr-app"
	}

	emailDispatcherTemplateConfig := emailDispatcherConfig.Templates.SuspiciousActivity
	interpolate := func(templateText string) string {
		templateText = strings.ReplaceAll(templateText, "{{.Recipient}}", recipient)
		templateText = strings.ReplaceAll(templateText, "{{.To}}", recipient)
		templateText = strings.ReplaceAll(templateText, "{{.AppName}}", appName)
		templateText = strings.ReplaceAll(templateText, "{{.IP}}", clientIP)
		templateText = strings.ReplaceAll(templateText, "{{.Device}}", userAgent)
		return templateText
	}

	subject := interpolate(emailDispatcherTemplateConfig.Subject)
	htmlBody := interpolate(emailDispatcherTemplateConfig.HTML)
	textBody := interpolate(emailDispatcherTemplateConfig.Text)

	if subject != "" && (htmlBody != "" || textBody != "") {
		log.Debugf("email template resolved via configured runtime templates for kind=%s", EmailDispatcherMessageKindSuspiciousActivity)
		return subject, htmlBody, textBody
	}

	// 3. Built-in defaults
	log.Debugf("email template resolved via built-in default templates for kind=%s", EmailDispatcherMessageKindSuspiciousActivity)
	defaultEmailDispatcherTemplateConfig := EmailDispatcherDefaultConfig().Templates.SuspiciousActivity
	return interpolate(defaultEmailDispatcherTemplateConfig.Subject), interpolate(defaultEmailDispatcherTemplateConfig.HTML), interpolate(defaultEmailDispatcherTemplateConfig.Text)
}

// Send dispatches an outbound email message according to the configured driver.
func (dispatcher *EmailDispatcher) Send(ctx context.Context, emailDispatcherMessage EmailDispatcherMessage) error {
	log.Tracef("dispatching email to=%s subject=%q kind=%s", emailDispatcherMessage.To, emailDispatcherMessage.Subject, emailDispatcherMessage.Kind)

	if !dispatcher.IsConfigured() {
		log.Debugf("email dispatch rejected: not configured")
		return ErrEmailDispatcherNotConfigured
	}
	emailDispatcherConfig := dispatcher.resolveConfig()

	emailDispatcherMessage.To = strings.TrimSpace(emailDispatcherMessage.To)
	if emailDispatcherMessage.To == "" || !strings.Contains(emailDispatcherMessage.To, "@") || strings.HasPrefix(emailDispatcherMessage.To, "@") || strings.HasSuffix(emailDispatcherMessage.To, "@") {
		log.Debugf("email dispatch rejected: invalid recipient %q", emailDispatcherMessage.To)
		return ErrEmailDispatcherInvalidRecipient
	}
	if strings.TrimSpace(emailDispatcherMessage.Subject) == "" {
		log.Debugf("email dispatch rejected: empty subject")
		return ErrEmailDispatcherEmptySubject
	}
	if strings.TrimSpace(emailDispatcherMessage.HTML) == "" && strings.TrimSpace(emailDispatcherMessage.Text) == "" {
		log.Debugf("email dispatch rejected: empty body")
		return ErrEmailDispatcherEmptyBody
	}

	if emailDispatcherMessage.SenderName == "" {
		if emailDispatcherConfig != nil && emailDispatcherConfig.SenderName != "" {
			emailDispatcherMessage.SenderName = emailDispatcherConfig.SenderName
		} else {
			appName := core.GetConfig().Project.Name
			if appName != "" {
				emailDispatcherMessage.SenderName = appName
			} else {
				emailDispatcherMessage.SenderName = "Layr"
			}
		}
	}

	var driver string
	if emailDispatcherConfig.Driver != nil {
		driver = strings.ToLower(strings.TrimSpace(*emailDispatcherConfig.Driver))
	}

	if driver == "webhook" {
		return dispatcher.sendViaWebhook(ctx, emailDispatcherMessage, emailDispatcherConfig)
	}
	return dispatcher.sendViaSMTP(ctx, emailDispatcherMessage, emailDispatcherConfig)
}

func (dispatcher *EmailDispatcher) sendViaSMTP(ctx context.Context, emailDispatcherMessage EmailDispatcherMessage, emailDispatcherConfig *EmailDispatcherConfig) error {
	emailDispatcherSMTPConfig := emailDispatcherConfig.SMTP
	host := emailDispatcherSMTPConfig.Host
	port := emailDispatcherSMTPConfig.Port
	address := fmt.Sprintf("%s:%d", host, port)

	tlsMode := strings.ToLower(strings.TrimSpace(emailDispatcherSMTPConfig.TLSMode))
	if tlsMode == "" || tlsMode == "auto" {
		if port == smtpSecurePort {
			tlsMode = "tls"
		} else {
			tlsMode = "starttls"
		}
	}

	log.Debugf("connecting to smtp server %s with tlsMode=%s", address, tlsMode)

	tlsConfig := &tls.Config{
		ServerName:         host,
		InsecureSkipVerify: emailDispatcherSMTPConfig.InsecureSkipVerify, //nolint:gosec
	}

	var connection net.Conn
	var err error

	dialContext := dispatcher.dialTCP
	if dialContext == nil {
		dialer := &net.Dialer{Timeout: defaultEmailDialTimeout}
		dialContext = dialer.DialContext
	}

	dialTLSContext := dispatcher.dialTLS
	if dialTLSContext == nil {
		dialTLSContext = func(ctx context.Context, network, addr string, tlsConfig *tls.Config) (*tls.Conn, error) {
			dialer := &tls.Dialer{
				NetDialer: &net.Dialer{Timeout: defaultEmailDialTimeout},
				Config:    tlsConfig,
			}
			tlsConnection, dialErr := dialer.DialContext(ctx, network, addr)
			if dialErr != nil {
				return nil, fmt.Errorf("smtp tls dial failed: %w", dialErr)
			}
			return tlsConnection.(*tls.Conn), nil
		}
	}

	switch tlsMode {
	case "tls":
		connection, err = dialTLSContext(ctx, "tcp", address, tlsConfig)
	default:
		connection, err = dialContext(ctx, "tcp", address)
	}
	if err != nil {
		return fmt.Errorf("failed to connect to smtp server: %w", err)
	}
	defer func() {
		_ = connection.Close()
	}()

	client, err := smtp.NewClient(connection, host)
	if err != nil {
		return fmt.Errorf("failed to create smtp client: %w", err)
	}
	defer func() {
		_ = client.Close()
	}()

	if tlsMode == "starttls" {
		if hasStartTLS, _ := client.Extension("STARTTLS"); hasStartTLS {
			if err = client.StartTLS(tlsConfig); err != nil {
				return fmt.Errorf("starttls handshake failed: %w", err)
			}
		}
	}

	// Authenticate if credentials provided
	password := emailDispatcherSMTPConfig.Password
	if password != "" && strings.HasPrefix(password, "enc:v1:") && dispatcher.cryptoKeyManager != nil {
		decrypted, decryptErr := dispatcher.cryptoKeyManager.DecryptField(password)
		if decryptErr != nil {
			return fmt.Errorf("failed to decrypt smtp password: %w", decryptErr)
		}
		password = string(decrypted)
	}

	if emailDispatcherSMTPConfig.Username != "" && password != "" {
		plainAuth := smtp.PlainAuth("", emailDispatcherSMTPConfig.Username, password, host)
		if err = client.Auth(plainAuth); err != nil {
			return fmt.Errorf("smtp authentication failed: %w", err)
		}
	}

	senderEmail := emailDispatcherMessage.SenderEmail
	if senderEmail == "" {
		senderEmail = emailDispatcherConfig.SenderEmail
	}
	if senderEmail == "" {
		senderEmail = "no-reply@layr.sh"
	}

	if err = client.Mail(senderEmail); err != nil {
		return fmt.Errorf("smtp mail from failed: %w", err)
	}
	if err = client.Rcpt(emailDispatcherMessage.To); err != nil {
		return fmt.Errorf("smtp rcpt to failed: %w", err)
	}

	dataWriteCloser, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data command failed: %w", err)
	}
	rawMIME := formatMIME(emailDispatcherMessage, senderEmail)
	_, _ = dataWriteCloser.Write(rawMIME)
	if err = dataWriteCloser.Close(); err != nil {
		return fmt.Errorf("smtp data delivery failed: %w", err)
	}

	log.Debugf("smtp email delivered successfully to %s", emailDispatcherMessage.To)
	return nil
}

func formatMIME(emailDispatcherMessage EmailDispatcherMessage, senderEmail string) []byte {
	var buffer bytes.Buffer
	senderName := emailDispatcherMessage.SenderName
	if senderName == "" {
		senderName = "Layr"
	}

	fmt.Fprintf(&buffer, "From: \"%s\" <%s>\r\n", senderName, senderEmail)
	fmt.Fprintf(&buffer, "To: %s\r\n", emailDispatcherMessage.To)
	fmt.Fprintf(&buffer, "Subject: =?UTF-8?B?%s?=\r\n", hex.EncodeToString([]byte(emailDispatcherMessage.Subject)))
	buffer.WriteString("MIME-Version: 1.0\r\n")

	multipartWriter := multipart.NewWriter(&buffer)
	fmt.Fprintf(&buffer, "Content-Type: multipart/alternative; boundary=\"%s\"\r\n\r\n", multipartWriter.Boundary())

	if emailDispatcherMessage.Text != "" {
		textMIMEHeader := make(textproto.MIMEHeader)
		textMIMEHeader.Set("Content-Type", "text/plain; charset=UTF-8")
		textMIMEHeader.Set("Content-Transfer-Encoding", "quoted-printable")
		partWriter, _ := multipartWriter.CreatePart(textMIMEHeader)
		_, _ = partWriter.Write([]byte(emailDispatcherMessage.Text))
	}

	if emailDispatcherMessage.HTML != "" {
		htmlMIMEHeader := make(textproto.MIMEHeader)
		htmlMIMEHeader.Set("Content-Type", "text/html; charset=UTF-8")
		htmlMIMEHeader.Set("Content-Transfer-Encoding", "quoted-printable")
		partWriter, _ := multipartWriter.CreatePart(htmlMIMEHeader)
		_, _ = partWriter.Write([]byte(emailDispatcherMessage.HTML))
	}

	_ = multipartWriter.Close()
	return buffer.Bytes()
}

func (dispatcher *EmailDispatcher) sendViaWebhook(ctx context.Context, emailDispatcherMessage EmailDispatcherMessage, emailDispatcherConfig *EmailDispatcherConfig) error {
	emailDispatcherWebhookConfig := emailDispatcherConfig.Webhook

	timeout := time.Duration(emailDispatcherWebhookConfig.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = defaultEmailWebhookTimeout
	}

	signingSecret := emailDispatcherWebhookConfig.SigningSecret
	if signingSecret != "" && strings.HasPrefix(signingSecret, "enc:v1:") && dispatcher.cryptoKeyManager != nil {
		decrypted, err := dispatcher.cryptoKeyManager.DecryptField(signingSecret)
		if err != nil {
			return fmt.Errorf("failed to decrypt webhook signing secret: %w", err)
		}
		signingSecret = string(decrypted)
	}

	payloadMap := map[string]any{
		"kind":         "email",
		"message_kind": string(emailDispatcherMessage.Kind),
		"to":           emailDispatcherMessage.To,
		"user_id":      emailDispatcherMessage.UserID,
		"code":         emailDispatcherMessage.Code,
		"subject":      emailDispatcherMessage.Subject,
		"html":         emailDispatcherMessage.HTML,
		"text":         emailDispatcherMessage.Text,
		"sender_email": emailDispatcherMessage.SenderEmail,
		"sender_name":  emailDispatcherMessage.SenderName,
	}

	payloadBytes, _ := json.Marshal(payloadMap)

	requestCtx, requestCancel := context.WithTimeout(ctx, timeout)
	defer requestCancel()

	request, err := http.NewRequestWithContext(requestCtx, http.MethodPost, emailDispatcherWebhookConfig.URL, bytes.NewReader(payloadBytes))
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

	log.Debugf("dispatching email webhook to %s", emailDispatcherWebhookConfig.URL)
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

	log.Debugf("email webhook delivered successfully with status %d", response.StatusCode)
	return nil
}
