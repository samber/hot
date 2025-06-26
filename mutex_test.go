package hot

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMutexMock(t *testing.T) {
	is := assert.New(t)

	// Test that mutexMock implements rwMutex interface
	var _ rwMutex = (*mutexMock)(nil)

	// Create a mutexMock instance
	mock := mutexMock{}

	// Test that all methods can be called without panicking
	is.NotPanics(func() {
		mock.Lock()
	})

	is.NotPanics(func() {
		mock.Unlock()
	})

	is.NotPanics(func() {
		mock.RLock()
	})

	is.NotPanics(func() {
		mock.RUnlock()
	})

	// Test that multiple calls don't cause issues
	is.NotPanics(func() {
		mock.Lock()
		mock.Lock()
		mock.Unlock() //nolint:staticcheck
		mock.Unlock()
	})

	is.NotPanics(func() {
		mock.RLock()
		mock.RLock()
		mock.RUnlock() //nolint:staticcheck
		mock.RUnlock()
	})

	// Test mixed read/write operations
	is.NotPanics(func() {
		mock.Lock()
		mock.Unlock() //nolint:staticcheck
		mock.RLock()
		mock.RUnlock() //nolint:staticcheck
		mock.Lock()
		mock.Unlock() //nolint:staticcheck
	})
}

func TestMutexMockConcurrency(t *testing.T) {
	is := assert.New(t)

	mock := mutexMock{}

	// Test that multiple goroutines can call the mock methods without issues
	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func() {
			defer func() { done <- true }()

			// Call all methods multiple times
			for j := 0; j < 100; j++ {
				mock.Lock()
				mock.Unlock() //nolint:staticcheck
				mock.RLock()
				mock.RUnlock() //nolint:staticcheck
			}
		}()
	}

	// Wait for all goroutines to complete
	for i := 0; i < 10; i++ {
		<-done
	}

	// If we get here without panicking, the test passes
	is.True(true)
}
