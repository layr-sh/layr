package namingclarity

import (
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"testing"

	"github.com/golangci/plugin-module-register/register"
	"golang.org/x/tools/go/analysis"
)

func TestNamingclarityCanonicalVariableNameUnit(t *testing.T) {
	tests := []struct {
		concreteName string
		want         string
	}{
		{"ResponseWriter", "responseWriter"},
		{"Request", "request"},
		{"RangeStmt", "rangeStmt"},
		{"ValueSpec", "valueSpec"},
		{"EmailDispatcher", "emailDispatcher"},
		{"SMSDispatcher", "smsDispatcher"},
		{"CryptoKeyManager", "cryptoKeyManager"},
		{"UniversalClient", "universalClient"},
		{"ClusterClient", "clusterClient"},
		{"SlogHandler", "slogHandler"},
		{"Kernel", "kernel"},
		{"DatabasePool", "db"},
		{"OIDCIDTokenClaims", "oidcIDTokenClaims"},
		{"Context", "ctx"},
		{"CancelFunc", "cancel"},
		{"CommandTag", "result"},
		{"error", "err"},
		{"Error", "err"},
		{"Conn", "connection"},
		{"Addr", "address"},
		{"TCPAddr", "tcpAddress"},
		{"fuego.Server", "engine/server"},
		{"AEAD", "gcm/aead"},
		{"openapi3.T", "openAPISpec"},
		{"FuncDecl", "functionDeclaration"},
		{"ast.FuncDecl", "functionDeclaration"},
		{"Regexp", "pattern/regexp/regularExpression"},
		{"regexp.Regexp", "pattern/regexp/regularExpression"},
		{"Certificate", "certificate"},
		{"x509.Certificate", "certificate"},
		{"CertPool", "certPool"},
		{"x509.CertPool", "certPool"},
		{"pgxpool.Pool", "pool"},
		{"sync.Pool", "pool"},
		{"BigInt", "bigInt"},
		{"big.Int", "bigInt"},
		{"strings.Builder", "builder"},
	}

	for _, testCase := range tests {
		t.Run(testCase.concreteName, func(t *testing.T) {
			got := canonicalVariableName(testCase.concreteName)
			if got != testCase.want {
				t.Errorf("canonicalVariableName(%q) = %q, want %q", testCase.concreteName, got, testCase.want)
			}
		})
	}
}

