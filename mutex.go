package hot

// rwMutex defines the interface for read-write mutex operations.
// This interface allows for different mutex implementations, including no-op mocks.
type rwMutex interface {
	Lock()
	Unlock()

	RLock()
	RUnlock()
}

// mutexMock is a no-op implementation of rwMutex used when locking is disabled.
// It provides zero-cost mutex operations for single-threaded usage.
type mutexMock struct{}

// Ensure mutexMock implements rwMutex interface.
var _ rwMutex = (*mutexMock)(nil)

// Lock is a no-op operation for the mock mutex.
func (f mutexMock) Lock() {}

// Unlock is a no-op operation for the mock mutex.
func (f mutexMock) Unlock() {}

// RLock is a no-op operation for the mock mutex.
func (f mutexMock) RLock() {}

// RUnlock is a no-op operation for the mock mutex.
func (f mutexMock) RUnlock() {}
