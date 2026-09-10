package namingclarity

import (
	"strings"
	"unicode"
)

// Prohibited representation suffixes (Hungarian notation).
// Identifiers must convey domain purpose rather than encoding their primitive data type.
// Examples:
//   - Disallowed: countInt, nameStr, validBool, activeBoolean
//   - Allowed:    count, name, isValid, active
var prohibitedSuffixes = []string{
	"Boolean",
	"Bool",
	"Str",
	"Int",
}

// Known acronyms to preserve or lowercase consistently according to standard Go conventions.
// Go initialisms should either be all-uppercase (e.g. "URL", "ID", "HTTP") or all-lowercase
// when starting an unexported camelCase identifier (e.g. "url", "id", "httpServer").
// Examples:
//   - Correct:   userID, databaseURL, httpHandler, oauthToken
//   - Incorrect: userId, databaseUrl, HttpHandler, oAuthToken
var knownInitialisms = []string{
	"OAuth", "OIDC", "UUID", "HTTP", "JSON", "HTML", "SMTP", "TOTP",
	"REST", "JWT", "URL", "TTL", "API", "SMS", "OTP", "TLS", "TCP",
	"UDP", "SQL", "URI", "UID", "GID", "IP", "ID", "DB",
}

// Bad initialism casing when used as component words.
// Maps Pascal-cased acronym misnomers to their standard lowercase form.
// Examples:
//   - Disallowed: userId, serverUrl, tokenTtl, sessionSid
//   - Expected:   userID, serverURL, tokenTTL, sessionSID
var badInitialisms = map[string]string{
	"Id":  "id",
	"Url": "url",
	"Ttl": "ttl",
	"Jwt": "jwt",
	"Sid": "sid",
}

// Disallowed standalone component words.
// Stored as map[string]bool to serve as an idiomatic Go Set providing O(1) hash lookup.
//
// Identifiers are split into camelCase component words via splitIdentifierWords(name)
// before checking against this set. This flags standalone shorthand or ambiguous abbreviations
// without causing false positives on legitimate English words.
//
// Examples of flagged identifiers:
//   - "userCfg"   -> word "Cfg" is disallowed (use "config" or "configuration")
//   - "authReq"   -> word "Req" is disallowed (use "request")
//   - "apiResp"   -> word "Resp" is disallowed (use "response")
//   - "userMgr"   -> word "Mgr" is disallowed (use "manager")
//   - "saToken"   -> word "Sa" is disallowed (use "serviceAccount")
//
// Examples of allowed identifiers (NOT falsely flagged because they are whole distinct words):
//   - "validate", "value", "valid"   (contains "val", but "val" is not a standalone component)
//   - "result", "resource", "reset"  (contains "res", but "res" is not a standalone component)
//   - "channel", "check", "cache"    (contains "ch", but "ch" is not a standalone component)
//   - "column", "collection"         (contains "col", but "col" is not a standalone component)
var disallowedWords = map[string]bool{
	"b64":           true,
	"cfg":           true,
	"pool":          true,
	"km":            true,
	"mgr":           true,
	"sa":            true,
	"wh":            true,
	"ch":            true,
	"chan":          true,
	"mu":            true,
	"col":           true,
	"cols":          true,
	"val":           true,
	"vals":          true,
	"obj":           true,
	"mut":           true,
	"req":           true,
	"res":           true,
	"resp":          true,
	"tz":            true,
	"admin":         true,
	"administrator": true,
	"operator":      true,
	"cp":            true,
	"usr":           true,
}

// Disallowed compound substrings.
// Checked via strings.Contains(strings.ToLower(identifier), pattern) across the entire identifier.
//
// Unlike disallowedWords (which checks split camelCase words), this list catches mashed-together
// compound slang and abbreviations where no camelCase boundary exists.
//
// Examples of flagged identifiers:
//   - "keymgr"     -> flags "mykeymgrService", "keymgr" (use "keyManager")
//   - "sakey"      -> flags "sakey", "userSakey" (use "serviceAccountKey")
//   - "said"       -> flags "said" (use "serviceAccountID")
//   - "valrows"    -> flags "valrows" (use "valueRows")
//   - "colquoted"  -> flags "colquoted" (use "columnQuoted")
//   - "tzmap"      -> flags "tzmap" (use "timeZoneMap")
var disallowedCompoundPatterns = []string{
	"b64",
	"keymgr",
	"said",
	"sakey",
	"samanager",
	"whserver",
	"whid",
	"colquoted",
	"badcol",
	"valrows",
	"badval",
	"firstobj",
	"setobj",
	"badmut",
	"mutop",
	"mutres",
	"reqmatch",
	"tzmap",
	"tzentries",
	"consoleusr",
	"cpuser",
}

