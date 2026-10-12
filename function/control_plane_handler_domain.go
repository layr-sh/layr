// Package function defines the serverless and edge function execution engine.
package function

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"uuid"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"golang.org/x/crypto/acme"
	"golang.org/x/crypto/acme/autocert"
	"layr.sh/core"
)

const (
	maxDomainLength             = 253
	maxDomainLabelLength        = 63
	maxPathPrefixLength         = 255
	customDomainCertRenewWindow = 7 * 24 * time.Hour
)

// TLSProvisionerFunc defines the signature for custom domain ACME certificate provisioning.
type TLSProvisionerFunc func(ctx context.Context, server *core.Server, domain string, email string) error

// namespacedACMEAccountCache partitions ACME account keys by email in autocert.Cache
// while preserving domain certificates under their unnamespaced domain keys.
type namespacedACMEAccountCache struct {
	email           string
	underlyingCache autocert.Cache
}

func (namespacedCache *namespacedACMEAccountCache) transformKey(key string) string {
	if namespacedCache.email != "" && strings.HasSuffix(key, "+key") {
		return fmt.Sprintf("%s:%s", key, strings.ToLower(namespacedCache.email))
	}
	return key
}

func (namespacedCache *namespacedACMEAccountCache) Get(ctx context.Context, key string) ([]byte, error) {
	data, err := namespacedCache.underlyingCache.Get(ctx, namespacedCache.transformKey(key))
	if err != nil {
		return nil, err //nolint:wrapcheck
	}
	return data, nil
}

func (namespacedCache *namespacedACMEAccountCache) Put(ctx context.Context, key string, data []byte) error {
	if err := namespacedCache.underlyingCache.Put(ctx, namespacedCache.transformKey(key), data); err != nil {
		return fmt.Errorf("failed to put cert data in cache: %w", err)
	}
	return nil
}

func (namespacedCache *namespacedACMEAccountCache) Delete(ctx context.Context, key string) error {
	if err := namespacedCache.underlyingCache.Delete(ctx, namespacedCache.transformKey(key)); err != nil {
		return fmt.Errorf("failed to delete cert data from cache: %w", err)
	}
	return nil
}

var (
	domainLabelPattern = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

	lookupCNAMEResolver = func(ctx context.Context, host string) (string, error) {
		return net.DefaultResolver.LookupCNAME(ctx, host)
	}
	lookupIPResolver = func(ctx context.Context, host string) ([]net.IP, error) {
		return net.DefaultResolver.LookupIP(ctx, "ip", host)
	}
	tlsProvisionerFunc TLSProvisionerFunc = func(ctx context.Context, server *core.Server, domain string, email string) error {
		if server == nil || server.DBCertCache() == nil {
			return errors.New("server database certificate cache is not initialized")
		}

		resolvedEmail := strings.TrimSpace(email)
		if resolvedEmail == "" {
			resolvedEmail = strings.TrimSpace(core.GetConfig().Server.TLS.AcmeEmail)
		}

		accountCache := &namespacedACMEAccountCache{
			email:           resolvedEmail,
			underlyingCache: server.DBCertCache(),
		}

		manager := &autocert.Manager{
			Prompt:     autocert.AcceptTOS,
			Cache:      accountCache,
			Email:      resolvedEmail,
			HostPolicy: server.HostPolicy,
		}

		if core.GetConfig().Server.TLS.AcmeStaging {
			manager.Client = &acme.Client{
				DirectoryURL: "https://acme-staging-v02.api.letsencrypt.org/directory",
			}
		}

		type certResult struct {
			certificate *tls.Certificate
			err         error
		}
		resultChannel := make(chan certResult, 1)
		go func() {
			certificate, certErr := manager.GetCertificate(&tls.ClientHelloInfo{
				ServerName: domain,
				CipherSuites: []uint16{
					tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
					tls.TLS_AES_128_GCM_SHA256,
					tls.TLS_AES_256_GCM_SHA384,
					tls.TLS_CHACHA20_POLY1305_SHA256,
				},
				SupportedCurves: []tls.CurveID{
					tls.CurveP256,
					tls.X25519,
				},
				SignatureSchemes: []tls.SignatureScheme{
					tls.ECDSAWithP256AndSHA256,
				},
			})
			resultChannel <- certResult{certificate: certificate, err: certErr}
		}()

		select {
		case <-ctx.Done():
			return fmt.Errorf("tls provisioning canceled or timed out: %w", ctx.Err())
		case outcomeCertResult := <-resultChannel:
			if outcomeCertResult.err != nil {
				return fmt.Errorf("failed to get autocert certificate: %w", outcomeCertResult.err)
			}
			return nil
		}
	}
)

// NormalizeAndValidateDomain normalizes and validates a domain name.
func NormalizeAndValidateDomain(rawDomain string) (string, error) {
	trimmedDomain := strings.TrimSpace(rawDomain)
	if trimmedDomain == "" {
		return "", errors.New("domain name cannot be empty")
	}

	// Strip port if present
	if strings.Contains(trimmedDomain, ":") {
		host, port, splitErr := net.SplitHostPort(trimmedDomain)
		if splitErr != nil {
			return "", fmt.Errorf("invalid domain host port format: %w", splitErr)
		}
		if _, parsePortErr := strconv.Atoi(port); parsePortErr != nil {
			return "", fmt.Errorf("invalid domain port number: %w", parsePortErr)
		}
		trimmedDomain = host
	}

	normalized := strings.ToLower(trimmedDomain)
	if len(normalized) > maxDomainLength {
		return "", errors.New("domain name exceeds maximum length of 253 characters")
	}

	labels := strings.Split(normalized, ".")
	if len(labels) < 2 {
		return "", errors.New("domain must contain at least one dot separating subdomains")
	}

	for _, label := range labels {
		if label == "" {
			return "", errors.New("domain label cannot be empty")
		}
		if len(label) > maxDomainLabelLength {
			return "", errors.New("domain label exceeds maximum length of 63 characters")
		}
		if !domainLabelPattern.MatchString(label) {
			return "", fmt.Errorf("domain label %q contains invalid characters or malformed hyphens", label)
		}
	}

	return normalized, nil
}