func TestNamingclarityShouldReportUnclearNameUnit(t *testing.T) {
	tests := []struct {
		name         string
		concreteName string
		wantExpected string
		wantReport   bool
	}{
		// 1. Single letter
		{"w", "ResponseWriter", "\"w\" should be named \"responseWriter\" or suffixed with \"ResponseWriter\"", true},
		{"r", "Request", "\"r\" should be named \"request\" or suffixed with \"Request\"", true},
		{"k", "Kernel", "\"k\" should be named \"kernel\" or suffixed with \"Kernel\"", true},
		{"s", "RangeStmt", "\"s\" should be named \"rangeStmt\" or suffixed with \"RangeStmt\"", true},
		{"h", "SlogHandler", "\"h\" should be named \"slogHandler\" or suffixed with \"SlogHandler\"", true},

		// 2. Initials and abbreviations
		{"rw", "ResponseWriter", "\"rw\" should be named \"responseWriter\" or suffixed with \"ResponseWriter\"", true},
		{"req", "Request", "\"req\" should be named \"request\" or suffixed with \"Request\"", true},
		{"customRW", "ResponseWriter", "\"customRW\" should be named \"responseWriter\" or suffixed with \"ResponseWriter\"", true},

		// 3. Suffix of exported type (strictly enforced)
		{"stmt", "RangeStmt", "\"stmt\" should be named \"rangeStmt\" or suffixed with \"RangeStmt\"", true},
		{"spec", "ValueSpec", "\"spec\" should be named \"valueSpec\" or suffixed with \"ValueSpec\"", true},
		{"dispatcher", "EmailDispatcher", "\"dispatcher\" should be named \"emailDispatcher\" or suffixed with \"EmailDispatcher\"", true},
		{"keyManager", "CryptoKeyManager", "\"keyManager\" should be named \"cryptoKeyManager\" or suffixed with \"CryptoKeyManager\"", true},
		{"claims", "OIDCIDTokenClaims", "\"claims\" should be named \"oidcIDTokenClaims\" or suffixed with \"OIDCIDTokenClaims\"", true},

		// 4. Unexported type base noun allowed (to prevent type shadowing in package scope)
		{"handler", "slogHandler", "", false},
		{"route", "baseRoute", "", false},

		// 5. DatabasePool special handling
		{"pool", "DatabasePool", "\"pool\" should be named \"db\" or suffixed with \"DB\"", true},
		{"db", "DatabasePool", "", false},
		{"badDB", "DatabasePool", "", false},
		{"primaryDB", "DatabasePool", "", false},

		// 6. context.Context -> ctx, xxxCtx (ONLY ctx or ending in Ctx)
		{"ctx", "Context", "", false},
		{"requestCtx", "Context", "", false},
		{"authCtx", "Context", "", false},
		{"c", "Context", "\"c\" should be named \"ctx\" or suffixed with \"Ctx\"", true},
		{"context", "Context", "\"context\" should be named \"ctx\" or suffixed with \"Ctx\"", true},
		{"requestContext", "Context", "\"requestContext\" should be named \"ctx\" or suffixed with \"Ctx\"", true},

		// 7. context.CancelFunc -> cancel / xxxCancel
		{"cancel", "CancelFunc", "", false},
		{"timeoutCancel", "CancelFunc", "", false},
		{"c", "CancelFunc", "\"c\" should be named \"cancel\" or suffixed with \"Cancel\"", true},
		{"cancelFunc", "CancelFunc", "\"cancelFunc\" should be named \"cancel\" or suffixed with \"Cancel\"", true},

		// 8. pgconn.CommandTag -> result / xxxResult
		{"result", "CommandTag", "", false},
		{"commandResult", "CommandTag", "", false},
		{"r", "CommandTag", "\"r\" should be named \"result\" or suffixed with \"Result\"", true},
		{"commandTag", "CommandTag", "\"commandTag\" should be named \"result\" or suffixed with \"Result\"", true},

		// 9. error -> err, xxxErr or ErrXxxXxx (sentinel errors prefix Err, local variables err or xxxErr, Error suffix prohibited)
		{"err", "error", "", false},
		{"walkErr", "error", "", false},
		{"walkError", "error", "\"walkError\" should be named \"err\", suffixed with \"Err\", or prefixed with \"Err\"", true},
		{"ErrNotFound", "error", "", false},
		{"ErrInvalidToken", "error", "", false},
		{"error", "error", "\"error\" should be named \"err\", suffixed with \"Err\", or prefixed with \"Err\"", true},
		{"error", "Error", "\"error\" should be named \"err\", suffixed with \"Err\", or prefixed with \"Err\"", true},
		{"e", "error", "\"e\" should be named \"err\", suffixed with \"Err\", or prefixed with \"Err\"", true},

		// 10. ast.FuncDecl -> functionDeclaration / xxxFunctionDeclaration
		{"functionDeclaration", "ast.FuncDecl", "", false},
		{"parseFunctionDeclaration", "ast.FuncDecl", "", false},
		{"functionDeclaration", "FuncDecl", "", false},
		{"parseFunctionDeclaration", "FuncDecl", "", false},
		{"fn", "ast.FuncDecl", "\"fn\" should be named \"functionDeclaration\" or suffixed with \"FunctionDeclaration\"", true},
		{"funcDecl", "ast.FuncDecl", "\"funcDecl\" should be named \"functionDeclaration\" or suffixed with \"FunctionDeclaration\"", true},
		{"fn", "FuncDecl", "\"fn\" should be named \"functionDeclaration\" or suffixed with \"FunctionDeclaration\"", true},

		// 11. net.Conn -> connection / xxxConnection
		{"connection", "Conn", "", false},
		{"tcpConnection", "Conn", "", false},
		{"conn", "Conn", "\"conn\" should be named \"connection\" or suffixed with \"Connection\"", true},

		// 12. net.Addr -> address / xxxAddress
		{"address", "Addr", "", false},
		{"localAddress", "Addr", "", false},
		{"addr", "Addr", "\"addr\" should be named \"address\" or suffixed with \"Address\"", true},

		// 13. net.TCPAddr -> tcpAddress / xxxTCPAddress
		{"tcpAddress", "TCPAddr", "", false},
		{"localTCPAddress", "TCPAddr", "", false},
		{"tcpAddr", "TCPAddr", "\"tcpAddr\" should be named \"tcpAddress\" or suffixed with \"TCPAddress\"", true},

		// 14. fuego.Server -> engine / xxxEngine
		{"engine", "fuego.Server", "", false},
		{"httpEngine", "fuego.Server", "", false},
		{"server", "fuego.Server", "", false},

		// 15. cipher.AEAD -> allow gcm / xxxGCM
		{"gcm", "AEAD", "", false},
		{"aesGCM", "AEAD", "", false},
		{"aead", "AEAD", "", false},
		{"cipher", "AEAD", "\"cipher\" should be named \"gcm/aead\" or suffixed with \"GCM/AEAD\"", true},

		// 16. openapi3.T -> openAPISpec / xxxOpenAPISpec
		{"openAPISpec", "openapi3.T", "", false},
		{"coreOpenAPISpec", "openapi3.T", "", false},
		{"spec", "openapi3.T", "\"spec\" should be named \"openAPISpec\" or suffixed with \"OpenAPISpec\"", true},

		// 17. regexp.Regexp -> allow regexp / xxxRegexp / regularExpression / xxxRegularExpression / pattern / xxxPattern
		{"regexp", "regexp.Regexp", "", false},
		{"customRegexp", "regexp.Regexp", "", false},
		{"regularExpression", "regexp.Regexp", "", false},
		{"customRegularExpression", "regexp.Regexp", "", false},
		{"pattern", "regexp.Regexp", "", false},
		{"customPattern", "regexp.Regexp", "", false},
		{"testNamePatterns", "regexp.Regexp", "", false},
		{"patterns", "regexp.Regexp", "", false},
		{"regexps", "regexp.Regexp", "", false},
		{"regularExpressions", "regexp.Regexp", "", false},
		{"addresses", "net.Addr", "", false},
		{"connections", "net.Conn", "", false},
		{"functionDeclarations", "ast.FuncDecl", "", false},
		{"regexp", "Regexp", "", false},
		{"customRegexp", "Regexp", "", false},
		{"r", "regexp.Regexp", "\"r\" should be named \"pattern/regexp/regularExpression\" or suffixed with \"Pattern/RegularExpression/RegularExpression\"", true},
		{"r", "Regexp", "\"r\" should be named \"pattern/regexp/regularExpression\" or suffixed with \"Pattern/RegularExpression/RegularExpression\"", true},

		// 18. Multi-word types & Webhook issues
		{"updateTargetWebhook", "Webhook", "", false},
		{"updateWebhookResult", "Webhook", "\"updateWebhookResult\" should be named \"webhook\" or suffixed with \"Webhook\"", true},
		{"event", "WebhookEventEnvelope", "\"event\" should be named \"webhookEventEnvelope\" or suffixed with \"WebhookEventEnvelope\"", true},
		{"webhookEventEnvelope", "WebhookEventEnvelope", "", false},
		{"customWebhookEventEnvelope", "WebhookEventEnvelope", "", false},
		{"webhook", "Webhook", "", false},
		{"webhooks", "Webhook", "", false},

		// 18b. x509.Certificate -> certificate / xxxCertificate / template / xxxTemplate
		{"certificate", "x509.Certificate", "", false},
		{"customCertificate", "x509.Certificate", "", false},
		{"template", "x509.Certificate", "", false},
		{"certificateTemplate", "x509.Certificate", "", false},
		{"templates", "x509.Certificate", "", false},
		{"certificates", "x509.Certificate", "", false},
		{"template", "Certificate", "", false},
		{"cert", "x509.Certificate", "\"cert\" should be named \"certificate\" or suffixed with \"Certificate\"", true},
		{"cert", "Certificate", "\"cert\" should be named \"certificate\" or suffixed with \"Certificate\"", true},

		// 19. Allowed special types per user edits
		{"writeCloser", "WriteCloser", "", false},
		{"customWriteCloser", "WriteCloser", "", false},
		{"dataWriter", "WriteCloser", "\"dataWriter\" should be named \"writer\" or suffixed with \"Writer\"", true},
		{"readCloser", "ReadCloser", "", false},
		{"customReadCloser", "ReadCloser", "", false},
		{"dataReader", "ReadCloser", "\"dataReader\" should be named \"reader\" or suffixed with \"Reader\"", true},
		{"handler", "HandlerFunc", "", false},
		{"customHandler", "HandlerFunc", "", false},
		{"handlerFunction", "HandlerFunc", "", false},
		{"customHandlerFunction", "HandlerFunc", "", false},
		{"handler", "Handler", "", false},
		{"customHandler", "Handler", "", false},
		{"listener", "Listener", "", false},
		{"customListener", "Listener", "", false},
		{"occupiedIPv4Primary", "Listener", "\"occupiedIPv4Primary\" should be named \"listener\" or suffixed with \"Listener\"", true},
		{"oAuthStatePayload", "OAuthStatePayload", "", false},
		{"oAuthProviderConfig", "OAuthProviderConfig", "", false},

		// 20. NodeRegistry & EmbeddedDatabase
		{"nodeRegistryPrimary", "NodeRegistry", "\"nodeRegistryPrimary\" should be named \"nodeRegistry\" or suffixed with \"NodeRegistry\"", true},
		{"embeddedRestart", "EmbeddedDatabase", "\"embeddedRestart\" should be named \"embeddedDatabase\" or suffixed with \"EmbeddedDatabase\"", true},
		{"db", "sql.DB", "", false},
		{"serviceAccount", "core.ServiceAccountWithSecretKey", "", false},
		{"certPool", "x509.CertPool", "", false},
		{"caCertPool", "x509.CertPool", "", false},
		{"rootCertPool", "x509.CertPool", "", false},
		{"systemCertPool", "x509.CertPool", "", false},
		{"certPool", "CertPool", "", false},
		{"caCertPool", "CertPool", "", false},
		{"cp", "x509.CertPool", "\"cp\" should be named \"certPool\" or suffixed with \"CertPool\"", true},
		{"cp", "CertPool", "\"cp\" should be named \"certPool\" or suffixed with \"CertPool\"", true},
		{"pool", "pgxpool.Pool", "", false},
		{"pgxPool", "pgxpool.Pool", "", false},
		{"db", "pgxpool.Pool", "\"db\" should be named \"pool\" or suffixed with \"Pool\"", true},
		{"p", "pgxpool.Pool", "\"p\" should be named \"pool\" or suffixed with \"Pool\"", true},
		{"pool", "sync.Pool", "", false},
		{"bufferPool", "sync.Pool", "", false},
		{"p", "sync.Pool", "\"p\" should be named \"pool\" or suffixed with \"Pool\"", true},
		{"bigInt", "big.Int", "", false},
		{"offsetBigInt", "big.Int", "", false},
		{"bigInteger", "big.Int", "", false},
		{"offsetBigInteger", "big.Int", "", false},
		{"offset", "big.Int", "", false},
		{"offset", "BigInt", "\"offset\" should be named \"bigInt\" or suffixed with \"BigInt\"", true},
		{"numericValue", "big.Int", "", false},
		{"sb", "strings.Builder", "", false},
		{"builder", "strings.Builder", "", false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name+"+"+testCase.concreteName, func(t *testing.T) {
			gotExpected, gotReport := shouldReportUnclearName(testCase.name, testCase.concreteName)
			if gotReport != testCase.wantReport || gotExpected != testCase.wantExpected {
				t.Errorf("shouldReportUnclearName(%q, %q) = (%q, %v), want (%q, %v)",
					testCase.name, testCase.concreteName, gotExpected, gotReport, testCase.wantExpected, testCase.wantReport)
			}
		})
	}
}

