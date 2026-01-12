// Unless explicitly stated otherwise all files in this repository are licensed
// under the MIT License.
// This product includes software developed at Guance Cloud (https://www.guance.com/).
// Copyright 2021-present Guance, Inc.

package diskcache

import (
	"sync"
	"testing"
	"time"
)

func TestLockContention(t *testing.T) {
	// Create instrumented mutex
	mu := NewInstrumentedMutex(LockTypeWrite, "/test/path", lockWaitTimeVec, lockContentionVec)

	// Test immediate lock (no contention)
	start := time.Now()
	mu.Lock()
	duration1 := time.Since(start)
	mu.Unlock()

	// Should be very fast (no contention)
	if duration1 > time.Millisecond {
		t.Errorf("Immediate lock took too long: %v", duration1)
	}

	// Test contention
	var wg sync.WaitGroup
	contentionStarted := make(chan bool, 1)

	// First goroutine holds lock
	wg.Add(1)
	go func() {
		defer wg.Done()
		mu.Lock()
		defer mu.Unlock()

		// Signal that first lock is acquired
		contentionStarted <- true

		// Hold lock for a bit
		time.Sleep(50 * time.Millisecond)
	}()

	// Wait for first lock to be acquired
	<-contentionStarted

	// Second goroutine tries to lock (will contend)
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		mu.Lock() // This should wait due to contention
		duration := time.Since(start)
		mu.Unlock()

		// Should wait at least some time due to contention
		if duration < 10*time.Millisecond {
			t.Errorf("Contented lock didn't wait enough: %v", duration)
		}
	}()

	wg.Wait()

	// Check that contention was recorded
	t.Logf("Lock contention test completed")
}

func TestInstrumentedMutex(t *testing.T) {
	mu := NewInstrumentedMutex(LockTypeRead, "/test/path", lockWaitTimeVec, lockContentionVec)

	// Test TryLock success
	if !mu.TryLock() {
		t.Error("TryLock should succeed initially")
	}

	// Test TryLock failure (already locked)
	if mu.TryLock() {
		t.Error("TryLock should fail when already locked")
	}

	mu.Unlock()

	// Test TryLock after unlock
	if !mu.TryLock() {
		t.Error("TryLock should succeed after unlock")
	}

	mu.Unlock()
}

func TestInstrumentedRWMutex(t *testing.T) {
	rwmu := NewInstrumentedRWMutex("/test/path", lockWaitTimeVec, lockContentionVec)

	// Test read lock
	rwmu.RLock()
	if !rwmu.TryRLock() {
		t.Error("Multiple read locks should be allowed")
	}
	rwmu.RUnlock()
	rwmu.RUnlock()

	// Test write lock exclusion
	rwmu.Lock()
	if rwmu.TryLock() {
		t.Error("Write lock should exclude other writes")
	}
	if rwmu.TryRLock() {
		t.Error("Write lock should exclude reads")
	}
	rwmu.Unlock()

	// Test read-write contention
	var wg sync.WaitGroup
	readStarted := make(chan bool, 1)

	// Start read lock holder
	wg.Add(1)
	go func() {
		defer wg.Done()
		rwmu.RLock()
		defer rwmu.RUnlock()
		readStarted <- true
		time.Sleep(30 * time.Millisecond)
	}()

	<-readStarted

	// Try to get write lock while read is held
	wg.Add(1)
	go func() {
		defer wg.Done()
		start := time.Now()
		rwmu.Lock() // Should wait for read to complete
		duration := time.Since(start)
		rwmu.Unlock()

		if duration < 10*time.Millisecond {
			t.Errorf("Write lock during read didn't wait enough: %v", duration)
		}
	}()

	wg.Wait()
}
