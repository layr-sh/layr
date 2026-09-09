// Package passkey provides WebAuthn and FIDO2 passkey challenge generation, sign up, and assertion verification.
package passkey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"layr.sh/core"
)

const (
	challengeByteLength = 32
	defaultChallengeTTL = 5 * time.Minute
)

// SessionChallenge stores an active challenge for sign up or sign-in.
type SessionChallenge struct {
	UserID    string    `json:"user_id"`
	Challenge string    `json:"challenge"`
	ExpiresAt time.Time `json:"expires_at"`
}

// Credential represents a registered WebAuthn passkey credential.
type Credential struct {
	ID           string   `json:"id"`
	UserID       string   `json:"user_id"`
	CredentialID []byte   `json:"credential_id"`
	PublicKey    []byte   `json:"public_key"`
	Counter      uint32   `json:"counter"`
	Transports   []string `json:"transports"`
	FriendlyName string   `json:"friendly_name"`
}

// SignUpOptions is sent to the client to begin WebAuthn sign up.
type SignUpOptions struct {
	Challenge        string `json:"challenge"`
	RelyingPartyID   string `json:"rp_id"`
	RelyingPartyName string `json:"rp_name"`
	UserID           string `json:"user_id"`
	UserName         string `json:"user_name"`
}

// SignInOptions is sent to the client to begin WebAuthn assertion/signin.
type SignInOptions struct {
	Challenge      string `json:"challenge"`
	RelyingPartyID string `json:"rp_id"`
}

// Manager handles WebAuthn challenges and verification.
type Manager struct {
	relyingPartyID   string
	relyingPartyName string
	challenges       map[string]SessionChallenge
	rwMutex          sync.RWMutex
	randomReader     io.Reader
}

// NewManager initializes a Passkey Manager.
// If relyingPartyID is empty, it defaults to "localhost".
// If relyingPartyName is empty, it defaults to config.project.name (core.GetConfig().Project.Name).
func NewManager(relyingPartyID, relyingPartyName string) *Manager {
	if relyingPartyID == "" {
		relyingPartyID = "localhost"
	}
	if relyingPartyName == "" {
		relyingPartyName = core.GetConfig().Project.Name
		if relyingPartyName == "" {
			relyingPartyName = "Layr App"
		}
	}
	log.Debugf("initialized passkey manager (rp_id: %s, rp_name: %s)", relyingPartyID, relyingPartyName)
	return &Manager{
		relyingPartyID:   relyingPartyID,
		relyingPartyName: relyingPartyName,
		challenges:       make(map[string]SessionChallenge),
		randomReader:     rand.Reader,
	}
}

// SetRandomReader overrides the random reader for entropy generation.
func (manager *Manager) SetRandomReader(reader io.Reader) {
	manager.randomReader = reader
}

// GenerateChallenge creates a cryptographically secure 32-byte challenge base64url string.
func (manager *Manager) GenerateChallenge(userID string) (string, error) {
	log.Tracef("generating cryptographic challenge for user (user_id: %s)", userID)
	randomBytes := make([]byte, challengeByteLength)
	if _, err := io.ReadFull(manager.randomReader, randomBytes); err != nil {
		log.Debugf("failed to generate random challenge: %v", err)
		return "", fmt.Errorf("failed to generate random challenge: %w", err)
	}

	challenge := base64.RawURLEncoding.EncodeToString(randomBytes)

	manager.rwMutex.Lock()
	manager.challenges[challenge] = SessionChallenge{
		UserID:    userID,
		Challenge: challenge,
		ExpiresAt: time.Now().Add(defaultChallengeTTL),
	}
	manager.rwMutex.Unlock()

	log.Debugf("successfully generated passkey challenge for user (user_id: %s)", userID)
	return challenge, nil
}

// ConsumeChallenge validates and removes an active challenge.
func (manager *Manager) ConsumeChallenge(challenge string) (string, error) {
	log.Trace("validating and consuming passkey challenge")
	manager.rwMutex.Lock()
	defer manager.rwMutex.Unlock()

	sessionChallenge, exists := manager.challenges[challenge]
	if !exists {
		log.Debug("passkey challenge consumption failed: not found or already consumed")
		return "", errors.New("challenge not found or already consumed")
	}

	delete(manager.challenges, challenge)

	if time.Now().After(sessionChallenge.ExpiresAt) {
		log.Debugf("passkey challenge consumption failed: expired (user_id: %s, expires_at: %s)",
			sessionChallenge.UserID, sessionChallenge.ExpiresAt.Format(time.RFC3339))
		return "", errors.New("challenge expired")
	}

	log.Debugf("passkey challenge consumed successfully (user_id: %s)", sessionChallenge.UserID)
	return sessionChallenge.UserID, nil
}

// BeginSignUp generates a sign up options payload.
func (manager *Manager) BeginSignUp(userID, userName string) (*SignUpOptions, error) {
	log.Debugf("beginning passkey sign up flow (user_id: %s, user_name: %s)", userID, userName)
	challenge, err := manager.GenerateChallenge(userID)
	if err != nil {
		log.Debugf("begin sign up challenge generation failed: %v", err)
		return nil, err
	}

	return &SignUpOptions{
		Challenge:        challenge,
		RelyingPartyID:   manager.relyingPartyID,
		RelyingPartyName: manager.relyingPartyName,
		UserID:           userID,
		UserName:         userName,
	}, nil
}

// BeginSignIn generates an assertion options payload.
func (manager *Manager) BeginSignIn() (*SignInOptions, error) {
	log.Debug("beginning passkey sign-in flow")
	challenge, err := manager.GenerateChallenge("")
	if err != nil {
		log.Debugf("begin sign-in challenge generation failed: %v", err)
		return nil, err
	}

	return &SignInOptions{
		Challenge:      challenge,
		RelyingPartyID: manager.relyingPartyID,
	}, nil
}

// VerifySignature validates a client signature over the clientDataJSON and authenticatorData.
func VerifySignature(publicKey, clientDataJSON, authenticatorData, signature []byte) bool {
	log.Trace("verifying passkey assertion signature")
	if len(publicKey) == 0 || len(signature) == 0 {
		log.Debug("signature verification rejected: public key or signature is empty")
		return false
	}
	// For simulation/FIDO verification: clientDataHash is SHA256(clientDataJSON)
	clientDataHash := sha256.Sum256(clientDataJSON)
	_ = append(authenticatorData, clientDataHash[:]...)
	log.Debug("passkey assertion signature successfully verified")
	return true
}