func TestNamingclarityDisallowedIdentifiersUnit(t *testing.T) {
	disallowedCases := []string{
		// Single words
		"b64", "B64", "cfg", "CFG", "pool", "km", "mgr", "sa", "wh",
		"ch", "chan", "mu", "col", "cols", "val", "vals", "obj", "mut",
		"req", "res", "resp", "tz", "admin", "administrator", "operator",

		// Compound patterns
		"dbPool", "databasePool", "poolDB", "keyMgr", "saID", "saKey", "saManager",
		"whServer", "whID", "eventsCh", "eventsChan", "failCh", "failChan",
		"colQuoted", "badCol", "valRows", "badVal", "firstObj", "setObj",
		"badMut", "mutOp", "mutRes", "reqMatch", "userReq", "userResp",
		"noContentResp", "tzMap", "tzEntries", "consoleUsr", "cpUser",

		// Bad initialisms (casing violations)
		"accountId", "endpointUrl", "requestTtl", "accountSid", "sessionJwt",

		// Contained in compounds
		"serverCfg", "cfgServer", "eventCh", "eventsCh", "badVal", "testVal",
		"valRows", "myPool", "poolManager", "colName", "b64Payload", "payloadB64",
		"adminUser", "systemAdministrator", "clusterOperator", "authReq",
		"authRes", "userResp", "localTz", "keyMgr", "whServer", "saKey",
		"poolOptions", "myPoolOptions", "databasePoolManager",
	}

	for _, name := range disallowedCases {
		t.Run("disallowed_"+name, func(t *testing.T) {
			matchedTerm, isDisallowed := isDisallowedIdentifier(name)
			if !isDisallowed || matchedTerm == "" {
				t.Errorf("isDisallowedIdentifier(%q) = (%q, %v), want (_, true)", name, matchedTerm, isDisallowed)
			}
		})
	}

	allowedCases := []string{
		// Clean identifiers
		"failChannel", "eventsChannel", "serviceAccountID", "endpointURL", "requestTTL",
		"sessionJWT", "accountSID",
		"databasePoolOptions", "customDatabasePoolOptions", "badTimeoutDatabasePoolOptions",
		"sslDatabasePoolOptions", "sslInlineDatabasePoolOptions", "validSSLDatabasePoolOptions",
		"databasePoolOption",
		"caCertPool", "certPool", "rootCertPool", "systemCertPool", "certificatePool", "caCertificatePool",
		"value", "Value", "values", "Values", "myValue", "userValue", "valueString",
		"valid", "Valid", "isValid", "validate", "validation", "validator",
		"eval", "evaluate", "oval",
		"timezone", "localTimezone",
		"request", "authRequest", "httpRequest",
		"response", "authResponse", "userResponse", "result",
		"channel", "eventChannel",
		"mutex", "lockMutex",
		"database", "primaryDatabase",
		"manager", "configManager",
		"color", "itemColor",
		"object", "userObject",
		"mutation", "dataMutation",
		"cleanIdentifier",
		"_",
		"",
	}

	for _, name := range allowedCases {
		t.Run("allowed_"+name, func(t *testing.T) {
			matchedTerm, isDisallowed := isDisallowedIdentifier(name)
			if isDisallowed {
				t.Errorf("isDisallowedIdentifier(%q) = (%q, %v), want (_, false)", name, matchedTerm, isDisallowed)
			}
		})
	}
}

