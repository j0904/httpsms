package handlers

import (
	"context"
	"testing"
	"time"

	"github.com/NdoleStudio/httpsms/pkg/cache"
	"github.com/NdoleStudio/httpsms/pkg/telemetry"
	ttlCache "github.com/patrickmn/go-cache"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestCacheNonceStore() NonceStore {
	logger := testBufferedLogger()
	tracer := telemetry.NewOtelLogger("", logger)
	return NewCacheNonceStore(cache.NewMemoryCache(tracer, ttlCache.New(time.Minute, 2*time.Minute)))
}

func TestCacheNonceStoreSingleUse(t *testing.T) {
	store := newTestCacheNonceStore()
	ctx := context.Background()

	id, err := store.Issue(ctx, "did:key:zAlice", "nonce1")
	require.NoError(t, err)

	matched, err := store.Match(ctx, id, "did:key:zAlice", "nonce1")
	require.NoError(t, err)
	assert.True(t, matched)

	// a wrong did does not match and does not consume
	matched, _ = store.Match(ctx, id, "did:key:zMallory", "nonce1")
	assert.False(t, matched)

	// a failed signature attempt (Match only, no Consume) leaves it live
	matched, _ = store.Match(ctx, id, "did:key:zAlice", "nonce1")
	assert.True(t, matched)

	consumed, err := store.Consume(ctx, id, "did:key:zAlice", "nonce1")
	require.NoError(t, err)
	assert.True(t, consumed)

	// replay is rejected
	consumed, _ = store.Consume(ctx, id, "did:key:zAlice", "nonce1")
	assert.False(t, consumed)
	matched, _ = store.Match(ctx, id, "did:key:zAlice", "nonce1")
	assert.False(t, matched)
}

func TestCacheNonceStoreConsumeMismatch(t *testing.T) {
	store := newTestCacheNonceStore()
	ctx := context.Background()

	id, err := store.Issue(ctx, "did:key:zAlice", "nonce1")
	require.NoError(t, err)

	consumed, err := store.Consume(ctx, id, "did:key:zMallory", "nonce1")
	require.NoError(t, err)
	assert.False(t, consumed)
}