// specialTypeRule defines variable naming constraints for specific Go types.
//
// Fields:
//   - canonicalName: The preferred standalone variable name (e.g. "ctx" for context.Context).
//   - expectedTail:  The required camelCase suffix if not using canonicalName (e.g. "Ctx" for userCtx).
//   - allowPlural:   When true, pluralized forms ("s" or "es") are accepted (e.g. "connections").
//   - isAllowed:     Custom validator predicate. Returning true disables suffix enforcement
//     for general-purpose types (e.g. time.Duration, big.Int, strings.Builder).
type specialTypeRule struct {
	canonicalName string
	expectedTail  string
	allowPlural   bool
	isAllowed     func(name string) bool
}

// checkAllowed evaluates whether an identifier satisfies this rule, including optional plural suffixes.
func (r specialTypeRule) checkAllowed(name string) bool {
	if r.isAllowed == nil {
		return false
	}
	if r.isAllowed(name) {
		return true
	}
	if r.allowPlural {
		if strings.HasSuffix(name, "s") && r.isAllowed(strings.TrimSuffix(name, "s")) {
			return true
		}
		if strings.HasSuffix(name, "es") && r.isAllowed(strings.TrimSuffix(name, "es")) {
			return true
		}
	}
	return false
}

// specialTypeRules maps concrete Go type names (pkg.Type) to their naming constraints.
// Builtin universe types without package (error) remain bare.
//
// Types configured with `isAllowed: func(name string) bool { return true }` allow any domain name
// without enforcing artificial type suffixes (e.g. big.Int, time.Duration, strings.Builder).
var specialTypeRules = map[string]specialTypeRule{
	"context.Context": {
		canonicalName: "ctx",
		expectedTail:  "Ctx",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "ctx" || strings.HasSuffix(name, "Ctx")
		},
	},
	"context.CancelFunc": {
		canonicalName: "cancel",
		expectedTail:  "Cancel",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "cancel" || strings.HasSuffix(name, "Cancel")
		},
	},
	"pgconn.CommandTag": {
		canonicalName: "result",
		expectedTail:  "Result",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "result" || strings.HasSuffix(name, "Result")
		},
	},
	"error": {
		canonicalName: "err",
		expectedTail:  "Err",
		allowPlural:   false,
		isAllowed:     isErrorAllowed,
	},
	"ast.FuncDecl": {
		canonicalName: "functionDeclaration",
		expectedTail:  "FunctionDeclaration",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "functionDeclaration" || strings.HasSuffix(name, "FunctionDeclaration")
		},
	},
	"core.DatabasePool": {
		canonicalName: "db",
		expectedTail:  "DB",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "db" || strings.HasSuffix(name, "DB")
		},
	},
	"sql.DB": {
		canonicalName: "db",
		expectedTail:  "DB",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "db" || strings.HasSuffix(name, "DB")
		},
	},
	"net.Conn": {
		canonicalName: "connection",
		expectedTail:  "Connection",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "connection" || strings.HasSuffix(name, "Connection")
		},
	},
	"net.Addr": {
		canonicalName: "address",
		expectedTail:  "Address",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "address" || strings.HasSuffix(name, "Address")
		},
	},
	"net.TCPAddr": {
		canonicalName: "tcpAddress",
		expectedTail:  "TCPAddress",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "tcpAddress" || strings.HasSuffix(name, "TCPAddress")
		},
	},
	"fuego.Server": {
		canonicalName: "engine/server",
		expectedTail:  "Engine/Server",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "engine" || strings.HasSuffix(name, "Engine") || name == "server" || strings.HasSuffix(name, "Server")
		},
	},
	"cipher.AEAD": {
		canonicalName: "gcm/aead",
		expectedTail:  "GCM/AEAD",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "gcm" || strings.HasSuffix(name, "GCM") || name == "aead" || strings.HasSuffix(name, "AEAD")
		},
	},
	"openapi3.T": {
		canonicalName: "openAPISpec",
		expectedTail:  "OpenAPISpec",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "openAPISpec" || strings.HasSuffix(name, "OpenAPISpec")
		},
	},
	"regexp.Regexp": {
		canonicalName: "pattern/regexp/regularExpression",
		expectedTail:  "Pattern/RegularExpression/RegularExpression",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "pattern" || strings.HasSuffix(name, "Pattern") ||
				name == "regexp" || strings.HasSuffix(name, "Regexp") ||
				name == "regularExpression" || strings.HasSuffix(name, "RegularExpression")
		},
	},
	"io.WriteCloser": {
		canonicalName: "writer",
		expectedTail:  "Writer",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "writeCloser" || strings.HasSuffix(name, "WriteCloser")
		},
	},
	"io.ReadCloser": {
		canonicalName: "reader",
		expectedTail:  "Reader",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "readCloser" || strings.HasSuffix(name, "ReadCloser")
		},
	},
	"http.HandlerFunc": {
		canonicalName: "handler/handlerFunction",
		expectedTail:  "Handler/HandlerFunction",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "handler" || strings.HasSuffix(name, "Handler") || name == "handlerFunction" || strings.HasSuffix(name, "HandlerFunction")
		},
	},
	"http.Handler": {
		canonicalName: "handler",
		expectedTail:  "Handler",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "handler" || strings.HasSuffix(name, "Handler")
		},
	},
	"net.Listener": {
		canonicalName: "listener",
		expectedTail:  "Listener",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "listener" || strings.HasSuffix(name, "Listener")
		},
	},
	"time.Time": {
		canonicalName: "time/now/start/end",
		expectedTail:  "Time/Start/End/At/Until/Since",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "time" || name == "now" || name == "start" || name == "end" ||
				strings.HasSuffix(name, "Time") || strings.HasSuffix(name, "Start") || strings.HasSuffix(name, "End") ||
				strings.HasSuffix(name, "At") || strings.HasSuffix(name, "Until") || strings.HasSuffix(name, "Since")
		},
	},
	"time.Duration": {
		canonicalName: "duration",
		expectedTail:  "Duration",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return true
		},
	},
	"core.Logger": {
		canonicalName: "log",
		expectedTail:  "Log",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "log" || strings.HasSuffix(name, "Log") ||
				name == "logger" || strings.HasSuffix(name, "Logger")
		},
	},
	"core.ServiceAccountWithSecretKey": {
		canonicalName: "serviceAccount",
		expectedTail:  "ServiceAccount",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "serviceAccount" || strings.HasSuffix(name, "ServiceAccount")
		},
	},
	"x509.Certificate": {
		canonicalName: "certificate",
		expectedTail:  "Certificate",
		allowPlural:   true,
		isAllowed: func(name string) bool {
			return name == "certificate" || strings.HasSuffix(name, "Certificate") ||
				name == "template" || strings.HasSuffix(name, "Template")
		},
	},
	"x509.CertPool": {
		canonicalName: "certPool",
		expectedTail:  "CertPool",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "certPool" || strings.HasSuffix(name, "CertPool")
		},
	},
	"pgxpool.Pool": {
		canonicalName: "pool",
		expectedTail:  "Pool",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "pool" || strings.HasSuffix(name, "Pool")
		},
	},
	"sync.Pool": {
		canonicalName: "pool",
		expectedTail:  "Pool",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "pool" || strings.HasSuffix(name, "Pool")
		},
	},
	"big.Int": {
		canonicalName: "bigInt",
		expectedTail:  "BigInt",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return true
		},
	},
	"strings.Builder": {
		canonicalName: "builder",
		expectedTail:  "Builder",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return true
		},
	},
	"uuid.UUID": {
		canonicalName: "id",
		expectedTail:  "ID",
		allowPlural:   false,
		isAllowed: func(name string) bool {
			return name == "id" || strings.HasSuffix(name, "ID") || name == "uuid" || strings.HasSuffix(name, "UUID")
		},
	},
}

