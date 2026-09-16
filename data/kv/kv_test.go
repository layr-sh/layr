package kv

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestKVExtractClientIPUnit(t *testing.T) {
	ctx := context.Background()
	// 1. X-Forwarded-For single and multiple IPs with whitespace
	singleForwardedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	singleForwardedRequest.Header.Set("X-Forwarded-For", "203.0.113.195")
	if clientIP := ExtractClientIP(singleForwardedRequest); clientIP != "203.0.113.195" {
		t.Fatalf("expected 203.0.113.195 from X-Forwarded-For, got %s", clientIP)
	}

	multiForwardedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	multiForwardedRequest.Header.Set("X-Forwarded-For", "  203.0.113.195  , 70.41.3.18, 192.0.2.1 ")
	if clientIP := ExtractClientIP(multiForwardedRequest); clientIP != "203.0.113.195" {
		t.Fatalf("expected trimmed first IP 203.0.113.195, got %s", clientIP)
	}

	// 2. X-Forwarded-For with empty first part falls through to X-Real-IP or RemoteAddr
	emptyForwardedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	emptyForwardedRequest.Header.Set("X-Forwarded-For", "   , 70.41.3.18")
	emptyForwardedRequest.Header.Set("X-Real-IP", "198.51.100.2")
	if clientIP := ExtractClientIP(emptyForwardedRequest); clientIP != "198.51.100.2" {
		t.Fatalf("expected fallback to X-Real-IP, got %s", clientIP)
	}

	onlyWhitespaceForwardedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	onlyWhitespaceForwardedRequest.Header.Set("X-Forwarded-For", "   ")
	onlyWhitespaceForwardedRequest.RemoteAddr = "127.0.0.1:8080"
	if clientIP := ExtractClientIP(onlyWhitespaceForwardedRequest); clientIP != "127.0.0.1" {
		t.Fatalf("expected fallback to RemoteAddr 127.0.0.1, got %s", clientIP)
	}

	// 3. X-Real-IP header
	realIPRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	realIPRequest.Header.Set("X-Real-IP", "  198.51.100.1  ")
	if clientIP := ExtractClientIP(realIPRequest); clientIP != "198.51.100.1" {
		t.Fatalf("expected 198.51.100.1 from X-Real-IP, got %s", clientIP)
	}

	// 4. RemoteAddr with IPv4 port
	remoteAddrWithPortRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	remoteAddrWithPortRequest.RemoteAddr = "192.0.2.1:12345"
	if clientIP := ExtractClientIP(remoteAddrWithPortRequest); clientIP != "192.0.2.1" {
		t.Fatalf("expected 192.0.2.1 from RemoteAddr, got %s", clientIP)
	}

	// 5. RemoteAddr with IPv6 port
	remoteAddrIPv6Request := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	remoteAddrIPv6Request.RemoteAddr = "[2001:db8::1]:8080"
	if clientIP := ExtractClientIP(remoteAddrIPv6Request); clientIP != "2001:db8::1" {
		t.Fatalf("expected 2001:db8::1 from IPv6 RemoteAddr, got %s", clientIP)
	}

	// 6. RemoteAddr without port (e.g. unix socket or raw string)
	remoteAddrWithoutPortRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	remoteAddrWithoutPortRequest.RemoteAddr = "unix_socket_addr"
	if clientIP := ExtractClientIP(remoteAddrWithoutPortRequest); clientIP != "unix_socket_addr" {
		t.Fatalf("expected unix_socket_addr from RemoteAddr, got %s", clientIP)
	}
}

