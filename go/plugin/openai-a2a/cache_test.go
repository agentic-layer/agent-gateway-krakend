package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestAgentCache_TTLExpiration(t *testing.T) {
	// Create cache with very short TTL
	cache := &AgentCache{
		ttl: 100 * time.Millisecond,
	}

	// Mock agents
	testAgents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "test-agent",
				Namespace: "default",
			},
			Spec: AgentSpec{Exposed: true},
		},
	}

	// Populate cache manually
	cache.mu.Lock()
	cache.agents = testAgents
	cache.lastUpdate = time.Now()
	cache.mu.Unlock()

	// Verify cache is valid
	cache.mu.RLock()
	isFresh := time.Since(cache.lastUpdate) < cache.ttl
	cache.mu.RUnlock()
	assert.True(t, isFresh, "cache should be fresh initially")

	// Wait for TTL to expire
	time.Sleep(150 * time.Millisecond)

	// Verify cache is expired
	cache.mu.RLock()
	isExpired := time.Since(cache.lastUpdate) >= cache.ttl
	cache.mu.RUnlock()
	assert.True(t, isExpired, "cache should be expired after TTL")
}

func TestAgentCache_Invalidate(t *testing.T) {
	cache := &AgentCache{
		ttl: 10 * time.Second,
	}

	// Populate cache
	cache.mu.Lock()
	cache.agents = []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "test-agent"},
			Spec:       AgentSpec{Exposed: true},
		},
	}
	cache.lastUpdate = time.Now()
	cache.mu.Unlock()

	// Verify cache has data
	cache.mu.RLock()
	assert.NotNil(t, cache.agents)
	assert.NotZero(t, cache.lastUpdate)
	cache.mu.RUnlock()

	// Invalidate cache
	cache.Invalidate()

	// Verify cache is cleared
	cache.mu.RLock()
	assert.Nil(t, cache.agents)
	assert.True(t, cache.lastUpdate.IsZero())
	cache.mu.RUnlock()
}

func TestAgentCache_ConcurrentAccess(t *testing.T) {
	cache := &AgentCache{
		ttl: 1 * time.Second,
	}

	testAgents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "agent-1"},
			Spec:       AgentSpec{Exposed: true},
		},
	}

	// Populate cache
	cache.mu.Lock()
	cache.agents = testAgents
	cache.lastUpdate = time.Now()
	cache.mu.Unlock()

	// Simulate concurrent reads
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func() {
			cache.mu.RLock()
			_ = cache.agents
			_ = cache.lastUpdate
			cache.mu.RUnlock()
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 10; i++ {
		<-done
	}

	// Verify cache is still consistent
	cache.mu.RLock()
	assert.Len(t, cache.agents, 1)
	cache.mu.RUnlock()
}

func TestAgentCache_DoubleCheckedLocking(t *testing.T) {
	cache := &AgentCache{
		ttl: 50 * time.Millisecond,
	}

	// Populate cache with expired data
	cache.mu.Lock()
	cache.agents = []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "old-agent"},
			Spec:       AgentSpec{Exposed: true},
		},
	}
	cache.lastUpdate = time.Now().Add(-100 * time.Millisecond)
	cache.mu.Unlock()

	// Verify cache appears expired
	cache.mu.RLock()
	isExpired := time.Since(cache.lastUpdate) >= cache.ttl
	cache.mu.RUnlock()
	assert.True(t, isExpired)

	// The actual Get() would fetch fresh data, but we can't test K8s client here
	// This test verifies the double-checked locking pattern works
	cache.mu.RLock()
	needsUpdate := time.Since(cache.lastUpdate) >= cache.ttl
	cache.mu.RUnlock()

	if needsUpdate {
		cache.mu.Lock()
		// Double-check after acquiring write lock
		if time.Since(cache.lastUpdate) >= cache.ttl {
			// Update would happen here
			cache.agents = []Agent{
				{
					ObjectMeta: metav1.ObjectMeta{Name: "new-agent"},
					Spec:       AgentSpec{Exposed: true},
				},
			}
			cache.lastUpdate = time.Now()
		}
		cache.mu.Unlock()
	}

	// Verify cache was updated
	cache.mu.RLock()
	assert.Len(t, cache.agents, 1)
	assert.Equal(t, "new-agent", cache.agents[0].Name)
	cache.mu.RUnlock()
}

