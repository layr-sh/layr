package threat

import (
	"context"
	"testing"
	"time"

	"layr.sh/core"
)

func TestThreatRiskEvaluationUnit(t *testing.T) {
	ctx := context.Background()

	// 1. Nil DB, Nil KVStore
	riskAssessment, err := EvaluateSignInRisk(ctx, nil, nil, "user-1", "1.2.3.4", "Mozilla/5.0", 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if riskAssessment.Level != RiskLevelLow || riskAssessment.Score != 0 {
		t.Fatalf("expected low risk with 0 score, got level=%s, score=%d", riskAssessment.Level, riskAssessment.Score)
	}

	// 2. Medium risk through failed attempts
	driver := newInMemoryKVDriver()
	kvStore := core.NewKVStoreFromDriver(driver)

	// Record 3 failed attempts on IP
	_, _ = RecordFailedAttempt(ctx, kvStore, "1.2.3.4", time.Minute)
	_, _ = RecordFailedAttempt(ctx, kvStore, "1.2.3.4", time.Minute)
	_, _ = RecordFailedAttempt(ctx, kvStore, "1.2.3.4", time.Minute)

	medRiskAssessment, medErr := EvaluateSignInRisk(ctx, nil, kvStore, "user-1", "1.2.3.4", "Mozilla/5.0", 30)
	if medErr != nil {
		t.Fatalf("unexpected error: %v", medErr)
	}
	if medRiskAssessment.Level != RiskLevelMedium || medRiskAssessment.Score != 30 {
		t.Fatalf("expected medium risk with 30 score, got level=%s, score=%d", medRiskAssessment.Level, medRiskAssessment.Score)
	}

	// 3. High risk through 5 failed attempts (IP + UserID)
	_, _ = RecordFailedAttempt(ctx, kvStore, "user-1", time.Minute)
	_, _ = RecordFailedAttempt(ctx, kvStore, "user-1", time.Minute)

	highRiskAssessment, highErr := EvaluateSignInRisk(ctx, nil, kvStore, "user-1", "1.2.3.4", "Mozilla/5.0", 30)
	if highErr != nil {
		t.Fatalf("unexpected error: %v", highErr)
	}
	if highRiskAssessment.Level != RiskLevelHigh || highRiskAssessment.Score != 50 {
		t.Fatalf("expected high risk with 50 score, got level=%s, score=%d", highRiskAssessment.Level, highRiskAssessment.Score)
	}
}

func TestThreatRiskFailedAttemptsUnit(t *testing.T) {
	ctx := context.Background()

	// 1. Nil KVStore or empty identifier
	count, recordErr := RecordFailedAttempt(ctx, nil, "user-1", 0)
	if recordErr != nil || count != 0 {
		t.Fatalf("expected (0, nil), got (%d, %v)", count, recordErr)
	}

	countEmpty, emptyRecordErr := RecordFailedAttempt(ctx, core.NewKVStoreFromDriver(newInMemoryKVDriver()), "  ", 0)
	if emptyRecordErr != nil || countEmpty != 0 {
		t.Fatalf("expected (0, nil), got (%d, %v)", countEmpty, emptyRecordErr)
	}

	nilResetErr := ResetFailedAttempts(ctx, nil, "user-1")
	if nilResetErr != nil {
		t.Fatalf("expected nil error on nil kvStore reset, got %v", nilResetErr)
	}

	emptyResetErr := ResetFailedAttempts(ctx, core.NewKVStoreFromDriver(newInMemoryKVDriver()), "  ")
	if emptyResetErr != nil {
		t.Fatalf("expected nil error on empty identifier reset, got %v", emptyResetErr)
	}

	getNil := GetFailedAttempts(ctx, nil, "user-1")
	if getNil != 0 {
		t.Fatalf("expected 0 for nil kvStore, got %d", getNil)
	}

	getEmpty := GetFailedAttempts(ctx, core.NewKVStoreFromDriver(newInMemoryKVDriver()), "  ")
	if getEmpty != 0 {
		t.Fatalf("expected 0 for empty identifier, got %d", getEmpty)
	}

	// 2. Normal flow with in-memory driver
	driver := newInMemoryKVDriver()
	kvStore := core.NewKVStoreFromDriver(driver)

	count1, firstErr := RecordFailedAttempt(ctx, kvStore, "user-1", 0)
	if firstErr != nil || count1 != 1 {
		t.Fatalf("expected count 1, got (%d, %v)", count1, firstErr)
	}

	count2, secondErr := RecordFailedAttempt(ctx, kvStore, "user-1", -1)
	if secondErr != nil || count2 != 2 {
		t.Fatalf("expected count 2, got (%d, %v)", count2, secondErr)
	}

	fetchedCount := GetFailedAttempts(ctx, kvStore, "user-1")
	if fetchedCount != 2 {
		t.Fatalf("expected 2 fetched, got %d", fetchedCount)
	}

	// Reset
	if err := ResetFailedAttempts(ctx, kvStore, "user-1"); err != nil {
		t.Fatalf("unexpected reset error: %v", err)
	}

	// Reset again (key not found - should be absorbed)
	if err := ResetFailedAttempts(ctx, kvStore, "user-1"); err != nil {
		t.Fatalf("expected absorbed key not found error, got %v", err)
	}

	afterReset := GetFailedAttempts(ctx, kvStore, "user-1")
	if afterReset != 0 {
		t.Fatalf("expected 0 after reset, got %d", afterReset)
	}

	// 3. Driver errors
	driver.shouldFailIncrement = true
	if _, err := RecordFailedAttempt(ctx, kvStore, "user-1", time.Minute); err == nil {
		t.Fatal("expected record failure error")
	}

	driver.shouldFailDelete = true
	if err := ResetFailedAttempts(ctx, kvStore, "user-1"); err == nil {
		t.Fatal("expected reset failure error")
	}

	driver.shouldFailGet = true
	if count := GetFailedAttempts(ctx, kvStore, "user-1"); count != 0 {
		t.Fatalf("expected 0 on get error, got %d", count)
	}

	// Non-numeric value in KVStore
	driver.shouldFailGet = false
	_ = kvStore.Set(ctx, "auth:threat:failed:bad-data", "not_a_number", time.Minute)
	if count := GetFailedAttempts(ctx, kvStore, "bad-data"); count != 0 {
		t.Fatalf("expected 0 on parse error, got %d", count)
	}
}