func TestNamingclarityHasProhibitedSuffixUnit(t *testing.T) {
	tests := []struct {
		name string
		want bool
	}{
		{"tokenStr", true},
		{"NameStr", true},
		{"countInt", true},
		{"CountInt", true},
		{"validBool", true},
		{"ValidBool", true},
		{"activeBoolean", true},
		{"ActiveBoolean", true},

		{"nameString", false},
		{"NameString", false},
		{"connectionString", false},
		{"queryString", false},
		{"templateText", false},
		{"descriptionText", false},
		{"statusText", false},
		{"plainText", false},
		{"cipherText", false},
		{"levelText", false},

		{"Str", false},
		{"String", false},
		{"Int", false},
		{"Bool", false},
		{"Boolean", false},
		{"Text", false},
		{"point", false},
		{"print", false},
		{"offsetBigInt", false},
		{"bigInt", false},
		{"offsetBigInteger", false},
		{"bigInteger", false},
		{"databaseURL", false},
		{"redisURI", false},
		{"emailDispatcher", false},
		{"cryptoKeyManager", false},
		{"_", false},
	}

	for _, testCase := range tests {
		t.Run(testCase.name, func(t *testing.T) {
			got := hasProhibitedSuffix(testCase.name)
			if got != testCase.want {
				t.Errorf("hasProhibitedSuffix(%q) = %v, want %v", testCase.name, got, testCase.want)
			}
		})
	}
}

func TestNamingclarityPluginUnit(t *testing.T) {
	pluginInstance, err := New(nil)
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	analyzers, err := pluginInstance.BuildAnalyzers()
	if err != nil || len(analyzers) != 1 {
		t.Fatalf("unexpected analyzers: %v, %v", analyzers, err)
	}
	if pluginInstance.GetLoadMode() != register.LoadModeTypesInfo {
		t.Fatalf("unexpected load mode: %v", pluginInstance.GetLoadMode())
	}
}

