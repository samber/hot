package base

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestEvictionCallback(t *testing.T) {
	is := assert.New(t)

	// Test that EvictionCallback can be assigned to a variable
	var callback EvictionCallback[string, int] = func(reason EvictionReason, key string, value int) {
		// This is just a test to ensure the type works
	}

	is.NotNil(callback)
}

func TestEvictionCallback_Execution(t *testing.T) {
	is := assert.New(t)

	var capturedReason EvictionReason
	var capturedKey string
	var capturedValue int

	// Create an eviction callback that captures the parameters
	callback := EvictionCallback[string, int](func(reason EvictionReason, key string, value int) {
		capturedReason = reason
		capturedKey = key
		capturedValue = value
	})

	// Test that the callback executes correctly
	testKey := "test-key"
	testValue := 42

	callback(EvictionReasonStale, testKey, testValue)

	is.Equal(EvictionReasonStale, capturedReason)
	is.Equal(testKey, capturedKey)
	is.Equal(testValue, capturedValue)
}

func TestEvictionCallback_NilCallback(t *testing.T) {
	is := assert.New(t)

	// Test that a nil callback doesn't panic
	var callback EvictionCallback[string, int] = nil

	// This should panic
	is.Panics(func() {
		callback(EvictionReasonStale, "key", 42)
	})

	is.True(true) // If we get here, no panic occurred
}

func TestEvictionCallback_DifferentTypes(t *testing.T) {
	is := assert.New(t)

	// Test with different key and value types
	var stringIntCallback EvictionCallback[string, int]
	var intStringCallback EvictionCallback[int, string]
	var boolFloatCallback EvictionCallback[bool, float64]

	// Test string key, int value
	stringIntCallback = func(reason EvictionReason, key string, value int) {
		is.Equal("test", key)
		is.Equal(123, value)
	}
	stringIntCallback(EvictionReasonStale, "test", 123)

	// Test int key, string value
	intStringCallback = func(reason EvictionReason, key int, value string) {
		is.Equal(456, key)
		is.Equal("value", value)
	}
	intStringCallback(EvictionReasonStale, 456, "value")

	// Test bool key, float64 value
	boolFloatCallback = func(reason EvictionReason, key bool, value float64) {
		is.Equal(true, key)
		is.Equal(3.14, value)
	}
	boolFloatCallback(EvictionReasonStale, true, 3.14)
}

func TestEvictionCallback_Closure(t *testing.T) {
	is := assert.New(t)

	// Test that the callback can capture variables from its scope
	counter := 0
	expectedCount := 3

	callback := EvictionCallback[string, int](func(reason EvictionReason, key string, value int) {
		counter++
	})

	// Execute the callback multiple times
	callback(EvictionReasonStale, "key1", 1)
	callback(EvictionReasonStale, "key2", 2)
	callback(EvictionReasonStale, "key3", 3)

	is.Equal(expectedCount, counter)
}

func TestEvictionCallback_InterfaceCompliance(t *testing.T) {
	is := assert.New(t)

	// Mock implementation
	mockCache := &mockCacheWithEviction[string, int]{
		callback: nil,
	}

	// Test setting and triggering callback
	testKey := "evicted-key"
	testValue := 999

	evictionCallback := EvictionCallback[string, int](func(reason EvictionReason, key string, value int) {
		is.Equal(testKey, key)
		is.Equal(testValue, value)
	})

	mockCache.SetEvictionCallback(evictionCallback)
	mockCache.TriggerEviction(EvictionReasonStale, testKey, testValue)
}

// Mock implementation for testing
type mockCacheWithEviction[K comparable, V any] struct {
	callback EvictionCallback[K, V]
}

func (m *mockCacheWithEviction[K, V]) SetEvictionCallback(callback EvictionCallback[K, V]) {
	m.callback = callback
}

func (m *mockCacheWithEviction[K, V]) TriggerEviction(reason EvictionReason, key K, value V) {
	if m.callback != nil {
		m.callback(reason, key, value)
	}
}