func TestKVComputeVisitorHashUnit(t *testing.T) {
	referenceTime := time.Unix(1700000000, 0)

	// 1. Default salt when empty string passed
	hashWithDefaultSalt := ComputeVisitorHash("1.2.3.4", "Mozilla/5.0", referenceTime, "")
	if len(hashWithDefaultSalt) != 64 {
		t.Fatalf("expected 64 character hex string, got %d", len(hashWithDefaultSalt))
	}

	// Explicit DefaultSaltSecret produces exact same hash
	hashWithExplicitDefault := ComputeVisitorHash("1.2.3.4", "Mozilla/5.0", referenceTime, DefaultSaltSecret)
	if hashWithDefaultSalt != hashWithExplicitDefault {
		t.Fatal("expected empty salt to use DefaultSaltSecret")
	}

	// 2. Custom salt produces different hash
	hashWithCustomSalt := ComputeVisitorHash("1.2.3.4", "Mozilla/5.0", referenceTime, "custom-salt")
	if hashWithDefaultSalt == hashWithCustomSalt {
		t.Fatal("expected different hash when using custom salt")
	}

	// 3. Different client IP produces different hash
	hashDifferentIP := ComputeVisitorHash("5.6.7.8", "Mozilla/5.0", referenceTime, "custom-salt")
	if hashWithCustomSalt == hashDifferentIP {
		t.Fatal("expected different hash for different client IP")
	}

	// 4. Different user agent produces different hash
	hashDifferentAgent := ComputeVisitorHash("1.2.3.4", "Curl/8.0", referenceTime, "custom-salt")
	if hashWithCustomSalt == hashDifferentAgent {
		t.Fatal("expected different hash for different user agent")
	}

	// 5. Same bucket within 24 hours produces identical hash
	bucketStart := time.Unix((1700000000/(24*3600))*(24*3600), 0)
	hashBucketStart := ComputeVisitorHash("1.2.3.4", "Mozilla/5.0", bucketStart, "custom-salt")
	hashBucketMid := ComputeVisitorHash("1.2.3.4", "Mozilla/5.0", bucketStart.Add(12*time.Hour), "custom-salt")
	hashBucketEnd := ComputeVisitorHash("1.2.3.4", "Mozilla/5.0", bucketStart.Add(23*time.Hour+59*time.Minute), "custom-salt")
	if hashBucketStart != hashBucketMid || hashBucketStart != hashBucketEnd {
		t.Fatal("expected identical hash within the same 24 hour bucket")
	}

	// 6. Next 24h bucket rotates the hash
	hashNextDay := ComputeVisitorHash("1.2.3.4", "Mozilla/5.0", bucketStart.Add(24*time.Hour+time.Second), "custom-salt")
	if hashBucketStart == hashNextDay {
		t.Fatal("expected rotated hash on the next day bucket")
	}

	// 7. Unix epoch edge case
	epochHash := ComputeVisitorHash("", "", time.Unix(0, 0), "")
	if len(epochHash) != 64 {
		t.Fatalf("expected valid 64 char hash for epoch zero, got %s", epochHash)
	}
}

func TestKVExtractAuthContextUnit(t *testing.T) {
	ctx := context.Background()
	// 1. Authenticated caller with role and subject
	authenticatedRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	authenticatedRequest.Header.Set("X-JWT-Sub", "usr_1001")
	authenticatedRequest.Header.Set("X-JWT-Role", "editor")
	authenticatedRequest.Header.Set("User-Agent", "Mozilla/5.0")
	authenticatedRequest.RemoteAddr = "10.0.0.1:8080"

	authContext := ExtractAuthContext(authenticatedRequest, "test-salt")
	if authContext.Subject != "usr_1001" || authContext.Role != "editor" || authContext.VisitorHash != "" {
		t.Fatalf("unexpected auth context for authenticated caller: %+v", authContext)
	}

	// 2. Authenticated caller with empty role defaults to "authenticated"
	emptyRoleRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	emptyRoleRequest.Header.Set("X-JWT-Sub", "usr_2002")
	emptyRoleAuthContext := ExtractAuthContext(emptyRoleRequest, "test-salt")
	if emptyRoleAuthContext.Subject != "usr_2002" || emptyRoleAuthContext.Role != "authenticated" || emptyRoleAuthContext.VisitorHash != "" {
		t.Fatalf("unexpected empty role context: %+v", emptyRoleAuthContext)
	}

	// 3. Authenticated caller with role and empty subject does not fall through to anon
	serviceRoleRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	serviceRoleRequest.Header.Set("X-JWT-Role", "service_role")
	serviceRoleAuthContext := ExtractAuthContext(serviceRoleRequest, "test-salt")
	if serviceRoleAuthContext.Subject != "" || serviceRoleAuthContext.Role != "service_role" || serviceRoleAuthContext.VisitorHash != "" {
		t.Fatalf("unexpected service_role context: %+v", serviceRoleAuthContext)
	}

	// 4. Anonymous caller with explicit 'anon' role generates visitor hash
	anonymousExplicitRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	anonymousExplicitRequest.Header.Set("X-JWT-Role", "anon")
	anonymousExplicitRequest.Header.Set("User-Agent", "Mozilla/5.0")
	anonymousExplicitRequest.RemoteAddr = "10.0.0.1:8080"
	anonAuthContext := ExtractAuthContext(anonymousExplicitRequest, "test-salt")
	if anonAuthContext.Subject != "" || anonAuthContext.Role != "anon" || len(anonAuthContext.VisitorHash) != 64 {
		t.Fatalf("unexpected explicit anon context: %+v", anonAuthContext)
	}

	// 5. Anonymous caller with no claims defaults role to "anon" and generates visitor hash
	anonymousDefaultRequest := httptest.NewRequestWithContext(ctx, http.MethodGet, "/", nil)
	anonymousDefaultRequest.Header.Set("User-Agent", "LayrClient/1.0")
	anonymousDefaultRequest.RemoteAddr = "192.168.1.50:9000"
	anonymousAuthContext := ExtractAuthContext(anonymousDefaultRequest, "test-salt")
	if anonymousAuthContext.Subject != "" || anonymousAuthContext.Role != "anon" || len(anonymousAuthContext.VisitorHash) != 64 {
		t.Fatalf("unexpected anonymous context: %+v", anonymousAuthContext)
	}
}