func validateTLSAcmeEmail(rawEmail string) error {
	trimmed := strings.TrimSpace(rawEmail)
	if trimmed == "" {
		return errors.New("tls_acme_email cannot be empty")
	}
	if !strings.Contains(trimmed, "@") || strings.HasPrefix(trimmed, "@") || strings.HasSuffix(trimmed, "@") {
		return errors.New("tls_acme_email must be a valid email address containing '@'")
	}
	parts := strings.Split(trimmed, "@")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" || !strings.Contains(parts[1], ".") {
		return errors.New("tls_acme_email must contain a valid domain name")
	}
	return nil
}

// NormalizeAndValidatePathPrefix normalizes and validates a custom domain routing path prefix.
func NormalizeAndValidatePathPrefix(rawPrefix string) (string, error) {
	trimmed := strings.TrimSpace(rawPrefix)
	if trimmed == "" || trimmed == "/" {
		return "/", nil
	}

	if strings.Contains(trimmed, "?") || strings.Contains(trimmed, "#") {
		return "", errors.New("path prefix cannot contain query parameters or fragment identifiers")
	}

	if strings.ContainsAny(trimmed, " \t\r\n") {
		return "", errors.New("path prefix cannot contain whitespace characters")
	}

	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}

	cleanPath := path.Clean(trimmed)
	if len(cleanPath) > maxPathPrefixLength {
		return "", errors.New("path prefix exceeds maximum length of 255 characters")
	}

	return cleanPath, nil
}

// buildRequiredDNSRecords generates DNS record instructions required to point traffic to Layr.
func (controlPlaneHandler *ControlPlaneHandler) buildRequiredDNSRecords(domain string, domainStatus string, tlsStatus string) []RequiredDNSRecord {
	cnameTarget := "localhost"
	var apexIP string
	if controlPlaneHandler.configManager != nil {
		functionConfig := controlPlaneHandler.configManager.Get()
		if trimmedCNAME := strings.TrimSpace(functionConfig.CustomDomainCNAMETarget); trimmedCNAME != "" {
			cnameTarget = trimmedCNAME
		}
		apexIP = strings.TrimSpace(functionConfig.CustomDomainApexIP)
	}

	if cnameTarget == "localhost" {
		serverConfig := core.GetConfig()
		if len(serverConfig.Server.TLS.Domains) > 0 && serverConfig.Server.TLS.Domains[0] != "" {
			cnameTarget = serverConfig.Server.TLS.Domains[0]
		} else {
			baseURL := serverConfig.ServerBaseURL()
			if parsedURL, parseURLErr := url.Parse(baseURL); parseURLErr == nil && parsedURL.Hostname() != "" {
				cnameTarget = parsedURL.Hostname()
			}
		}
	}

	var recordStatus string
	if domainStatus == "active" {
		recordStatus = "verified"
	} else if tlsStatus == "failed" {
		recordStatus = "misconfigured"
	} else {
		recordStatus = "pending"
	}

	isApex, prefix := extractDomainApexAndPrefix(domain)
	if !isApex {
		return []RequiredDNSRecord{
			{
				Type:     "CNAME",
				Name:     prefix,
				Target:   cnameTarget,
				Status:   recordStatus,
				Required: true,
			},
		}
	}

	targetIP := apexIP
	if targetIP == "" {
		if cnameTarget == "localhost" {
			targetIP = "127.0.0.1"
		} else if net.ParseIP(cnameTarget) != nil {
			targetIP = cnameTarget
		}
	}

	var records []RequiredDNSRecord
	if targetIP != "" && net.ParseIP(targetIP) != nil {
		records = append(records, RequiredDNSRecord{
			Type:     "A",
			Name:     "@",
			Target:   targetIP,
			Status:   recordStatus,
			Required: true,
		})
		records = append(records, RequiredDNSRecord{
			Type:     "ALIAS",
			Name:     "@",
			Target:   cnameTarget,
			Status:   recordStatus,
			Required: false,
			Comment:  "Alternative for DNS providers supporting ALIAS/ANAME (e.g. Cloudflare, Route53)",
		})
	} else {
		records = append(records, RequiredDNSRecord{
			Type:     "ALIAS",
			Name:     "@",
			Target:   cnameTarget,
			Status:   recordStatus,
			Required: true,
			Comment:  "Point apex domain using ALIAS/ANAME or CNAME flattening",
		})
	}

	return records
}

// extractDomainApexAndPrefix inspects a domain name and returns whether it is an apex/root domain along with the subdomain prefix.
func extractDomainApexAndPrefix(domain string) (bool, string) {
	labels := strings.Split(domain, ".")
	if len(labels) <= 2 {
		return true, "@"
	}

	if len(labels) == 3 && isTwoPartCCTLD(labels[1], labels[2]) {
		return true, "@"
	}

	if len(labels) > 3 && isTwoPartCCTLD(labels[len(labels)-2], labels[len(labels)-1]) {
		subdomainLabels := labels[:len(labels)-3]
		return false, strings.Join(subdomainLabels, ".")
	}

	subdomainLabels := labels[:len(labels)-2]
	return false, strings.Join(subdomainLabels, ".")
}