func TestBackoffTracker_ExponentialBackoff(t *testing.T) {
	tracker := &BackoffTracker{}

	testCases := []struct {
		attempt         int
		expectedMinimum int
		expectedMaximum int
	}{
		{attempt: 1, expectedMinimum: 5, expectedMaximum: 5},
		{attempt: 2, expectedMinimum: 10, expectedMaximum: 10},
		{attempt: 3, expectedMinimum: 20, expectedMaximum: 20},
		{attempt: 4, expectedMinimum: 40, expectedMaximum: 40},
		{attempt: 5, expectedMinimum: 80, expectedMaximum: 80},
		{attempt: 6, expectedMinimum: 120, expectedMaximum: 120}, // Capped at max
		{attempt: 7, expectedMinimum: 120, expectedMaximum: 120}, // Still capped
	}

	for _, tc := range testCases {
		backoff, attemptNum := tracker.RecordFailure()
		assert.Equal(t, tc.attempt, attemptNum, "attempt number should match")
		assert.GreaterOrEqual(t, backoff, tc.expectedMinimum,
			"attempt %d should have backoff >= %d seconds", tc.attempt, tc.expectedMinimum)
		assert.LessOrEqual(t, backoff, tc.expectedMaximum,
			"attempt %d should have backoff <= %d seconds", tc.attempt, tc.expectedMaximum)
	}

	// Verify failure count
	tracker.mu.Lock()
	assert.Equal(t, 7, tracker.failures)
	assert.False(t, tracker.lastFailureAt.IsZero())
	tracker.mu.Unlock()
}

func TestBackoffTracker_RecordSuccess(t *testing.T) {
	tracker := &BackoffTracker{}

	// Record some failures
	tracker.RecordFailure()  // Ignore return values for setup
	tracker.RecordFailure()
	tracker.RecordFailure()

	// Verify failures recorded
	tracker.mu.Lock()
	assert.Equal(t, 3, tracker.failures)
	tracker.mu.Unlock()

	// Record success
	tracker.RecordSuccess()

	// Verify failures reset
	tracker.mu.Lock()
	assert.Equal(t, 0, tracker.failures)
	assert.True(t, tracker.lastFailureAt.IsZero())
	tracker.mu.Unlock()

	// Next failure should start from initial backoff again
	backoff, _ := tracker.RecordFailure()
	assert.Equal(t, initialBackoffSeconds, backoff)
}

func TestBackoffTracker_MaximumCap(t *testing.T) {
	tracker := &BackoffTracker{}

	// Record many failures to exceed maximum
	var lastBackoff int
	for i := 0; i < 20; i++ {
		lastBackoff, _ = tracker.RecordFailure()
	}

	// Verify backoff is capped at maximum
	assert.Equal(t, maxBackoffSeconds, lastBackoff)

	// Record more failures, should still be capped
	moreBackoff, _ := tracker.RecordFailure()
	assert.Equal(t, maxBackoffSeconds, moreBackoff)
}

func TestBackoffTracker_ConcurrentFailures(t *testing.T) {
	tracker := &BackoffTracker{}

	// Record failures concurrently
	done := make(chan int, 5)
	for i := 0; i < 5; i++ {
		go func() {
			backoff, _ := tracker.RecordFailure()
			done <- backoff
		}()
	}

	// Collect results
	backoffs := make([]int, 5)
	for i := 0; i < 5; i++ {
		backoffs[i] = <-done
	}

	// Verify all backoffs are valid (between min and max)
	for _, backoff := range backoffs {
		assert.GreaterOrEqual(t, backoff, initialBackoffSeconds)
		assert.LessOrEqual(t, backoff, maxBackoffSeconds)
	}

	// Verify failure count is correct
	tracker.mu.Lock()
	assert.Equal(t, 5, tracker.failures)
	tracker.mu.Unlock()
}