// findSpecialTypeRule looks up a naming rule by exact type name or package suffix.
// Examples:
//   - "context.Context" -> matches rule for "context.Context"
//   - "Context"         -> matches rule for "context.Context" via dot suffix ".Context"
//   - "error" / "Error" -> matches rule for "error"
func findSpecialTypeRule(concreteName string) (specialTypeRule, bool) {
	if rule, ok := specialTypeRules[concreteName]; ok {
		return rule, true
	}
	if strings.EqualFold(concreteName, "error") {
		return specialTypeRules["error"], true
	}
	dotSuffix := "." + concreteName
	for key, rule := range specialTypeRules {
		if strings.HasSuffix(key, dotSuffix) {
			return rule, true
		}
	}
	return specialTypeRule{}, false
}

// bareTypeName strips the package prefix from a type name.
// Examples:
//   - "context.Context" -> "Context"
//   - "error"           -> "error"
func bareTypeName(concreteName string) string {
	if idx := strings.LastIndex(concreteName, "."); idx != -1 {
		return concreteName[idx+1:]
	}
	return concreteName
}

// isErrorAllowed checks if an identifier satisfies Go error naming conventions.
// Examples:
//   - Allowed variables: "err", "dbErr", "readErr"
//   - Allowed sentinels: "ErrNotFound", "ErrTimeout" (starts with "Err" followed by capital letter)
//   - Prohibited:        "error", "e", "myError"
func isErrorAllowed(name string) bool {
	if name == "err" || strings.HasSuffix(name, "Err") {
		return true
	}
	// Sentinel errors starting with "Err" (e.g. ErrNotFound)
	return strings.HasPrefix(name, "Err") && len(name) > 3 && unicode.IsUpper(rune(name[3]))
}
