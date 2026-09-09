package namingclarity

import (
	"strings"
	"unicode"
)

// Prohibited representation suffixes (Hungarian notation).
var prohibitedSuffixes = []string{
	"Boolean",
	"Bool",
	"Str",
	"Int",
}

// Known acronyms to preserve or lowercase consistently in camelCase.
var knownInitialisms = []string{
	"OAuth", "OIDC", "UUID", "HTTP", "JSON", "HTML", "SMTP", "TOTP",
	"REST", "JWT", "URL", "TTL", "API", "SMS", "OTP", "TLS", "TCP",
	"UDP", "SQL", "URI", "UID", "GID", "IP", "ID", "DB",
}

// Bad initialism casing when used as component words (e.g. "Id", "Url", "Ttl", "Jwt", "Sid").
var badInitialisms = map[string]string{
	"Id":  "id",
	"Url": "url",
	"Ttl": "ttl",
	"Jwt": "jwt",
	"Sid": "sid",
}

// Disallowed standalone component words.
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

type specialTypeRule struct {
	canonicalName string
	expectedTail  string
	allowPlural   bool
	isAllowed     func(name string) bool
}

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

// Every imported type rule is standardized with its package prefix (pkg.Type).
// Builtin universe types without package (error) remain bare.
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
}

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

func bareTypeName(concreteName string) string {
	if idx := strings.LastIndex(concreteName, "."); idx != -1 {
		return concreteName[idx+1:]
	}
	return concreteName
}

func isErrorAllowed(name string) bool {
	if name == "err" || strings.HasSuffix(name, "Err") {
		return true
	}
	// Sentinel errors starting with "Err" (e.g. ErrNotFound)
	return strings.HasPrefix(name, "Err") && len(name) > 3 && unicode.IsUpper(rune(name[3]))
}
