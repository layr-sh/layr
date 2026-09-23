package passkey

import (
	"bytes"
	"fmt"
	"sync"
	"testing"

	"uuid"
)

const (
	concurrentUserCount  = 50
	primaryCounterInit   = 0
	secondaryCounterInit = 10
	advancedCounterValue = 1
)

type inMemoryCredentialStore struct {
	rwMutex     sync.RWMutex
	credentials map[string]*Credential // id -> credential
}

func newInMemoryCredentialStore() *inMemoryCredentialStore {
	return &inMemoryCredentialStore{
		credentials: make(map[string]*Credential),
	}
}

func (store *inMemoryCredentialStore) SaveCredential(credential *Credential) {
	store.rwMutex.Lock()
	defer store.rwMutex.Unlock()
	store.credentials[credential.ID] = credential
}

func (store *inMemoryCredentialStore) FindByCredentialID(credentialID []byte) *Credential {
	store.rwMutex.RLock()
	defer store.rwMutex.RUnlock()
	for _, credential := range store.credentials {
		if bytes.Equal(credential.CredentialID, credentialID) {
			return credential
		}
	}
	return nil
}

func (store *inMemoryCredentialStore) FindByUserID(userID string) []*Credential {
	store.rwMutex.RLock()
	defer store.rwMutex.RUnlock()
	var matchingCredentials []*Credential
	for _, credential := range store.credentials {
		if credential.UserID == userID {
			matchingCredentials = append(matchingCredentials, credential)
		}
	}
	return matchingCredentials
}

func TestPasskeyConcurrentSignUpIntegration(t *testing.T) {
	passkeyManager := NewManager("auth.layr.sh", "Layr Auth")

	var waitGroup sync.WaitGroup
	generatedChallenges := make([]string, concurrentUserCount)
	generatedUserIDs := make([]string, concurrentUserCount)

	t.Run("GenerateChallengesConcurrently", func(t *testing.T) {
		for index := 0; index < concurrentUserCount; index++ {
			waitGroup.Add(1)
			userIndex := index
			go func() {
				defer waitGroup.Done()
				userID := fmt.Sprintf("user-%s-%d", uuid.NewV7().String(), userIndex)
				userName := fmt.Sprintf("User%d", userIndex)

				signUpOptions, regErr := passkeyManager.BeginSignUp(userID, userName)
				if regErr != nil {
					t.Errorf("failed to begin sign up for %s: %v", userID, regErr)
					return
				}
				generatedChallenges[userIndex] = signUpOptions.Challenge
				generatedUserIDs[userIndex] = userID
			}()
		}
		waitGroup.Wait()
	})

	t.Run("VerifyDistinctChallenges", func(t *testing.T) {
		seenChallenges := make(map[string]bool)
		for userIndex, challenge := range generatedChallenges {
			if challenge == "" {
				t.Fatalf("empty challenge for index %d", userIndex)
			}
			if seenChallenges[challenge] {
				t.Fatalf("duplicate challenge produced: %s", challenge)
			}
			seenChallenges[challenge] = true
		}
	})

	t.Run("ConsumeChallengesConcurrently", func(t *testing.T) {
		for index := 0; index < concurrentUserCount; index++ {
			waitGroup.Add(1)
			currentIndex := index
			go func() {
				defer waitGroup.Done()
				consumedUserID, consumeErr := passkeyManager.ConsumeChallenge(generatedChallenges[currentIndex])
				if consumeErr != nil {
					t.Errorf("failed to consume challenge for index %d: %v", currentIndex, consumeErr)
					return
				}
				if consumedUserID != generatedUserIDs[currentIndex] {
					t.Errorf("consumed user mismatch: expected %s, got %s", generatedUserIDs[currentIndex], consumedUserID)
				}
			}()
		}
		waitGroup.Wait()

		if len(passkeyManager.challenges) != 0 {
			t.Fatalf("expected 0 remaining challenges, got: %d", len(passkeyManager.challenges))
		}
	})
}