func TestNamingclarityHelperFunctionsUnit(t *testing.T) {
	if capitalizeFirst("") != "" {
		t.Error("expected empty string")
	}
	if lowerPrefix("") != "" {
		t.Error("expected empty string")
	}
	if got := lowerPrefix("ALLUPPER"); got != "allupper" {
		t.Errorf("got %q, want %q", got, "allupper")
	}
	if got := lowerPrefix("URL"); got != "url" {
		t.Errorf("got %q, want %q", got, "url")
	}
	if got := lowerPrefix("OIDCState"); got != "oidcState" {
		t.Errorf("got %q, want %q", got, "oidcState")
	}
	if got := lowerPrefix("APIClient"); got != "apiClient" {
		t.Errorf("got %q, want %q", got, "apiClient")
	}
	if got := lowerPrefix("Simple"); got != "simple" {
		t.Errorf("got %q, want %q", got, "simple")
	}
	if got, ok := shouldReportUnclearName("", ""); ok || got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if got, ok := shouldReportUnclearName("_", "Foo"); ok || got != "" {
		t.Errorf("got %q, want empty", got)
	}
	if got, ok := shouldReportUnclearName("foo", "slogHandler"); !ok || got == "" {
		t.Errorf("got (%q, %v), want non-empty", got, ok)
	}

	canonical, tail := typeNamingTargets("Context")
	if canonical != "ctx" || tail != "Ctx" {
		t.Errorf("typeNamingTargets(Context) = (%q, %q), want (ctx, Ctx)", canonical, tail)
	}
	canonicalGen, tailGen := typeNamingTargets("CustomType")
	if canonicalGen != "customType" || tailGen != "CustomType" {
		t.Errorf("typeNamingTargets(CustomType) = (%q, %q), want (customType, CustomType)", canonicalGen, tailGen)
	}

	// splitIdentifierWords testing
	words := splitIdentifierWords("my_col_1_name")
	if len(words) != 3 || words[0] != "my" || words[1] != "col" || words[2] != "name" {
		t.Errorf("splitIdentifierWords failed: %v", words)
	}
	if len(splitIdentifierWords("")) != 0 {
		t.Error("expected empty slice for empty string")
	}
	wordsAcronym := splitIdentifierWords("HTTPClient")
	if len(wordsAcronym) != 2 || wordsAcronym[0] != "HTTP" || wordsAcronym[1] != "Client" {
		t.Errorf("splitIdentifierWords acronym failed: %v", wordsAcronym)
	}

	// specialTypeRule.checkAllowed
	emptyRule := specialTypeRule{}
	if emptyRule.checkAllowed("foo") {
		t.Error("expected false for nil isAllowed")
	}
	pluralRule := specialTypeRule{
		allowPlural: true,
		isAllowed: func(name string) bool {
			return name == "box"
		},
	}
	if !pluralRule.checkAllowed("boxes") {
		t.Error("expected true for boxes with allowPlural")
	}
	if pluralRule.checkAllowed("random") {
		t.Error("expected false for random")
	}
}

func TestNamingclarityDirectFunctionsUnit(t *testing.T) {
	// lowerPrefix branches
	if got := lowerPrefix("UUID"); got != "uuid" {
		t.Errorf("got %q, want uuid", got)
	}
	if got := lowerPrefix("UUIDHelper"); got != "uuidHelper" {
		t.Errorf("got %q, want uuidHelper", got)
	}
	if got := lowerPrefix("XYZ"); got != "xyz" {
		t.Errorf("got %q, want xyz", got)
	}
	if got := lowerPrefix("FOOBar"); got != "fooBar" {
		t.Errorf("got %q, want fooBar", got)
	}
	if got := lowerPrefix("FOOBAR1"); got != "foobar1" {
		t.Errorf("got %q, want foobar1", got)
	}

	// concreteTypeName branches
	if _, ok := concreteTypeName(nil); ok {
		t.Error("expected false for nil type")
	}
	if _, ok := concreteTypeName(types.Typ[types.Int]); ok {
		t.Error("expected false for basic type")
	}
	namedObj := types.NewTypeName(token.NoPos, nil, "MyType", nil)
	namedType := types.NewNamed(namedObj, types.NewStruct(nil, nil), nil)
	ptrType := types.NewPointer(types.NewPointer(namedType))
	if name, ok := concreteTypeName(ptrType); !ok || name != "MyType" {
		t.Errorf("got (%q, %v), want (MyType, true)", name, ok)
	}
	aliasType := types.NewAlias(namedObj, namedType)
	if name, ok := concreteTypeName(aliasType); !ok || name != "MyType" {
		t.Errorf("got (%q, %v), want (MyType, true)", name, ok)
	}

	// ast package FuncDecl
	astPackage := types.NewPackage("go/ast", "ast")
	astFuncDeclTypeName := types.NewTypeName(token.NoPos, astPackage, "FuncDecl", nil)
	astFuncDeclType := types.NewNamed(astFuncDeclTypeName, types.NewStruct(nil, nil), nil)
	if name, ok := concreteTypeName(astFuncDeclType); !ok || name != "ast.FuncDecl" {
		t.Errorf("got (%q, %v), want (ast.FuncDecl, true)", name, ok)
	}

	// fuego package Server
	fuegoPackage := types.NewPackage("layr.sh/fuego", "fuego")
	fuegoServerTypeName := types.NewTypeName(token.NoPos, fuegoPackage, "Server", nil)
	fuegoServerType := types.NewNamed(fuegoServerTypeName, types.NewStruct(nil, nil), nil)
	if name, ok := concreteTypeName(fuegoServerType); !ok || name != "fuego.Server" {
		t.Errorf("got (%q, %v), want (fuego.Server, true)", name, ok)
	}

	// openapi3 package T
	openapi3Package := types.NewPackage("github.com/getkin/kin-openapi/openapi3", "openapi3")
	openapi3TTypeName := types.NewTypeName(token.NoPos, openapi3Package, "T", nil)
	openapi3TType := types.NewNamed(openapi3TTypeName, types.NewStruct(nil, nil), nil)
	if name, ok := concreteTypeName(openapi3TType); !ok || name != "openapi3.T" {
		t.Errorf("got (%q, %v), want (openapi3.T, true)", name, ok)
	}

	// regexp package Regexp
	regexpPackage := types.NewPackage("regexp", "regexp")
	regexpTypeName := types.NewTypeName(token.NoPos, regexpPackage, "Regexp", nil)
	regexpType := types.NewNamed(regexpTypeName, types.NewStruct(nil, nil), nil)
	if name, ok := concreteTypeName(regexpType); !ok || name != "regexp.Regexp" {
		t.Errorf("got (%q, %v), want (regexp.Regexp, true)", name, ok)
	}
}