func isTwoPartCCTLD(secondLast, last string) bool {
	combined := secondLast + "." + last
	switch combined {
	case "co.uk", "org.uk", "gov.uk", "me.uk", "net.uk",
		"com.au", "net.au", "org.au", "edu.au", "gov.au",
		"co.nz", "net.nz", "org.nz",
		"co.jp", "ne.jp", "or.jp",
		"com.br", "net.br", "org.br",
		"co.id", "web.id", "or.id", "net.id", "sch.id",
		"com.sg", "edu.sg", "net.sg", "org.sg",
		"co.in", "net.in", "org.in", "gen.in",
		"com.mx", "org.mx", "net.mx", "edu.mx",
		"co.za", "net.za", "org.za",
		"com.tw", "idv.tw", "org.tw", "net.tw",
		"com.ar", "net.ar", "org.ar",
		"com.co", "co.co", "net.co",
		"co.kr", "ne.kr", "or.kr",
		"com.tr", "net.tr", "org.tr",
		"com.ng", "org.ng",
		"co.th", "in.th",
		"com.my", "net.my", "org.my",
		"com.ph", "net.ph", "org.ph",
		"com.vn", "net.vn",
		"com.pk", "net.pk",
		"com.bd", "org.bd",
		"com.sa", "net.sa",
		"com.eg", "net.eg",
		"co.il", "org.il",
		"com.pe", "net.pe",
		"com.ve", "net.ve":
		return true
	default:
		return false
	}
}

// fetchCustomDomainByID retrieves a custom domain by its UUID primary key.
func (controlPlaneHandler *ControlPlaneHandler) fetchCustomDomainByID(ctx context.Context, domainID uuid.UUID) (*CustomDomain, error) {
	const querySQL = `
		SELECT id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at
		FROM function.custom_domains
		WHERE id = $1;
	`

	var customDomain CustomDomain
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(ctx, querySQL, domainID).Scan(
		&customDomain.ID,
		&customDomain.Domain,
		&customDomain.Status,
		&customDomain.TLSEnabled,
		&customDomain.TLSStatus,
		&customDomain.TLSAcmeEmail,
		&customDomain.TLSErrorMessage,
		&customDomain.TLSIssuedAt,
		&customDomain.TLSExpiresAt,
		&customDomain.CreatedAt,
		&customDomain.UpdatedAt,
	)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return nil, ErrCustomDomainNotFound
		}
		return nil, fmt.Errorf("failed to get custom domain: %w", scanErr)
	}

	return &customDomain, nil
}

func isReservedCoreServerDomain(domain string) bool {
	config := core.GetConfig()
	if config.Server.BaseURL != "" {
		if parsedURL, parseErr := url.Parse(config.Server.BaseURL); parseErr == nil && parsedURL.Hostname() != "" {
			if strings.EqualFold(parsedURL.Hostname(), domain) {
				return true
			}
		}
	}
	for _, staticDomain := range config.Server.TLS.Domains {
		cleanedDomain := strings.TrimSpace(strings.ToLower(staticDomain))
		if cleanHost, _, splitErr := net.SplitHostPort(cleanedDomain); splitErr == nil {
			cleanedDomain = cleanHost
		}
		if cleanedDomain != "" && cleanedDomain == domain {
			return true
		}
	}
	return false
}

// handleCreateCustomDomain handles POST /v1/_/function/domains registering a standalone custom domain.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateCustomDomain(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create custom domain request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	var createCustomDomainInput CreateCustomDomainInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createCustomDomainInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	normalizedDomain, validateErr := NormalizeAndValidateDomain(createCustomDomainInput.Domain)
	if validateErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("domain validation failed: %s", validateErr.Error()))
		return
	}

	if isReservedCoreServerDomain(normalizedDomain) {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "domain conflicts with core server host")
		return
	}

	tlsEnabled := true
	if createCustomDomainInput.TLSEnabled != nil {
		tlsEnabled = *createCustomDomainInput.TLSEnabled
	}
	tlsStatus := "pending"
	if !tlsEnabled {
		tlsStatus = "disabled"
	}

	var tlsAcmeEmail *string
	if createCustomDomainInput.TLSAcmeEmail != nil {
		trimmedEmail := strings.TrimSpace(*createCustomDomainInput.TLSAcmeEmail)
		if trimmedEmail != "" {
			if err := validateTLSAcmeEmail(trimmedEmail); err != nil {
				core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
				return
			}
			tlsAcmeEmail = &trimmedEmail
		}
	}

	const insertSQL = `
		INSERT INTO function.custom_domains (id, domain, status, tls_enabled, tls_status, tls_acme_email, created_at, updated_at)
		VALUES (uuidv7(), $1, 'pending', $2, $3, $4, clock_timestamp(), clock_timestamp())
		RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
	`

	var customDomain CustomDomain
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), insertSQL, normalizedDomain, tlsEnabled, tlsStatus, tlsAcmeEmail).Scan(
		&customDomain.ID,
		&customDomain.Domain,
		&customDomain.Status,
		&customDomain.TLSEnabled,
		&customDomain.TLSStatus,
		&customDomain.TLSAcmeEmail,
		&customDomain.TLSErrorMessage,
		&customDomain.TLSIssuedAt,
		&customDomain.TLSExpiresAt,
		&customDomain.CreatedAt,
		&customDomain.UpdatedAt,
	)
	if scanErr != nil {
		var pgError *pgconn.PgError
		if errors.As(scanErr, &pgError) && pgError.Code == "23505" {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, ErrCustomDomainConflict.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	controlPlaneHandler.InvalidateHostValidationCache(customDomain.Domain)
	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewCustomDomainCreatedEvent(customDomain.ID.String(), CustomDomainCreatedEventData(customDomain)),
	)

	customDomain.RequiredDNSRecords = controlPlaneHandler.buildRequiredDNSRecords(customDomain.Domain, customDomain.Status, customDomain.TLSStatus)
	log.Debugf("custom domain %s successfully created", customDomain.ID)
	core.WriteJSONResponse(responseWriter, http.StatusCreated, customDomain)
}