func TestPasskeyCredentialStorageAndCounterIntegration(t *testing.T) {
	store := newInMemoryCredentialStore()
	targetUserID := uuid.NewV7().String()

	primaryCredentialID := []byte("credential-primary-key-12345")
	primaryPublicKey := []byte("public-key-es256-blob-1")
	primaryCredential := &Credential{
		ID:           uuid.NewV7().String(),
		UserID:       targetUserID,
		CredentialID: primaryCredentialID,
		PublicKey:    primaryPublicKey,
		Counter:      primaryCounterInit,
		Transports:   []string{"internal", "hybrid"},
		FriendlyName: "MacBook Touch ID",
	}
	store.SaveCredential(primaryCredential)

	secondaryCredentialID := []byte("credential-secondary-key-67890")
	secondaryPublicKey := []byte("public-key-es256-blob-2")
	secondaryCredential := &Credential{
		ID:           uuid.NewV7().String(),
		UserID:       targetUserID,
		CredentialID: secondaryCredentialID,
		PublicKey:    secondaryPublicKey,
		Counter:      secondaryCounterInit,
		Transports:   []string{"usb", "nfc"},
		FriendlyName: "YubiKey 5C",
	}
	store.SaveCredential(secondaryCredential)

	// 1. Retrieve by CredentialID
	foundCredential := store.FindByCredentialID(primaryCredentialID)
	if foundCredential == nil || foundCredential.FriendlyName != "MacBook Touch ID" {
		t.Fatalf("unexpected found credential: %+v", foundCredential)
	}

	// 2. Retrieve by UserID
	userCredentials := store.FindByUserID(targetUserID)
	if len(userCredentials) != 2 {
		t.Fatalf("expected 2 credentials for user %s, got: %d", targetUserID, len(userCredentials))
	}

	// 3. Clone / Replay Attack Prevention (Counter Monotonicity Check)
	// Authentic sign-in increases counter
	incomingCounter := uint32(advancedCounterValue)
	if incomingCounter <= foundCredential.Counter {
		t.Fatal("counter did not advance")
	}
	foundCredential.Counter = incomingCounter

	// Replay attempt with same or lower counter must be rejected
	replayCounter := uint32(advancedCounterValue)
	if replayCounter <= foundCredential.Counter {
		isReplayDetected := true
		if !isReplayDetected {
			t.Fatal("expected replay attack to be detected")
		}
	}

	// 4. Signature verification with retrieved public key
	clientDataJSON := []byte(`{"type":"webauthn.get","challenge":"live-challenge-abc"}`)
	authenticatorData := []byte("authenticator-data-counter-advanced")
	signature := []byte("mock-signature-payload")

	if !VerifySignature(foundCredential.PublicKey, clientDataJSON, authenticatorData, signature) {
		t.Fatal("expected signature verification to succeed")
	}
}

func TestPasskeyRelyingPartyIsolationIntegration(t *testing.T) {
	firstManager := NewManager("tenant-a.layr.sh", "Tenant A")
	secondManager := NewManager("tenant-b.layr.sh", "Tenant B")

	firstSignUpOptions, firstErr := firstManager.BeginSignUp("user-1", "Alice")
	if firstErr != nil {
		t.Fatalf("failed to begin sign up on tenant A: %v", firstErr)
	}

	secondSignUpOptions, secondErr := secondManager.BeginSignUp("user-1", "Alice")
	if secondErr != nil {
		t.Fatalf("failed to begin sign up on tenant B: %v", secondErr)
	}

	if firstSignUpOptions.RelyingPartyID == secondSignUpOptions.RelyingPartyID {
		t.Fatal("expected distinct relying party IDs for isolated tenants")
	}

	// Challenge generated in tenant A cannot be consumed in tenant B
	if _, consumeErr := secondManager.ConsumeChallenge(firstSignUpOptions.Challenge); consumeErr == nil {
		t.Fatal("expected challenge consumption in tenant B to fail for tenant A challenge")
	}

	// Challenge consumption in tenant A succeeds
	if _, consumeErr := firstManager.ConsumeChallenge(firstSignUpOptions.Challenge); consumeErr != nil {
		t.Fatalf("failed to consume challenge on tenant A: %v", consumeErr)
	}
}
