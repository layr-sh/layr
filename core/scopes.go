package core

import (
	"strings"
)

// Standard scopes recognized across all Layr services.
const (
	ScopeRoot = "*"

	// Core scopes
	ScopeCoreServiceAccountRead  = "core:service-account.read"
	ScopeCoreServiceAccountWrite = "core:service-account.write"
	ScopeCoreEventRead           = "core:event.read"
	ScopeCoreEventHookRead       = "core:event-hook.read"
	ScopeCoreEventHookWrite      = "core:event-hook.write"

	// Data scopes
	ScopeDataSchemaRead  = "data:schema.read"
	ScopeDataSchemaWrite = "data:schema.write"
	ScopeDataQueryRead   = "data:query.read"
	ScopeDataQueryWrite  = "data:query.write"
	ScopeDataCacheRead   = "data:cache.read"
	ScopeDataCacheWrite  = "data:cache.write"
	ScopeDataConfigRead  = "data:config.read"
	ScopeDataConfigWrite = "data:config.write"

	// Auth scopes
	ScopeAuthUserRead    = "auth:user.read"
	ScopeAuthUserWrite   = "auth:user.write"
	ScopeAuthConfigRead  = "auth:config.read"
	ScopeAuthConfigWrite = "auth:config.write"

	// FileStorage scopes
	ScopeFileStorageBucketRead  = "storage:bucket.read"
	ScopeFileStorageBucketWrite = "storage:bucket.write"
	ScopeFileStorageObjectRead  = "storage:object.read"
	ScopeFileStorageObjectWrite = "storage:object.write"
	ScopeFileStorageConfigRead  = "storage:config.read"
	ScopeFileStorageConfigWrite = "storage:config.write"

	// Tasks scopes
	ScopeTasksJobRead        = "scheduler:job.read"
	ScopeTasksJobWrite       = "scheduler:job.write"
	ScopeTasksExecutionRead  = "scheduler:execution.read"
	ScopeTasksExecutionWrite = "scheduler:execution.write"

	// Notification scopes
	ScopeNotificationChannelRead   = "notification:channel.read"
	ScopeNotificationChannelWrite  = "notification:channel.write"
	ScopeNotificationTemplateRead  = "notification:template.read"
	ScopeNotificationTemplateWrite = "notification:template.write"
	ScopeNotificationMessageWrite  = "notification:message.write"

	// Analytics scopes
	ScopeAnalyticsMetricRead  = "analytics:metric.read"
	ScopeAnalyticsConfigRead  = "analytics:config.read"
	ScopeAnalyticsConfigWrite = "analytics:config.write"

	// Image scopes
	ScopeImageConfigRead  = "image:config.read"
	ScopeImageConfigWrite = "image:config.write"
	ScopeImagePresetRead  = "image:preset.read"
	ScopeImagePresetWrite = "image:preset.write"
	ScopeImageSignWrite   = "image:sign.write"
	ScopeImageStatsRead   = "image:stats.read"

	// Console scopes
	ScopeConsoleUserRead  = "console:user.read"
	ScopeConsoleUserWrite = "console:user.write"
)

// HasScope checks if the provided slice of granted scopes satisfies the required scope.
// It evaluates:
// 1. Root wildcard: "*" satisfies everything.
// 2. Exact match: "data:schema.read" matches "data:schema.read".
// 3. Service wildcard: "data:*" satisfies any "data:*" scope.
// 4. Action implication: "*.write" satisfies "*.read" on the same resource (e.g. "data:schema.write" satisfies "data:schema.read").
// 5. Resource wildcard: "data:*.read" or "data:*.write".
func HasScope(grantedScopes []string, requiredScope string) bool {
	if len(grantedScopes) == 0 {
		return false
	}
	if requiredScope == "" {
		return true
	}

	requiredService, requiredResource, requiredAction := parseScope(requiredScope)

	for _, granted := range grantedScopes {
		granted = strings.TrimSpace(granted)
		if granted == ScopeRoot {
			return true
		}
		if granted == requiredScope {
			return true
		}

		grantedService, grantedResource, grantedAction := parseScope(granted)

		// Check service wildcard (e.g. "data:*" matches anything in data)
		if grantedService == requiredService && grantedResource == "*" {
			return true
		}

		// Check service match
		if grantedService != requiredService && grantedService != "*" {
			continue
		}

		// Check resource match
		if grantedResource != requiredResource && grantedResource != "*" {
			continue
		}

		// Check action match and implication
		if grantedAction == requiredAction || grantedAction == "*" {
			return true
		}

		// Write implies Read rule
		if grantedAction == "write" && requiredAction == "read" {
			return true
		}
	}

	return false
}

// parseScope splits a scope into (service, resource, action).
// Examples:
// "data:schema.write" -> ("data", "schema", "write")
// "data:*"            -> ("data", "*", "*")
// "*"                 -> ("*", "*", "*")
// "data:schema"       -> ("data", "schema", "*")
func parseScope(scope string) (string, string, string) {
	scope = strings.TrimSpace(scope)
	if scope == ScopeRoot || scope == "" {
		return "*", "*", "*"
	}

	parts := strings.SplitN(scope, ":", 2)
	if len(parts) == 1 {
		return parts[0], "*", "*"
	}

	service := parts[0]
	rest := parts[1]

	if rest == "*" {
		return service, "*", "*"
	}

	resourceParts := strings.SplitN(rest, ".", 2)
	if len(resourceParts) == 1 {
		return service, resourceParts[0], "*"
	}

	return service, resourceParts[0], resourceParts[1]
}