// handleListCustomDomains handles GET /v1/_/function/domains listing all registered custom domains.
func (controlPlaneHandler *ControlPlaneHandler) handleListCustomDomains(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list custom domains request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointRead) {
		return
	}

	const querySQL = `
		SELECT id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at
		FROM function.custom_domains
		ORDER BY created_at ASC;
	`

	rows, queryErr := controlPlaneHandler.kernel.DB().Query(request.Context(), querySQL)
	if queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}
	defer rows.Close()

	customDomains := make([]CustomDomain, 0)
	for rows.Next() {
		var customDomain CustomDomain
		_ = rows.Scan(
			&customDomain.ID,
			&customDomain.Domain,
			&customDomain.Status,
			&customDomain.TLSEnabled,
			&customDomain.TLSStatus,
			&customDomain.TLSAcmeEmail,
			&customDomain.TLSErrorMessage,
			&customDomain.TLSIssuedAt,
			&customDomain.TLSExpiresAt,
			&customDomain.CreatedAt,
			&customDomain.UpdatedAt,
		)
		customDomain.RequiredDNSRecords = controlPlaneHandler.buildRequiredDNSRecords(customDomain.Domain, customDomain.Status, customDomain.TLSStatus)
		customDomains = append(customDomains, customDomain)
	}

	log.Debugf("retrieved %d custom domain(s)", len(customDomains))
	core.WriteJSONResponse(responseWriter, http.StatusOK, ListCustomDomainsResponse{
		Data:  customDomains,
		Total: len(customDomains),
	})
}

// handleGetCustomDomain handles GET /v1/_/function/domains/{domain_id} returning a single custom domain definition with DNS instructions.
func (controlPlaneHandler *ControlPlaneHandler) handleGetCustomDomain(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling get custom domain request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointRead) {
		return
	}

	domainIDString := request.PathValue("domain_id")
	domainID, parseDomainErr := uuid.Parse(domainIDString)
	if parseDomainErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid custom domain ID")
		return
	}

	fetchedCustomDomain, fetchErr := controlPlaneHandler.fetchCustomDomainByID(request.Context(), domainID)
	if fetchErr != nil {
		if errors.Is(fetchErr, ErrCustomDomainNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, ErrCustomDomainNotFound.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchErr.Error())
		return
	}

	fetchedCustomDomain.RequiredDNSRecords = controlPlaneHandler.buildRequiredDNSRecords(fetchedCustomDomain.Domain, fetchedCustomDomain.Status, fetchedCustomDomain.TLSStatus)
	core.WriteJSONResponse(responseWriter, http.StatusOK, fetchedCustomDomain)
}

// handleDeleteCustomDomain handles DELETE /v1/_/function/domains/{domain_id} removing a standalone custom domain.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteCustomDomain(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling delete custom domain request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	domainIDString := request.PathValue("domain_id")
	domainID, parseDomainErr := uuid.Parse(domainIDString)
	if parseDomainErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid custom domain ID")
		return
	}

	var customDomain CustomDomain
	const deleteSQL = `
		DELETE FROM function.custom_domains
		WHERE id = $1
		RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
	`
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), deleteSQL, domainID).Scan(
		&customDomain.ID,
		&customDomain.Domain,
		&customDomain.Status,
		&customDomain.TLSEnabled,
		&customDomain.TLSStatus,
		&customDomain.TLSAcmeEmail,
		&customDomain.TLSErrorMessage,
		&customDomain.TLSIssuedAt,
		&customDomain.TLSExpiresAt,
		&customDomain.CreatedAt,
		&customDomain.UpdatedAt,
	)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, ErrCustomDomainNotFound.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	controlPlaneHandler.InvalidateHostValidationCache(customDomain.Domain)
	if controlPlaneHandler.kernel.Server() != nil && controlPlaneHandler.kernel.Server().DBCertCache() != nil {
		_ = controlPlaneHandler.kernel.Server().DBCertCache().Delete(request.Context(), customDomain.Domain)
	}
	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewCustomDomainDeletedEvent(domainID.String(), CustomDomainDeletedEventData(customDomain)),
	)

	log.Debugf("custom domain %s successfully deleted", domainID)
	responseWriter.WriteHeader(http.StatusNoContent)
}