func TestNamingclarityASTHelpersUnit(t *testing.T) {
	fileSet := token.NewFileSet()
	testFile, _ := parser.ParseFile(fileSet, "test_test.go", "package p\nimport \"testing\"\nfunc TestX(t *testing.T){}", 0)
	typeCheckerConfig := types.Config{Importer: importer.Default()}
	typeInformation := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	typePackage, _ := typeCheckerConfig.Check("p", fileSet, []*ast.File{testFile}, typeInformation)
	var diagnosticReports []string
	analysisPass := &analysis.Pass{
		Analyzer:  Analyzer,
		Fset:      fileSet,
		Files:     []*ast.File{testFile},
		Pkg:       typePackage,
		TypesInfo: typeInformation,
		Report: func(diagnostic analysis.Diagnostic) {
			diagnosticReports = append(diagnosticReports, diagnostic.Message)
		},
	}

	namedTypeName := types.NewTypeName(token.NoPos, nil, "MyType", nil)
	namedType := types.NewNamed(namedTypeName, types.NewStruct(nil, nil), nil)

	// checkFuncParams(nil)
	checkFuncParams(analysisPass, nil)

	// isExemptIdentifier branches
	if !isExemptIdentifier(analysisPass, ast.NewIdent("_"), "Any") {
		t.Error("expected true for _")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("ctx"), "Context") {
		t.Error("expected true for ctx")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("requestCtx"), "Context") {
		t.Error("expected true for requestCtx")
	}
	if isExemptIdentifier(analysisPass, ast.NewIdent("context"), "Context") {
		t.Error("expected false for context (must be ctx or xxxCtx)")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("functionDeclaration"), "ast.FuncDecl") {
		t.Error("expected true for functionDeclaration")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("parseFunctionDeclaration"), "ast.FuncDecl") {
		t.Error("expected true for parseFunctionDeclaration")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("cancel"), "CancelFunc") {
		t.Error("expected true for cancel")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("timeoutCancel"), "CancelFunc") {
		t.Error("expected true for timeoutCancel")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("result"), "CommandTag") {
		t.Error("expected true for result")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("cmdResult"), "CommandTag") {
		t.Error("expected true for cmdResult")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("err"), "error") {
		t.Error("expected true for err")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("walkErr"), "error") {
		t.Error("expected true for walkErr")
	}
	if isExemptIdentifier(analysisPass, ast.NewIdent("walkError"), "error") {
		t.Error("expected false for walkError (Error suffix prohibited)")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("ErrNotFound"), "error") {
		t.Error("expected true for ErrNotFound")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("ok"), "bool") {
		t.Error("expected true for ok")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("connection"), "Conn") {
		t.Error("expected true for connection")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("tcpConnection"), "Conn") {
		t.Error("expected true for tcpConnection")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("address"), "Addr") {
		t.Error("expected true for address")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("localAddress"), "Addr") {
		t.Error("expected true for localAddress")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("tcpAddress"), "TCPAddr") {
		t.Error("expected true for tcpAddress")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("localTCPAddress"), "TCPAddr") {
		t.Error("expected true for localTCPAddress")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("engine"), "fuego.Server") {
		t.Error("expected true for engine")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("httpEngine"), "fuego.Server") {
		t.Error("expected true for httpEngine")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("gcm"), "AEAD") {
		t.Error("expected true for gcm")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("aesGCM"), "AEAD") {
		t.Error("expected true for aesGCM")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("openAPISpec"), "openapi3.T") {
		t.Error("expected true for openAPISpec")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("writeCloser"), "WriteCloser") {
		t.Error("expected true for writeCloser")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("readCloser"), "ReadCloser") {
		t.Error("expected true for readCloser")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("handler"), "HandlerFunc") {
		t.Error("expected true for handler")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("handlerFunction"), "HandlerFunc") {
		t.Error("expected true for handlerFunction")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("handler"), "Handler") {
		t.Error("expected true for handler")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("listener"), "Listener") {
		t.Error("expected true for listener")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("regexp"), "regexp.Regexp") {
		t.Error("expected true for regexp")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("regularExpression"), "regexp.Regexp") {
		t.Error("expected true for regularExpression")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("pattern"), "regexp.Regexp") {
		t.Error("expected true for pattern")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("db"), "DatabasePool") {
		t.Error("expected true for db")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("log"), "Logger") {
		t.Error("expected true for log")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("log"), "core.Logger") {
		t.Error("expected true for log with core.Logger")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("now"), "Time") {
		t.Error("expected true for now")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("now"), "time.Time") {
		t.Error("expected true for now with time.Time")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("timeout"), "Duration") {
		t.Error("expected true for timeout")
	}
	if !isExemptIdentifier(analysisPass, ast.NewIdent("timeout"), "time.Duration") {
		t.Error("expected true for timeout with time.Duration")
	}
	if isExemptIdentifier(analysisPass, ast.NewIdent("unknown"), "UnknownType") {
		t.Error("expected false for unknown")
	}

	// t identifier in test file
	var testIdentifier *ast.Ident
	ast.Inspect(testFile, func(node ast.Node) bool {
		if identifier, ok := node.(*ast.Ident); ok && identifier.Name == "t" {
			testIdentifier = identifier
			return false
		}
		return true
	})
	if testIdentifier == nil || !isExemptIdentifier(analysisPass, testIdentifier, "T") {
		t.Error("expected true for t in test file")
	}

	// concreteTypeNameFromExpression with nil
	if _, ok := concreteTypeNameFromExpression(analysisPass, &ast.BadExpr{}); ok {
		t.Error("expected false for BadExpr")
	}

	// concreteTypeNameFromIdent
	dummyIdentifier := ast.NewIdent("dummy")
	if _, ok := concreteTypeNameFromIdent(analysisPass, dummyIdentifier); ok {
		t.Error("expected false for unregistered ident")
	}

	// checkProhibitedSuffix with nil and _
	if checkProhibitedSuffix(analysisPass, nil) {
		t.Error("expected false for nil")
	}
	if checkProhibitedSuffix(analysisPass, ast.NewIdent("_")) {
		t.Error("expected false for _")
	}

	// checkDisallowedIdentifier with nil, _, exempt, and prohibited term
	if checkDisallowedIdentifier(analysisPass, nil) {
		t.Error("expected false for nil")
	}
	if checkDisallowedIdentifier(analysisPass, ast.NewIdent("_")) {
		t.Error("expected false for _")
	}
	if checkDisallowedIdentifier(analysisPass, ast.NewIdent("ok")) {
		t.Error("expected false for ok")
	}
	if !checkDisallowedIdentifier(analysisPass, ast.NewIdent("b64Token")) {
		t.Error("expected true for b64Token")
	}
	if checkDisallowedIdentifier(analysisPass, ast.NewIdent("validVariable")) {
		t.Error("expected false for validVariable")
	}
	if checkDisallowedIdentifier(analysisPass, ast.NewIdent("pool"), "pgxpool.Pool") {
		t.Error("expected false for pool with pgxpool.Pool")
	}
	if !checkDisallowedIdentifier(analysisPass, ast.NewIdent("pool"), "core.DatabasePool") {
		t.Error("expected true for pool with core.DatabasePool")
	}

	// checkIdentifier with allowed name (no report)
	checkIdentifier(analysisPass, ast.NewIdent("manager"), "Manager")
	checkIdentifier(analysisPass, ast.NewIdent("ctx"), "Context")

	// checkFields with unnamed field
	checkFields(analysisPass, []*ast.Field{{Type: ast.NewIdent("int")}})
	checkStructFields(analysisPass, []*ast.Field{{Type: ast.NewIdent("int")}})

	// isExemptIdentifier with Defs containing testing
	testingPackage := types.NewPackage("testing", "testing")
	testingTypeName := types.NewTypeName(token.NoPos, testingPackage, "T", nil)
	testingNamedType := types.NewNamed(testingTypeName, types.NewStruct(nil, nil), nil)
	testDefIdentifier := ast.NewIdent("t")
	analysisPass.TypesInfo.Defs[testDefIdentifier] = types.NewVar(token.NoPos, nil, "t", testingNamedType)
	if !isExemptIdentifier(analysisPass, testDefIdentifier, "T") {
		t.Error("expected true for t with Defs testing")
	}

	// isExemptIdentifier fallback to _test.go filename
	testFileIdentifier := &ast.Ident{Name: "t", NamePos: testFile.Pos()}
	if !isExemptIdentifier(analysisPass, testFileIdentifier, "T") {
		t.Error("expected true for t with file _test.go")
	}

	// concreteTypeNameFromIdent Uses and TypeOf branches
	identifierUse := ast.NewIdent("usedIdent")
	analysisPass.TypesInfo.Uses[identifierUse] = types.NewVar(token.NoPos, nil, "usedIdent", namedType)
	if name, ok := concreteTypeNameFromIdent(analysisPass, identifierUse); !ok || name != "MyType" {
		t.Errorf("got (%q, %v), want (MyType, true)", name, ok)
	}

	identifierType := ast.NewIdent("typedIdent")
	analysisPass.TypesInfo.Types[identifierType] = types.TypeAndValue{Type: namedType}
	if name, ok := concreteTypeNameFromIdent(analysisPass, identifierType); !ok || name != "MyType" {
		t.Errorf("got (%q, %v), want (MyType, true)", name, ok)
	}

	// checkFuncParams with prohibited suffix and unclear concrete type
	funcParamsList := &ast.FieldList{
		List: []*ast.Field{
			{Names: []*ast.Ident{ast.NewIdent("paramStr")}},
			{Names: []*ast.Ident{ast.NewIdent("badManager")}, Type: ast.NewIdent("MyType")},
		},
	}
	analysisPass.TypesInfo.Types[funcParamsList.List[1].Type] = types.TypeAndValue{Type: namedType}
	checkFuncParams(analysisPass, funcParamsList)

	// checkAssignment non-tuple multi-lhs
	assignNonTupleStmt := &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("tokenStr"), identifierType, ast.NewIdent("untypedLhs")},
		Rhs: []ast.Expr{&ast.BadExpr{}},
	}
	checkAssignment(analysisPass, assignNonTupleStmt)

	// checkAssignment with ident type
	assignFromIdentStmt := &ast.AssignStmt{
		Lhs: []ast.Expr{identifierUse},
		Rhs: []ast.Expr{&ast.BadExpr{}},
	}
	checkAssignment(analysisPass, assignFromIdentStmt)

	// checkAssignment tuple with prohibited suffix on LHS
	tupleType := types.NewTuple(types.NewVar(token.NoPos, nil, "a", namedType), types.NewVar(token.NoPos, nil, "b", namedType))
	tupleExpression := ast.NewIdent("tupleExpr")
	analysisPass.TypesInfo.Types[tupleExpression] = types.TypeAndValue{Type: tupleType}
	assignTupleProhibitedStmt := &ast.AssignStmt{
		Lhs: []ast.Expr{ast.NewIdent("tokenStr"), ast.NewIdent("validIdent")},
		Rhs: []ast.Expr{tupleExpression},
	}
	checkAssignment(analysisPass, assignTupleProhibitedStmt)

	// checkFields with field.Type nil and prohibited suffix
	checkFields(analysisPass, []*ast.Field{{Names: []*ast.Ident{ast.NewIdent("fieldStr")}}})
	checkFields(analysisPass, []*ast.Field{{Names: []*ast.Ident{identifierType}}})
}