func TestKVBuildInternalKeyUnit(t *testing.T) {
	// 1. Authenticated with explicit role and subject
	editorAuthContext := AuthContext{
		Role:    "editor",
		Subject: "usr_1001",
	}
	editorInternalKey := BuildInternalKey(editorAuthContext, "rest:active_users")
	if editorInternalKey != "cache:editor:usr_1001:rest:active_users" {
		t.Fatalf("unexpected internal key for editor: %s", editorInternalKey)
	}

	// 2. Authenticated with empty role defaults to "authenticated"
	emptyRoleAuthContext := AuthContext{
		Subject: "usr_2002",
	}
	emptyRoleInternalKey := BuildInternalKey(emptyRoleAuthContext, "graphql:feed")
	if emptyRoleInternalKey != "cache:authenticated:usr_2002:graphql:feed" {
		t.Fatalf("unexpected internal key for empty role: %s", emptyRoleInternalKey)
	}

	// 3. Authenticated with role and empty subject formats as <role>::<key> for KV and cache:<role>::<key> for cache
	serviceRoleAuthContext := AuthContext{
		Role:    "service_role",
		Subject: "",
	}
	serviceInternalKey := BuildInternalKey(serviceRoleAuthContext, "my_config")
	if serviceInternalKey != "service_role::my_config" {
		t.Fatalf("unexpected internal key for service_role KV: %s", serviceInternalKey)
	}
	serviceCacheKey := BuildInternalKey(serviceRoleAuthContext, "rest:table_data")
	if serviceCacheKey != "cache:service_role::rest:table_data" {
		t.Fatalf("unexpected internal key for service_role cache: %s", serviceCacheKey)
	}

	// 4. Anonymous caller with visitor hash for query cache
	anonymousAuthContext := AuthContext{
		Role:        "anon",
		VisitorHash: "a1b2c3d4e5f6",
	}
	anonymousInternalKey := BuildInternalKey(anonymousAuthContext, "rest:homepage")
	if anonymousInternalKey != "cache:anon:a1b2c3d4e5f6:rest:homepage" {
		t.Fatalf("unexpected internal key for anonymous: %s", anonymousInternalKey)
	}

	// 5. Developer ephemeral KV entries with subject do not have a prefix
	kvSubjectInternalKey := BuildInternalKey(editorAuthContext, "session_cart")
	if kvSubjectInternalKey != "editor:usr_1001:session_cart" {
		t.Fatalf("unexpected internal key for ephemeral KV with subject: %s", kvSubjectInternalKey)
	}

	// 6. Developer ephemeral KV entries for anonymous visitor do not have a prefix
	kvAnonInternalKey := BuildInternalKey(anonymousAuthContext, "session_cart")
	if kvAnonInternalKey != "anon:a1b2c3d4e5f6:session_cart" {
		t.Fatalf("unexpected internal key for ephemeral KV with anon visitor: %s", kvAnonInternalKey)
	}

	// 7. Anonymous caller with empty role defaults to "anon"
	emptyAnonAuthContext := AuthContext{
		VisitorHash: "anon123",
	}
	if emptyAnonKey := BuildInternalKey(emptyAnonAuthContext, "cart"); emptyAnonKey != "anon:anon123:cart" {
		t.Fatalf("unexpected internal key for empty anon role: %s", emptyAnonKey)
	}
}