// handleUpdateCustomDomain handles PATCH /v1/_/function/domains/{domain_id} updating custom domain configuration at runtime.
func (controlPlaneHandler *ControlPlaneHandler) handleUpdateCustomDomain(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling update custom domain request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	domainIDString := request.PathValue("domain_id")
	domainID, parseDomainErr := uuid.Parse(domainIDString)
	if parseDomainErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid custom domain ID")
		return
	}

	var updateCustomDomainInput UpdateCustomDomainInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&updateCustomDomainInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	existingCustomDomain, fetchErr := controlPlaneHandler.fetchCustomDomainByID(request.Context(), domainID)
	if fetchErr != nil {
		if errors.Is(fetchErr, ErrCustomDomainNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, ErrCustomDomainNotFound.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchErr.Error())
		return
	}

	tlsEnabled := existingCustomDomain.TLSEnabled
	tlsStatus := existingCustomDomain.TLSStatus
	tlsErrorMessage := existingCustomDomain.TLSErrorMessage
	tlsAcmeEmail := existingCustomDomain.TLSAcmeEmail

	if updateCustomDomainInput.TLSEnabled != nil {
		tlsEnabled = *updateCustomDomainInput.TLSEnabled
		if !tlsEnabled {
			tlsStatus = "disabled"
			tlsErrorMessage = nil
		} else if existingCustomDomain.TLSStatus == "disabled" {
			tlsStatus = "pending"
			tlsErrorMessage = nil
		}
	}

	if updateCustomDomainInput.TLSAcmeEmail != nil {
		trimmedEmail := strings.TrimSpace(*updateCustomDomainInput.TLSAcmeEmail)
		if trimmedEmail == "" {
			tlsAcmeEmail = nil
		} else {
			if err := validateTLSAcmeEmail(trimmedEmail); err != nil {
				core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, err.Error())
				return
			}
			tlsAcmeEmail = &trimmedEmail
		}
	}

	const updateSQL = `
		UPDATE function.custom_domains
		SET tls_enabled = $1, tls_status = $2, tls_error_message = $3, tls_acme_email = $4, updated_at = clock_timestamp()
		WHERE id = $5
		RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
	`

	var updatedCustomDomain CustomDomain
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), updateSQL, tlsEnabled, tlsStatus, tlsErrorMessage, tlsAcmeEmail, domainID).Scan(
		&updatedCustomDomain.ID,
		&updatedCustomDomain.Domain,
		&updatedCustomDomain.Status,
		&updatedCustomDomain.TLSEnabled,
		&updatedCustomDomain.TLSStatus,
		&updatedCustomDomain.TLSAcmeEmail,
		&updatedCustomDomain.TLSErrorMessage,
		&updatedCustomDomain.TLSIssuedAt,
		&updatedCustomDomain.TLSExpiresAt,
		&updatedCustomDomain.CreatedAt,
		&updatedCustomDomain.UpdatedAt,
	)
	if scanErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	controlPlaneHandler.InvalidateHostValidationCache(updatedCustomDomain.Domain)
	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewCustomDomainUpdatedEvent(domainID.String(), CustomDomainUpdatedEventData(updatedCustomDomain)),
	)

	updatedCustomDomain.RequiredDNSRecords = controlPlaneHandler.buildRequiredDNSRecords(updatedCustomDomain.Domain, updatedCustomDomain.Status, updatedCustomDomain.TLSStatus)
	log.Debugf("custom domain %s successfully updated", domainID)
	core.WriteJSONResponse(responseWriter, http.StatusOK, updatedCustomDomain)
}

// verifyCustomDomainDNS checks live DNS resolution (CNAME or A/AAAA IP) against expected DNS records.
func (controlPlaneHandler *ControlPlaneHandler) verifyCustomDomainDNS(ctx context.Context, customDomain *CustomDomain, expectedRecords []RequiredDNSRecord) (bool, string, string) {
	if len(expectedRecords) == 0 {
		return false, "unconfigured", ""
	}

	expectedTarget := expectedRecords[0].Target
	domain := customDomain.Domain

	// 1. If expected record is CNAME (subdomain)
	if expectedRecords[0].Type == "CNAME" {
		cleanExpected := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(expectedTarget)), ".")
		cname, cnameErr := lookupCNAMEResolver(ctx, domain)
		if cnameErr == nil {
			cleanCNAME := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(cname)), ".")
			if cleanCNAME == cleanExpected {
				return true, cleanCNAME, expectedTarget
			}
		}

		// Also check IP resolution: CNAME flattening or direct target IP alignment
		domainIPs, domainIPErr := lookupIPResolver(ctx, domain)
		if domainIPErr == nil && len(domainIPs) > 0 {
			targetIPs, targetIPErr := lookupIPResolver(ctx, expectedTarget)
			if targetIPErr == nil {
				for _, domainIP := range domainIPs {
					for _, targetIP := range targetIPs {
						if domainIP.Equal(targetIP) {
							return true, domainIP.String(), expectedTarget
						}
					}
				}
			}
			return false, domainIPs[0].String(), expectedTarget
		}

		if cnameErr == nil && cname != "" && strings.TrimSuffix(strings.ToLower(cname), ".") != strings.TrimSuffix(strings.ToLower(domain), ".") {
			return false, strings.TrimSuffix(strings.ToLower(cname), "."), expectedTarget
		}

		return false, "unresolvable", expectedTarget
	}

	// 2. If expected record is A/ALIAS (apex domain)
	domainIPs, domainIPErr := lookupIPResolver(ctx, domain)
	if domainIPErr == nil && len(domainIPs) > 0 {
		expectedIP := net.ParseIP(expectedTarget)
		if expectedIP != nil {
			for _, domainIP := range domainIPs {
				if domainIP.Equal(expectedIP) {
					return true, domainIP.String(), expectedTarget
				}
			}
		}

		// Also check any ALIAS record among expectedRecords
		for _, rec := range expectedRecords {
			if rec.Type == "ALIAS" && rec.Target != "" {
				aliasIPs, aliasIPErr := lookupIPResolver(ctx, rec.Target)
				if aliasIPErr == nil {
					for _, domainIP := range domainIPs {
						for _, aliasIP := range aliasIPs {
							if domainIP.Equal(aliasIP) {
								return true, domainIP.String(), expectedTarget
							}
						}
					}
				}
			}
		}

		return false, domainIPs[0].String(), expectedTarget
	}

	cname, cnameErr := lookupCNAMEResolver(ctx, domain)
	if cnameErr == nil {
		cleanCNAME := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(cname)), ".")
		for _, rec := range expectedRecords {
			if rec.Type == "ALIAS" && cleanCNAME == strings.TrimSuffix(strings.ToLower(rec.Target), ".") {
				return true, cleanCNAME, expectedTarget
			}
		}
		if cleanCNAME != "" && cleanCNAME != strings.TrimSuffix(strings.ToLower(domain), ".") {
			return false, cleanCNAME, expectedTarget
		}
	}

	return false, "unresolvable", expectedTarget
}

// isCertValidAndNotExpiringSoon checks whether certData contains a valid X.509 certificate
// that is unexpired and has at least customDomainCertRenewWindow duration remaining before expiration.
func isCertValidAndNotExpiringSoon(certData []byte) bool {
	remaining := certData
	for {
		var block *pem.Block
		block, remaining = pem.Decode(remaining)
		if block == nil {
			return false
		}
		if block.Type == "CERTIFICATE" {
			certificate, parseErr := x509.ParseCertificate(block.Bytes)
			if parseErr != nil {
				return false
			}
			return time.Now().Before(certificate.NotAfter.Add(-customDomainCertRenewWindow))
		}
	}
}

