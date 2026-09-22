package threat

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"layr.sh/core"
)

// RiskLevel constants.
const (
	RiskLevelLow    = "low"
	RiskLevelMedium = "medium"
	RiskLevelHigh   = "high"

	defaultRiskWindowDays = 30
	highRiskThreshold     = 50
	mediumRiskThreshold   = 25

	failedAttemptsMediumThreshold   = 3
	failedAttemptsCriticalThreshold = 5

	newDeviceRiskScore               = 30
	newIPRiskScore                   = 20
	excessiveFailedAttemptsRiskScore = 30
	criticalFailedAttemptsRiskScore  = 20

	failedAttemptKeyPrefix   = "auth:threat:failed:"
	defaultAttemptWindowTime = 15 * time.Minute
)

// RiskAssessment represents the evaluated risk factors for an authentication attempt.
type RiskAssessment struct {
	Score          int      `json:"score"`
	Level          string   `json:"level"`
	IsNewDevice    bool     `json:"is_new_device"`
	IsNewIP        bool     `json:"is_new_ip"`
	FailedAttempts int64    `json:"failed_attempts"`
	Reasons        []string `json:"reasons"`
}

// EvaluateSignInRisk evaluates whether an authentication attempt presents anomalous risk.
func EvaluateSignInRisk(ctx context.Context, db *core.DatabasePool, kvStore *core.KVStore, userID string, clientIP string, userAgent string, windowDays int) *RiskAssessment {
	if windowDays <= 0 {
		windowDays = defaultRiskWindowDays
	}

	riskAssessment := &RiskAssessment{
		Score:   0,
		Level:   RiskLevelLow,
		Reasons: make([]string, 0),
	}

	cleanIP := strings.TrimSpace(clientIP)
	cleanUA := strings.TrimSpace(userAgent)

	if db != nil && userID != "" {
		query := `
			SELECT COUNT(*),
			       COUNT(CASE WHEN user_agent = $1 THEN 1 END),
			       COUNT(CASE WHEN host(ip_address) = $2 THEN 1 END)
			FROM auth.sessions
			WHERE user_id = $3 AND created_at >= clock_timestamp() - ($4 || ' days')::INTERVAL
		`
		var totalSessions, matchingUACount, matchingIPCount int64
		err := db.QueryRow(ctx, query, cleanUA, cleanIP, userID, strconv.Itoa(windowDays)).Scan(
			&totalSessions, &matchingUACount, &matchingIPCount,
		)
		if err != nil {
			log.Warnf("failed to evaluate session history for user %s: %v", userID, err)
		} else if totalSessions > 0 {
			if matchingUACount == 0 && cleanUA != "" {
				riskAssessment.IsNewDevice = true
				riskAssessment.Score += newDeviceRiskScore
				riskAssessment.Reasons = append(riskAssessment.Reasons, "new_device")
			}
			if matchingIPCount == 0 && cleanIP != "" {
				riskAssessment.IsNewIP = true
				riskAssessment.Score += newIPRiskScore
				riskAssessment.Reasons = append(riskAssessment.Reasons, "new_ip")
			}
		}
	}

	var totalFailedAttempts int64
	if kvStore != nil {
		if cleanIP != "" {
			totalFailedAttempts += GetFailedAttempts(ctx, kvStore, cleanIP)
		}
		if userID != "" {
			totalFailedAttempts += GetFailedAttempts(ctx, kvStore, userID)
		}
	}

	riskAssessment.FailedAttempts = totalFailedAttempts
	if totalFailedAttempts >= failedAttemptsMediumThreshold {
		riskAssessment.Score += excessiveFailedAttemptsRiskScore
		riskAssessment.Reasons = append(riskAssessment.Reasons, "excessive_failed_attempts")
	}
	if totalFailedAttempts >= failedAttemptsCriticalThreshold {
		riskAssessment.Score += criticalFailedAttemptsRiskScore
		riskAssessment.Reasons = append(riskAssessment.Reasons, "critical_failed_attempts")
	}

	if riskAssessment.Score >= highRiskThreshold {
		riskAssessment.Level = RiskLevelHigh
	} else if riskAssessment.Score >= mediumRiskThreshold {
		riskAssessment.Level = RiskLevelMedium
	} else {
		riskAssessment.Level = RiskLevelLow
	}

	return riskAssessment
}

// RecordFailedAttempt records an authentication failure for the given identifier (IP or user ID).
func RecordFailedAttempt(ctx context.Context, kvStore *core.KVStore, identifier string, windowDuration time.Duration) (int64, error) {
	if kvStore == nil || strings.TrimSpace(identifier) == "" {
		return 0, nil
	}

	if windowDuration <= 0 {
		windowDuration = defaultAttemptWindowTime
	}

	key := failedAttemptKeyPrefix + strings.TrimSpace(identifier)
	count, err := kvStore.Increment(ctx, key, windowDuration)
	if err != nil {
		return 0, fmt.Errorf("failed to record failed attempt in KVStore: %w", err)
	}
	return count, nil
}

// ResetFailedAttempts clears the failure counter for the identifier after successful authentication.
func ResetFailedAttempts(ctx context.Context, kvStore *core.KVStore, identifier string) error {
	if kvStore == nil || strings.TrimSpace(identifier) == "" {
		return nil
	}

	key := failedAttemptKeyPrefix + strings.TrimSpace(identifier)
	if err := kvStore.Delete(ctx, key); err != nil && !errors.Is(err, core.ErrKVStoreKeyNotFound) {
		return fmt.Errorf("failed to reset failed attempts in KVStore: %w", err)
	}
	return nil
}

// GetFailedAttempts retrieves the current failure count for the identifier.
func GetFailedAttempts(ctx context.Context, kvStore *core.KVStore, identifier string) int64 {
	if kvStore == nil || strings.TrimSpace(identifier) == "" {
		return 0
	}

	key := failedAttemptKeyPrefix + strings.TrimSpace(identifier)
	content, err := kvStore.Get(ctx, key)
	if err != nil || content == "" {
		return 0
	}

	count, parseErr := strconv.ParseInt(strings.TrimSpace(content), 10, 64)
	if parseErr != nil {
		return 0
	}
	return count
}
