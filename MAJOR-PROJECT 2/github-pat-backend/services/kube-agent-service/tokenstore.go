package kubeagentservice

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// ─── Phase 2: In-Memory Token Store ──────────────────────────────────────────
// Tokens are short-lived (15 min), one-time-use, and NEVER stored in the DB.
// They map a cryptographic token string → { OrgID, ContextName }.

const tokenTTL = 25 * time.Minute

// TokenSessionInfo holds the context associated with a Phase 2 init token.
type TokenSessionInfo struct {
	OrgID       string
	ContextName string
	CreatedAt   time.Time
}

// tokenStoreMap is the thread-safe in-memory token store.
var (
	tokenStoreMap = make(map[string]TokenSessionInfo)
	tokenStoreMu  sync.RWMutex
)

// GeneratePhase2Token creates a cryptographically random 64-character hex token.
func GeneratePhase2Token() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}

// StorePhase2Token saves a token→session mapping in memory with a 15-minute TTL.
func StorePhase2Token(token string, info TokenSessionInfo) {
	tokenStoreMu.Lock()
	info.CreatedAt = time.Now()
	tokenStoreMap[token] = info
	tokenStoreMu.Unlock()

	// Auto-expire after TTL
	go func() {
		time.Sleep(tokenTTL)
		tokenStoreMu.Lock()
		delete(tokenStoreMap, token)
		tokenStoreMu.Unlock()
	}()
}

// LookupPhase2Token retrieves session info for a token. Returns error if not found or expired.
func LookupPhase2Token(token string) (TokenSessionInfo, error) {
	tokenStoreMu.RLock()
	defer tokenStoreMu.RUnlock()

	info, ok := tokenStoreMap[token]
	if !ok {
		return TokenSessionInfo{}, fmt.Errorf("token not found or expired")
	}

	// Double-check TTL (in case goroutine hasn't fired yet)
	if time.Since(info.CreatedAt) > tokenTTL {
		return TokenSessionInfo{}, fmt.Errorf("token expired")
	}

	return info, nil
}

// DeletePhase2Token removes a token from memory (one-time use invalidation).
func DeletePhase2Token(token string) {
	tokenStoreMu.Lock()
	delete(tokenStoreMap, token)
	tokenStoreMu.Unlock()
}

// ─── Phase 3: In-Memory Persistent Session Store ─────────────────────────────
// These tokens are long-lived (do not expire automatically) and manage the linked Websocket channels.

var (
	activeSessionMap = make(map[string]TokenSessionInfo)
	activeSessionMu  sync.RWMutex
)

// StoreSessionToken saves a persistent DB-less token.
func StoreSessionToken(token string, info TokenSessionInfo) {
	activeSessionMu.Lock()
	info.CreatedAt = time.Now()
	activeSessionMap[token] = info
	activeSessionMu.Unlock()
}

// LookupSessionToken retrieves session info for an active token.
func LookupSessionToken(token string) (TokenSessionInfo, error) {
	activeSessionMu.RLock()
	defer activeSessionMu.RUnlock()

	info, ok := activeSessionMap[token]
	if !ok {
		return TokenSessionInfo{}, fmt.Errorf("session token not found")
	}

	return info, nil
}

// GetSessionTokenByOrg finds an active session token for a given orgID.
func GetSessionTokenByOrg(orgID string) (string, error) {
	activeSessionMu.RLock()
	defer activeSessionMu.RUnlock()

	var latestToken string
	var latestTime time.Time
	found := false

	for token, info := range activeSessionMap {
		if info.OrgID == orgID {
			if !found || info.CreatedAt.After(latestTime) {
				latestTime = info.CreatedAt
				latestToken = token
				found = true
			}
		}
	}
	
	if !found {
		return "", fmt.Errorf("no active session found for orgID: %s", orgID)
	}
	
	return latestToken, nil
}