// handleVerifyCustomDomain handles POST /v1/_/function/domains/{domain_id}/verify triggering DNS verification and provisioning check.
func (controlPlaneHandler *ControlPlaneHandler) handleVerifyCustomDomain(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling verify custom domain request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	domainIDString := request.PathValue("domain_id")
	domainID, parseDomainErr := uuid.Parse(domainIDString)
	if parseDomainErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid custom domain ID")
		return
	}

	customDomain, fetchErr := controlPlaneHandler.fetchCustomDomainByID(request.Context(), domainID)
	if fetchErr != nil {
		if errors.Is(fetchErr, ErrCustomDomainNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, ErrCustomDomainNotFound.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchErr.Error())
		return
	}

	expectedRecords := controlPlaneHandler.buildRequiredDNSRecords(customDomain.Domain, customDomain.Status, customDomain.TLSStatus)
	matched, actualResolution, expectedTarget := controlPlaneHandler.verifyCustomDomainDNS(request.Context(), customDomain, expectedRecords)

	if !matched {
		diagnosticMessage := fmt.Sprintf("DNS verification failed: currently resolves to: %s, expected: %s", actualResolution, expectedTarget)
		failedTLSStatus := "failed"
		if !customDomain.TLSEnabled {
			failedTLSStatus = "disabled"
		}
		const updateFailedSQL = `
			UPDATE function.custom_domains
			SET status = 'pending', tls_status = $1, tls_error_message = $2, updated_at = clock_timestamp()
			WHERE id = $3
			RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
		`
		var failedCustomDomain CustomDomain
		scanErr := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), updateFailedSQL, failedTLSStatus, diagnosticMessage, domainID).Scan(
			&failedCustomDomain.ID,
			&failedCustomDomain.Domain,
			&failedCustomDomain.Status,
			&failedCustomDomain.TLSEnabled,
			&failedCustomDomain.TLSStatus,
			&failedCustomDomain.TLSAcmeEmail,
			&failedCustomDomain.TLSErrorMessage,
			&failedCustomDomain.TLSIssuedAt,
			&failedCustomDomain.TLSExpiresAt,
			&failedCustomDomain.CreatedAt,
			&failedCustomDomain.UpdatedAt,
		)
		if scanErr != nil {
			core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
			return
		}

		controlPlaneHandler.InvalidateHostValidationCache(failedCustomDomain.Domain)
		failedCustomDomain.RequiredDNSRecords = controlPlaneHandler.buildRequiredDNSRecords(failedCustomDomain.Domain, failedCustomDomain.Status, failedCustomDomain.TLSStatus)
		core.WriteJSONResponse(responseWriter, http.StatusOK, failedCustomDomain)
		return
	}

	var (
		nextTLSStatus      = "disabled"
		tlsErrorMessage    *string
		shouldProvisionTLS = false
	)

	if customDomain.TLSEnabled {
		var hasExistingCert bool
		if dbCertCache := controlPlaneHandler.kernel.Server().DBCertCache(); dbCertCache != nil {
			if certData, certErr := dbCertCache.Get(request.Context(), customDomain.Domain); certErr == nil && len(certData) > 0 {
				hasExistingCert = isCertValidAndNotExpiringSoon(certData)
			}
		}

		if hasExistingCert {
			nextTLSStatus = "active"
			shouldProvisionTLS = false
		} else if !controlPlaneHandler.kernel.Server().TLSEnabled() || controlPlaneHandler.kernel.Server().DBCertCache() == nil {
			nextTLSStatus = "failed"
			msg := "server TLS is not enabled on this gateway"
			tlsErrorMessage = &msg
			shouldProvisionTLS = false
		} else {
			nextTLSStatus = "provisioning"
			shouldProvisionTLS = true
		}
	}

	const updateSuccessSQL = `
		UPDATE function.custom_domains
		SET status = 'active', tls_status = $1, tls_error_message = $2, updated_at = clock_timestamp()
		WHERE id = $3
		RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
	`
	var verifiedCustomDomain CustomDomain
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), updateSuccessSQL, nextTLSStatus, tlsErrorMessage, domainID).Scan(
		&verifiedCustomDomain.ID,
		&verifiedCustomDomain.Domain,
		&verifiedCustomDomain.Status,
		&verifiedCustomDomain.TLSEnabled,
		&verifiedCustomDomain.TLSStatus,
		&verifiedCustomDomain.TLSAcmeEmail,
		&verifiedCustomDomain.TLSErrorMessage,
		&verifiedCustomDomain.TLSIssuedAt,
		&verifiedCustomDomain.TLSExpiresAt,
		&verifiedCustomDomain.CreatedAt,
		&verifiedCustomDomain.UpdatedAt,
	)
	if scanErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	controlPlaneHandler.InvalidateHostValidationCache(verifiedCustomDomain.Domain)
	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewCustomDomainUpdatedEvent(domainID.String(), CustomDomainUpdatedEventData(verifiedCustomDomain)),
	)

	if shouldProvisionTLS {
		acmeEmail := controlPlaneHandler.resolveCustomDomainACMEEmail(&verifiedCustomDomain)
		controlPlaneHandler.triggerTLSProvisioning(verifiedCustomDomain.Domain, acmeEmail) //nolint:contextcheck
	}

	verifiedCustomDomain.RequiredDNSRecords = controlPlaneHandler.buildRequiredDNSRecords(verifiedCustomDomain.Domain, verifiedCustomDomain.Status, verifiedCustomDomain.TLSStatus)
	core.WriteJSONResponse(responseWriter, http.StatusOK, verifiedCustomDomain)
}