func TestKVBuildRESTQueryKeyUnit(t *testing.T) {
	// 1. Custom suffix takes immediate precedence
	customKey := BuildRESTQueryKey("public", "products", nil, nil, nil, nil, 0, 0, false, "featured_items")
	if customKey != "rest:featured_items" {
		t.Fatalf("expected rest:featured_items, got %s", customKey)
	}

	// 2. Hashed key with all options populated
	fields := []string{"id", "title", "price"}
	embedded := []string{"category:id,name"}
	filters := []string{"status=eq.active", "price=lte.100"}
	order := []string{"created_at.desc"}

	firstHashKey := BuildRESTQueryKey("public", "products", fields, embedded, filters, order, 25, 50, true, "")
	if !strings.HasPrefix(firstHashKey, "rest:") || len(firstHashKey) != 5+64 {
		t.Fatalf("expected rest:<sha256>, got %s", firstHashKey)
	}

	// 3. Deterministic order sorting produces identical hash
	scrambledFields := []string{"price", "id", "title"}
	scrambledFilters := []string{"price=lte.100", "status=eq.active"}
	secondHashKey := BuildRESTQueryKey("public", "products", scrambledFields, embedded, scrambledFilters, order, 25, 50, true, "")
	if firstHashKey != secondHashKey {
		t.Fatalf("expected deterministic hash invariant, got %s vs %s", firstHashKey, secondHashKey)
	}

	// 4. Altered pagination produces different hash
	differentLimitKey := BuildRESTQueryKey("public", "products", fields, embedded, filters, order, 10, 50, true, "")
	if firstHashKey == differentLimitKey {
		t.Fatal("expected different hash when limit differs")
	}

	differentCountKey := BuildRESTQueryKey("public", "products", fields, embedded, filters, order, 25, 50, false, "")
	if firstHashKey == differentCountKey {
		t.Fatal("expected different hash when countExact differs")
	}

	// 5. Versioned query keys
	versionZeroKey := BuildRESTQueryKeyWithVersion("public", "products", 0, fields, embedded, filters, order, 25, 50, true, "")
	if versionZeroKey != firstHashKey {
		t.Fatalf("expected version 0 key to match unversioned key, got %s vs %s", versionZeroKey, firstHashKey)
	}

	versionOneKey := BuildRESTQueryKeyWithVersion("public", "products", 1, fields, embedded, filters, order, 25, 50, true, "")
	if versionOneKey == firstHashKey {
		t.Fatal("expected different hash when tableVersion differs")
	}
}

func TestKVBuildGraphQLQueryKeyUnit(t *testing.T) {
	// 1. Custom suffix takes immediate precedence
	customKey := BuildGraphQLQueryKey("query { users { id } }", nil, "all_users")
	if customKey != "graphql:all_users" {
		t.Fatalf("expected graphql:all_users, got %s", customKey)
	}

	// 2. Hashed query key with variables
	query := "query GetUser($id: ID!) { user(id: $id) { name } }"
	variables := map[string]any{"id": "usr_99"}

	firstHashKey := BuildGraphQLQueryKey(query, variables, "")
	if !strings.HasPrefix(firstHashKey, "graphql:") || len(firstHashKey) != 8+64 {
		t.Fatalf("expected graphql:<sha256>, got %s", firstHashKey)
	}

	// Query with whitespace padding produces identical hash
	paddedQuery := "  \n  query GetUser($id: ID!) { user(id: $id) { name } }  \t "
	secondHashKey := BuildGraphQLQueryKey(paddedQuery, variables, "")
	if firstHashKey != secondHashKey {
		t.Fatalf("expected trimmed query to produce same hash, got %s vs %s", firstHashKey, secondHashKey)
	}

	// Altered variable produces different hash
	differentVarKey := BuildGraphQLQueryKey(query, map[string]any{"id": "usr_100"}, "")
	if firstHashKey == differentVarKey {
		t.Fatal("expected different hash when variables differ")
	}

	// Versioned GraphQL query keys
	versionOneGraphQLKey := BuildGraphQLQueryKeyWithVersion(query, variables, "", 1)
	if versionOneGraphQLKey == firstHashKey {
		t.Fatal("expected different hash when GraphQL tableVersion differs")
	}
}
