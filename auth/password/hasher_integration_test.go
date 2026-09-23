package password

import (
	"fmt"
	"sync"
	"testing"
	"uuid"
)

type inMemoryUserPasswordStore struct {
	rwMutex        sync.RWMutex
	passwordHashes map[string]string // userID -> passwordHash
}

func newInMemoryUserPasswordStore() *inMemoryUserPasswordStore {
	return &inMemoryUserPasswordStore{
		passwordHashes: make(map[string]string),
	}
}

func (store *inMemoryUserPasswordStore) SavePasswordHash(userID, passwordHash string) {
	store.rwMutex.Lock()
	defer store.rwMutex.Unlock()
	store.passwordHashes[userID] = passwordHash
}

func (store *inMemoryUserPasswordStore) GetPasswordHash(userID string) string {
	store.rwMutex.RLock()
	defer store.rwMutex.RUnlock()
	return store.passwordHashes[userID]
}

func TestPasswordStorageAndRotationIntegration(t *testing.T) {
	hasher := NewHasher()
	store := newInMemoryUserPasswordStore()
	userID := uuid.NewV7().String()

	initialPassword := "MyInitialPassword2026!"
	initialHash, err := hasher.Hash(initialPassword)
	if err != nil {
		t.Fatalf("failed to hash initial password: %v", err)
	}
	store.SavePasswordHash(userID, initialHash)

	// 1. Initial password authentication
	storedHash := store.GetPasswordHash(userID)
	isValid, err := hasher.Verify(initialPassword, storedHash)
	if err != nil || !isValid {
		t.Fatal("expected initial password to verify successfully")
	}

	// 2. Password rotation
	updatedPassword := "MyNewRotatedPassword2026!#$"
	updatedHash, err := hasher.Hash(updatedPassword)
	if err != nil {
		t.Fatalf("failed to hash updated password: %v", err)
	}
	store.SavePasswordHash(userID, updatedHash)

	// 3. Old password must fail on updated hash
	oldPasswordValid, err := hasher.Verify(initialPassword, store.GetPasswordHash(userID))
	if err != nil || oldPasswordValid {
		t.Fatal("expected old password to fail on updated hash")
	}

	// 4. New password must succeed on updated hash
	newPasswordValid, err := hasher.Verify(updatedPassword, store.GetPasswordHash(userID))
	if err != nil || !newPasswordValid {
		t.Fatal("expected new password to verify successfully on updated hash")
	}
}

func TestPasswordSaltUniquenessIntegration(t *testing.T) {
	hasher := NewHasher()
	commonPassword := "UniversalPasswordForTesting123!"

	hashCount := 10
	hashes := make([]string, hashCount)

	for i := 0; i < hashCount; i++ {
		generatedHash, err := hasher.Hash(commonPassword)
		if err != nil {
			t.Fatalf("failed to hash password on index %d: %v", i, err)
		}
		hashes[i] = generatedHash
	}

	// All hashes for the exact same password must be distinct due to unique random salts
	seenHashes := make(map[string]bool)
	for index, currentHash := range hashes {
		t.Run(fmt.Sprintf("Hash_%d", index), func(t *testing.T) {
			if seenHashes[currentHash] {
				t.Fatalf("duplicate hash produced at index %d: %s", index, currentHash)
			}
			seenHashes[currentHash] = true

			// Each distinct hash must still verify the common password
			isValid, err := hasher.Verify(commonPassword, currentHash)
			if err != nil || !isValid {
				t.Fatalf("hash at index %d failed verification: %s", index, currentHash)
			}
		})
	}
}

func TestPasswordConcurrentHashingIntegration(t *testing.T) {
	hasher := NewHasher()
	concurrencyCount := 8

	var waitGroup sync.WaitGroup
	generatedHashes := make([]string, concurrencyCount)
	passwords := make([]string, concurrencyCount)

	for workerIndex := 0; workerIndex < concurrencyCount; workerIndex++ {
		waitGroup.Add(1)
		index := workerIndex
		passwords[index] = fmt.Sprintf("ConcurrentPassword-%d-!#$", index)

		go func() {
			defer waitGroup.Done()
			resultHash, err := hasher.Hash(passwords[index])
			if err != nil {
				t.Errorf("concurrent hashing worker %d failed: %v", index, err)
				return
			}
			generatedHashes[index] = resultHash
		}()
	}
	waitGroup.Wait()

	// Verify each concurrently generated hash against its original password
	for index := 0; index < concurrencyCount; index++ {
		isValid, err := hasher.Verify(passwords[index], generatedHashes[index])
		if err != nil || !isValid {
			t.Fatalf("concurrent hash verification failed for worker %d: %v", index, err)
		}
	}
}