// resolveCustomDomainACMEEmail resolves the ACME email for a custom domain:
// 1. Domain-specific override (customDomain.TLSAcmeEmail)
// 2. Function service default (service.configManager.Get().DefaultTLSAcmeEmail)
// 3. Core fallback (core.GetConfig().Server.TLS.AcmeEmail)
func (controlPlaneHandler *ControlPlaneHandler) resolveCustomDomainACMEEmail(customDomain *CustomDomain) string {
	if customDomain.TLSAcmeEmail != nil && strings.TrimSpace(*customDomain.TLSAcmeEmail) != "" {
		return strings.TrimSpace(*customDomain.TLSAcmeEmail)
	}
	defaultEmail := strings.TrimSpace(controlPlaneHandler.configManager.Get().DefaultTLSAcmeEmail)
	if defaultEmail != "" {
		return defaultEmail
	}
	return strings.TrimSpace(core.GetConfig().Server.TLS.AcmeEmail)
}

// triggerTLSProvisioning asynchronously initiates automated TLS certificate issuance via ACME.
func (controlPlaneHandler *ControlPlaneHandler) triggerTLSProvisioning(domain string, email string) { //nolint:contextcheck
	server := controlPlaneHandler.kernel.Server()

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		if err := tlsProvisionerFunc(ctx, server, domain, email); err != nil {
			log.Warnf("failed to auto-provision TLS certificate for custom domain %s: %v", domain, err)
			const updateErrSQL = `
				UPDATE function.custom_domains
				SET tls_status = 'failed', tls_error_message = $1, updated_at = clock_timestamp()
				WHERE domain = $2 AND tls_enabled = true AND tls_status = 'provisioning'
				RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
			`
			var failedCustomDomain CustomDomain
			if scanErr := controlPlaneHandler.kernel.DB().QueryRow(ctx, updateErrSQL, err.Error(), domain).Scan(
				&failedCustomDomain.ID,
				&failedCustomDomain.Domain,
				&failedCustomDomain.Status,
				&failedCustomDomain.TLSEnabled,
				&failedCustomDomain.TLSStatus,
				&failedCustomDomain.TLSAcmeEmail,
				&failedCustomDomain.TLSErrorMessage,
				&failedCustomDomain.TLSIssuedAt,
				&failedCustomDomain.TLSExpiresAt,
				&failedCustomDomain.CreatedAt,
				&failedCustomDomain.UpdatedAt,
			); scanErr == nil {
				controlPlaneHandler.InvalidateHostValidationCache(domain)
				controlPlaneHandler.kernel.EventBus().Publish(
					ctx,
					NewCustomDomainUpdatedEvent(failedCustomDomain.ID.String(), CustomDomainUpdatedEventData(failedCustomDomain)),
				)
			}
		} else {
			const updateActiveSQL = `
				UPDATE function.custom_domains
				SET tls_status = 'active', tls_error_message = NULL, updated_at = clock_timestamp()
				WHERE domain = $1 AND tls_enabled = true AND tls_status = 'provisioning'
				RETURNING id, domain, status, tls_enabled, tls_status, tls_acme_email, tls_error_message, tls_issued_at, tls_expires_at, created_at, updated_at;
			`
			var activeCustomDomain CustomDomain
			if scanErr := controlPlaneHandler.kernel.DB().QueryRow(ctx, updateActiveSQL, domain).Scan(
				&activeCustomDomain.ID,
				&activeCustomDomain.Domain,
				&activeCustomDomain.Status,
				&activeCustomDomain.TLSEnabled,
				&activeCustomDomain.TLSStatus,
				&activeCustomDomain.TLSAcmeEmail,
				&activeCustomDomain.TLSErrorMessage,
				&activeCustomDomain.TLSIssuedAt,
				&activeCustomDomain.TLSExpiresAt,
				&activeCustomDomain.CreatedAt,
				&activeCustomDomain.UpdatedAt,
			); scanErr == nil {
				controlPlaneHandler.InvalidateHostValidationCache(domain)
				controlPlaneHandler.kernel.EventBus().Publish(
					ctx,
					NewCustomDomainUpdatedEvent(activeCustomDomain.ID.String(), CustomDomainUpdatedEventData(activeCustomDomain)),
				)
			}
		}
	}()
}

