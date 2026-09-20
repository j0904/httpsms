package handlers

import (
	"context"
	"sync"
	"time"

	"github.com/NdoleStudio/httpsms/pkg/cache"
	"github.com/google/uuid"
)

// NonceExpiry is how long an issued challenge remains valid.
const NonceExpiry = 5 * time.Minute

// NonceStore issues one-time DID auth challenges and consumes them on success.
type NonceStore interface {
	// Issue stores a challenge and returns its id.
	Issue(ctx context.Context, didValue string, nonce string) (id string, err error)
	// Match reports whether a live challenge matches did+nonce without
	// consuming it, so a failed signature attempt does not burn the challenge.
	Match(ctx context.Context, id string, didValue string, nonce string) (bool, error)
	// Consume atomically removes the challenge. It returns false when the
	// challenge was already consumed, so a replay cannot succeed.
	Consume(ctx context.Context, id string, didValue string, nonce string) (bool, error)
}

// nonceEntry is a single issued challenge.
type nonceEntry struct {
	did       string
	nonce     string
	expiresAt time.Time
}

// memoryNonceStore is the single-process NonceStore (tests, single replica).
type memoryNonceStore struct {
	mu     sync.RWMutex
	challs map[string]*nonceEntry
}

func newMemoryNonceStore() *memoryNonceStore {
	return &memoryNonceStore{challs: map[string]*nonceEntry{}}
}

// Issue creates and stores a new challenge, returning its id.
func (store *memoryNonceStore) Issue(_ context.Context, didValue string, nonce string) (string, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	now := time.Now()
	for id, entry := range store.challs {
		if now.After(entry.expiresAt) {
			delete(store.challs, id)
		}
	}

	id := uuid.NewString()
	store.challs[id] = &nonceEntry{did: didValue, nonce: nonce, expiresAt: now.Add(NonceExpiry)}
	return id, nil
}

// Match reports whether a live challenge matches did+nonce without consuming it.
func (store *memoryNonceStore) Match(_ context.Context, id string, didValue string, nonce string) (bool, error) {
	store.mu.RLock()
	defer store.mu.RUnlock()

	entry, ok := store.challs[id]
	if !ok || entry.did != didValue || entry.nonce != nonce {
		return false, nil
	}
	return time.Now().Before(entry.expiresAt), nil
}

// Consume deletes a challenge, reporting whether it was still live and matching.
func (store *memoryNonceStore) Consume(_ context.Context, id string, didValue string, nonce string) (bool, error) {
	store.mu.Lock()
	defer store.mu.Unlock()

	entry, ok := store.challs[id]
	if !ok {
		return false, nil
	}
	delete(store.challs, id)

	if entry.did != didValue || entry.nonce != nonce || time.Now().After(entry.expiresAt) {
		return false, nil
	}
	return true, nil
}

// cacheNonceStore keeps challenges in the shared cache (Redis), so a challenge
// issued by one replica can be consumed by another.
type cacheNonceStore struct {
	cache cache.Cache
}

const nonceKeyPrefix = "did-auth:nonce:"

// NewCacheNonceStore creates a NonceStore backed by the shared cache.
func NewCacheNonceStore(c cache.Cache) NonceStore {
	return &cacheNonceStore{cache: c}
}

// Issue stores a challenge in the cache under a fresh id.
func (store *cacheNonceStore) Issue(ctx context.Context, didValue string, nonce string) (string, error) {
	id := uuid.NewString()
	if err := store.cache.Set(ctx, nonceKeyPrefix+id, didValue+"\n"+nonce, NonceExpiry); err != nil {
		return "", err
	}
	return id, nil
}

// Match reads the challenge without consuming it.
func (store *cacheNonceStore) Match(ctx context.Context, id string, didValue string, nonce string) (bool, error) {
	value, err := store.cache.Get(ctx, nonceKeyPrefix+id)
	if err != nil {
		return false, nil // absent or expired — not a match
	}
	return value == didValue+"\n"+nonce, nil
}

// Consume atomically removes the challenge; a concurrent verifier that already
// consumed it gets false and the replay is rejected.
func (store *cacheNonceStore) Consume(ctx context.Context, id string, didValue string, nonce string) (bool, error) {
	value, err := store.cache.GetDel(ctx, nonceKeyPrefix+id)
	if err != nil {
		return false, nil // already gone
	}
	return value == didValue+"\n"+nonce, nil
}
