package threat

import (
	"context"
	"testing"
)

func TestThreatRiskDatabaseEvaluationIntegration(t *testing.T) {
	ctx := context.Background()
	db, cleanup := setupTestThreatDatabase(t)
	defer cleanup()

	userID := "0191eb70-0000-7000-8000-000000000001"
	_, userInsertErr := db.Exec(ctx, "INSERT INTO auth.users (id, email) VALUES ($1, $2)", userID, "threat@example.com")
	if userInsertErr != nil {
		t.Fatalf("failed to insert test user: %v", userInsertErr)
	}

	// 1. Initial login - no prior sessions
	firstRiskAssessment := EvaluateSignInRisk(ctx, db, nil, userID, "1.2.3.4", "Mozilla/5.0 (Macintosh)", 30)
	if firstRiskAssessment.Score != 0 || firstRiskAssessment.IsNewDevice || firstRiskAssessment.IsNewIP {
		t.Fatalf("expected 0 score on first login without baseline, got score=%d", firstRiskAssessment.Score)
	}

	// Insert baseline session
	insertSessionSQL := `
		INSERT INTO auth.sessions (user_id, refresh_token_hash, ip_address, user_agent, expires_at)
		VALUES ($1, 'dummy-hash', $2::inet, $3, clock_timestamp() + interval '1 hour')
	`
	_, sessionErr := db.Exec(ctx, insertSessionSQL, userID, "1.2.3.4", "Mozilla/5.0 (Macintosh)")
	if sessionErr != nil {
		t.Fatalf("failed to insert baseline session: %v", sessionErr)
	}

	// 2. Known device and known IP
	knownRiskAssessment := EvaluateSignInRisk(ctx, db, nil, userID, "1.2.3.4", "Mozilla/5.0 (Macintosh)", 30)
	if knownRiskAssessment.Score != 0 || knownRiskAssessment.IsNewDevice || knownRiskAssessment.IsNewIP {
		t.Fatalf("expected 0 score for known device/ip, got score=%d", knownRiskAssessment.Score)
	}

	// 3. New device from known IP
	newDeviceRiskAssessment := EvaluateSignInRisk(ctx, db, nil, userID, "1.2.3.4", "Mozilla/5.0 (Windows)", 30)
	if !newDeviceRiskAssessment.IsNewDevice || newDeviceRiskAssessment.IsNewIP || newDeviceRiskAssessment.Score != 30 {
		t.Fatalf("expected new device (score 30), got score=%d, device=%v, ip=%v", newDeviceRiskAssessment.Score, newDeviceRiskAssessment.IsNewDevice, newDeviceRiskAssessment.IsNewIP)
	}

	// 4. Known device from new IP
	newIPRiskAssessment := EvaluateSignInRisk(ctx, db, nil, userID, "9.9.9.9", "Mozilla/5.0 (Macintosh)", 30)
	if newIPRiskAssessment.IsNewDevice || !newIPRiskAssessment.IsNewIP || newIPRiskAssessment.Score != 20 {
		t.Fatalf("expected new IP (score 20), got score=%d, device=%v, ip=%v", newIPRiskAssessment.Score, newIPRiskAssessment.IsNewDevice, newIPRiskAssessment.IsNewIP)
	}

	// 5. New device from new IP (High Risk)
	highRiskAssessment := EvaluateSignInRisk(ctx, db, nil, userID, "9.9.9.9", "Mozilla/5.0 (iPhone)", 30)
	if !highRiskAssessment.IsNewDevice || !highRiskAssessment.IsNewIP || highRiskAssessment.Score != 50 || highRiskAssessment.Level != RiskLevelHigh {
		t.Fatalf("expected high risk (score 50), got score=%d, level=%s", highRiskAssessment.Score, highRiskAssessment.Level)
	}
}

func TestThreatRiskBrokenPoolIntegration(t *testing.T) {
	ctx := context.Background()
	brokenDB := createBrokenThreatPool(t)

	riskAssessment := EvaluateSignInRisk(ctx, brokenDB, nil, "0191eb70-0000-7000-8000-000000000001", "1.2.3.4", "Mozilla/5.0", 30)
	if riskAssessment.Score != 0 || riskAssessment.Level != RiskLevelLow {
		t.Fatalf("expected default low risk assessment on database failure, got: %+v", riskAssessment)
	}
}