// handleCreateCustomDomainRoute handles POST /v1/_/function/endpoints/{endpoint_id}/domains attaching an endpoint to a domain route.
func (controlPlaneHandler *ControlPlaneHandler) handleCreateCustomDomainRoute(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling create endpoint domain route request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	endpointIDString := request.PathValue("endpoint_id")
	endpointID, parseEndpointErr := uuid.Parse(endpointIDString)
	if parseEndpointErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	var createCustomDomainRouteInput CreateCustomDomainRouteInput
	if decodeErr := json.NewDecoder(request.Body).Decode(&createCustomDomainRouteInput); decodeErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid request body")
		return
	}

	if createCustomDomainRouteInput.CustomDomainID == uuid.Nil() {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "custom_domain_id is required")
		return
	}

	if _, fetchEndpointErr := controlPlaneHandler.fetchEndpointByID(request.Context(), endpointID); fetchEndpointErr != nil {
		if errors.Is(fetchEndpointErr, ErrEndpointNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fetchEndpointErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchEndpointErr.Error())
		return
	}

	customDomain, fetchDomainErr := controlPlaneHandler.fetchCustomDomainByID(request.Context(), createCustomDomainRouteInput.CustomDomainID)
	if fetchDomainErr != nil {
		if errors.Is(fetchDomainErr, ErrCustomDomainNotFound) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, fetchDomainErr.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, fetchDomainErr.Error())
		return
	}

	if customDomain.Status != "active" {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, ErrCustomDomainNotVerified.Error())
		return
	}

	rawPrefix := "/"
	if createCustomDomainRouteInput.PathPrefix != nil {
		rawPrefix = *createCustomDomainRouteInput.PathPrefix
	}

	normalizedPrefix, validatePrefixErr := NormalizeAndValidatePathPrefix(rawPrefix)
	if validatePrefixErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, fmt.Sprintf("path prefix validation failed: %s", validatePrefixErr.Error()))
		return
	}

	const insertSQL = `
		INSERT INTO function.custom_domain_routes (id, custom_domain_id, endpoint_id, path_prefix, created_at, updated_at)
		VALUES (uuidv7(), $1, $2, $3, clock_timestamp(), clock_timestamp())
		RETURNING id, custom_domain_id, endpoint_id, path_prefix, created_at, updated_at;
	`

	var customDomainRoute CustomDomainRoute
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(
		request.Context(),
		insertSQL,
		createCustomDomainRouteInput.CustomDomainID,
		endpointID,
		normalizedPrefix,
	).Scan(
		&customDomainRoute.ID,
		&customDomainRoute.CustomDomainID,
		&customDomainRoute.EndpointID,
		&customDomainRoute.PathPrefix,
		&customDomainRoute.CreatedAt,
		&customDomainRoute.UpdatedAt,
	)
	if scanErr != nil {
		var pgError *pgconn.PgError
		if errors.As(scanErr, &pgError) && pgError.Code == "23505" {
			core.WriteErrorResponse(responseWriter, request, http.StatusConflict, ErrCustomDomainRouteConflict.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	customDomainRoute.Domain = customDomain.Domain

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewCustomDomainRouteCreatedEvent(customDomainRoute.ID.String(), CustomDomainRouteCreatedEventData(customDomainRoute)),
	)

	log.Debugf("custom domain route %s successfully created", customDomainRoute.ID)
	core.WriteJSONResponse(responseWriter, http.StatusCreated, customDomainRoute)
}

// handleListCustomDomainRoutes handles GET /v1/_/function/endpoints/{endpoint_id}/domains listing all route mappings for an endpoint.
func (controlPlaneHandler *ControlPlaneHandler) handleListCustomDomainRoutes(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling list endpoint domain routes request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointRead) {
		return
	}

	endpointIDString := request.PathValue("endpoint_id")
	endpointID, parseEndpointErr := uuid.Parse(endpointIDString)
	if parseEndpointErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	const querySQL = `
		SELECT r.id, r.custom_domain_id, d.domain, r.endpoint_id, r.path_prefix, r.created_at, r.updated_at
		FROM function.custom_domain_routes r
		JOIN function.custom_domains d ON d.id = r.custom_domain_id
		WHERE r.endpoint_id = $1
		ORDER BY r.created_at ASC;
	`

	rows, queryErr := controlPlaneHandler.kernel.DB().Query(request.Context(), querySQL, endpointID)
	if queryErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, queryErr.Error())
		return
	}
	defer rows.Close()

	routes := make([]CustomDomainRoute, 0)
	for rows.Next() {
		var customDomainRoute CustomDomainRoute
		_ = rows.Scan(
			&customDomainRoute.ID,
			&customDomainRoute.CustomDomainID,
			&customDomainRoute.Domain,
			&customDomainRoute.EndpointID,
			&customDomainRoute.PathPrefix,
			&customDomainRoute.CreatedAt,
			&customDomainRoute.UpdatedAt,
		)
		routes = append(routes, customDomainRoute)
	}

	log.Debugf("retrieved %d custom domain route(s)", len(routes))
	core.WriteJSONResponse(responseWriter, http.StatusOK, ListCustomDomainRoutesResponse{
		Data:  routes,
		Total: len(routes),
	})
}

// handleDeleteCustomDomainRoute handles DELETE /v1/_/function/endpoints/{endpoint_id}/domains/{route_id} unbinding an endpoint from a domain route.
func (controlPlaneHandler *ControlPlaneHandler) handleDeleteCustomDomainRoute(responseWriter http.ResponseWriter, request *http.Request) {
	log.Trace("handling delete endpoint domain route request")
	if !controlPlaneHandler.kernel.ServiceAccountManager().RequireScope(responseWriter, request, core.ScopeFunctionEndpointWrite) {
		return
	}

	endpointIDString := request.PathValue("endpoint_id")
	endpointID, parseEndpointErr := uuid.Parse(endpointIDString)
	if parseEndpointErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid endpoint ID")
		return
	}

	routeIDString := request.PathValue("route_id")
	routeID, parseRouteErr := uuid.Parse(routeIDString)
	if parseRouteErr != nil {
		core.WriteErrorResponse(responseWriter, request, http.StatusBadRequest, "Invalid custom domain route ID")
		return
	}

	var customDomainRoute CustomDomainRoute
	const deleteSQL = `
		DELETE FROM function.custom_domain_routes
		WHERE id = $1 AND endpoint_id = $2
		RETURNING id, custom_domain_id, endpoint_id, path_prefix, created_at, updated_at;
	`
	scanErr := controlPlaneHandler.kernel.DB().QueryRow(request.Context(), deleteSQL, routeID, endpointID).Scan(
		&customDomainRoute.ID,
		&customDomainRoute.CustomDomainID,
		&customDomainRoute.EndpointID,
		&customDomainRoute.PathPrefix,
		&customDomainRoute.CreatedAt,
		&customDomainRoute.UpdatedAt,
	)
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			core.WriteErrorResponse(responseWriter, request, http.StatusNotFound, ErrCustomDomainRouteNotFound.Error())
			return
		}
		core.WriteErrorResponse(responseWriter, request, http.StatusInternalServerError, scanErr.Error())
		return
	}

	controlPlaneHandler.kernel.EventBus().Publish(
		request.Context(),
		NewCustomDomainRouteDeletedEvent(routeID.String(), CustomDomainRouteDeletedEventData(customDomainRoute)),
	)

	log.Debugf("custom domain route %s successfully deleted", routeID)
	responseWriter.WriteHeader(http.StatusNoContent)
}