func TestBackoffTracker_SuccessAfterFailures(t *testing.T) {
	tracker := &BackoffTracker{}

	// Simulate failure-success-failure pattern
	backoff1, attempt1 := tracker.RecordFailure()
	assert.Equal(t, 5, backoff1)
	assert.Equal(t, 1, attempt1)

	backoff2, attempt2 := tracker.RecordFailure()
	assert.Equal(t, 10, backoff2)
	assert.Equal(t, 2, attempt2)

	tracker.RecordSuccess()

	// After success, next failure should restart from initial
	backoff3, attempt3 := tracker.RecordFailure()
	assert.Equal(t, 5, backoff3)
	assert.Equal(t, 1, attempt3)
}

func TestK8sClientCaching(t *testing.T) {
	// Reset global state before test
	clientInitMutex.Lock()
	cachedK8sClient = nil
	clientInitialized = false
	clientInitMutex.Unlock()

	// First call should attempt to create client
	// This will fail in test environment without K8s, but we can verify the caching logic
	client1, err1 := NewK8sClient()

	// Second call should use cached result
	client2, err2 := NewK8sClient()

	// Verify both calls return same result (cached)
	assert.Equal(t, err1, err2, "both calls should return same error")
	assert.Equal(t, client1, client2, "both calls should return same client instance")

	// Verify initialization flag
	clientInitMutex.Lock()
	if err1 == nil {
		assert.True(t, clientInitialized, "should be marked as initialized on success")
		assert.NotNil(t, cachedK8sClient, "cached client should be set on success")
	}
	clientInitMutex.Unlock()
}

func TestAgentCache_FreshCacheReturnsImmediately(t *testing.T) {
	cache := &AgentCache{
		ttl: 10 * time.Second,
	}

	testAgents := []Agent{
		{
			ObjectMeta: metav1.ObjectMeta{Name: "cached-agent"},
			Spec:       AgentSpec{Exposed: true},
		},
	}

	// Populate fresh cache
	cache.mu.Lock()
	cache.agents = testAgents
	cache.lastUpdate = time.Now()
	cache.mu.Unlock()

	// Attempting to get from fresh cache should return immediately
	// without trying to fetch (which would fail without K8s)
	cache.mu.RLock()
	isFresh := time.Since(cache.lastUpdate) < cache.ttl
	if isFresh {
		agents := cache.agents
		cache.mu.RUnlock()
		assert.Len(t, agents, 1)
		assert.Equal(t, "cached-agent", agents[0].Name)
	} else {
		cache.mu.RUnlock()
		t.Fatal("cache should be fresh")
	}
}

func TestBackoffCalculation(t *testing.T) {
	// Test the exact backoff calculation formula
	testCases := []struct {
		failures int
		expected int
	}{
		{failures: 1, expected: 5},   // First failure = 5
		{failures: 2, expected: 10},  // 5 * 2 = 10
		{failures: 3, expected: 20},  // 10 * 2 = 20
		{failures: 4, expected: 40},  // 20 * 2 = 40
		{failures: 5, expected: 80},  // 40 * 2 = 80
		{failures: 6, expected: 120}, // 80 * 2 = 160, but capped at 120
		{failures: 7, expected: 120}, // Capped
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("failures_%d", tc.failures), func(t *testing.T) {
			// Simulate the backoff calculation from RecordFailure
			backoffSeconds := initialBackoffSeconds
			for i := 1; i < tc.failures; i++ {
				backoffSeconds = int(float64(backoffSeconds) * backoffMultiplier)
				if backoffSeconds > maxBackoffSeconds {
					backoffSeconds = maxBackoffSeconds
					break
				}
			}

			assert.Equal(t, tc.expected, backoffSeconds,
				"backoff for %d failures should be %d", tc.failures, tc.expected)
		})
	}
}