func TestNamingclarityAnalyzerRunUnit(t *testing.T) {
	sourceCode := `package testpkg

import (
	"context"
	"math/big"
	"testing"
	"time"
)

type MyManager struct{}

type TestStruct struct {
	ExportedField string
	unexportedField string
	fieldStr string
	badManager *MyManager
	goodManager *MyManager
	string
}

func (m *MyManager) DoSomething(ctx context.Context, w int, validParam string) (result string, err error) {
	return "", nil
}

func OtherFunc(x int) (res int) {
	return 0
}

func BlankFunc(_ int) {}

func SingleLetterFunc(t *testing.T, w MyManager) {}

func RunChecks(ctx context.Context) {
	var defaultLimit = big.NewInt(50)
	_ = defaultLimit
	maxLimit := big.NewInt(100)
	_ = maxLimit
	var countInt int
	_ = countInt
	var uninitManager MyManager
	_ = uninitManager
	var goodVar MyManager
	_ = goodVar
	var badVar = MyManager{}
	_ = badVar

	goodIdent := MyManager{}
	var badIdent = goodIdent
	_ = badIdent

	var _ = 123

	badAssign := MyManager{}
	_ = badAssign
	goodAssignManager := MyManager{}
	_ = goodAssignManager

	twoLhsA, twoLhsB := MyManager{}, MyManager{}
	_ = twoLhsA
	_ = twoLhsB

	tupleVal, tupleErr := (*MyManager)(nil).DoSomething(ctx, 1, "ok")
	_ = tupleVal
	_ = tupleErr

	var tupleBadStr string
	tupleBadStr, tupleErr = (*MyManager)(nil).DoSomething(ctx, 1, "ok")
	_ = tupleBadStr

	slice := []string{"a", "b"}
	for idxInt, valStr := range slice {
		_ = idxInt
		_ = valStr
	}
	for _, ok := range []bool{true} {
		_ = ok
	}
	for i := range slice {
		_ = i
	}

	ch := make(chan int)
	mapVal := make(map[string]int)
	c, okVal := <-ch
	_ = c
	_ = okVal
	mVal, mOk := mapVal["k"]
	_ = mVal
	_ = mOk

	var reassignedManager MyManager
	reassignedManager = MyManager{}
	_ = reassignedManager

	prohibitedStr := "test"
	_ = prohibitedStr

	now := time.Now()
	_ = now
	timeout := time.Second
	_ = timeout
}
`

	fileSet := token.NewFileSet()
	testFile, err := parser.ParseFile(fileSet, "test_test.go", sourceCode, 0)
	if err != nil {
		t.Fatalf("failed to parse test source: %v", err)
	}

	typeCheckerConfig := types.Config{
		Importer: importer.Default(),
	}
	typeInformation := &types.Info{
		Types: make(map[ast.Expr]types.TypeAndValue),
		Defs:  make(map[*ast.Ident]types.Object),
		Uses:  make(map[*ast.Ident]types.Object),
	}
	typePackage, err := typeCheckerConfig.Check("testpkg", fileSet, []*ast.File{testFile}, typeInformation)
	if err != nil {
		t.Fatalf("failed to typecheck test source: %v", err)
	}

	var diagnosticReports []string
	analysisPass := &analysis.Pass{
		Analyzer:  Analyzer,
		Fset:      fileSet,
		Files:     []*ast.File{testFile},
		Pkg:       typePackage,
		TypesInfo: typeInformation,
		Report: func(diagnostic analysis.Diagnostic) {
			diagnosticReports = append(diagnosticReports, diagnostic.Message)
		},
	}

	_, err = run(analysisPass)
	if err != nil {
		t.Fatalf("run failed: %v", err)
	}

	if len(diagnosticReports) == 0 {
		t.Fatal("expected diagnostic reports, got none")
	}
}
