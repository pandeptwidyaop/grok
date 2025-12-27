package storage

import "sync"

// CircularBuffer is a thread-safe ring buffer with fixed capacity.
// When the buffer is full, new items overwrite the oldest ones.
type CircularBuffer[T any] struct {
	buffer  []T
	head    int // Index where next item will be written
	size    int // Current number of items in buffer
	maxSize int // Maximum capacity
	mu      sync.RWMutex
}

// NewCircularBuffer creates a new circular buffer with the specified capacity.
func NewCircularBuffer[T any](maxSize int) *CircularBuffer[T] {
	return &CircularBuffer[T]{
		buffer:  make([]T, maxSize),
		maxSize: maxSize,
	}
}

// Add adds an item to the buffer.
// If the buffer is full, the oldest item is overwritten.
func (cb *CircularBuffer[T]) Add(item T) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.buffer[cb.head] = item
	cb.head = (cb.head + 1) % cb.maxSize

	if cb.size < cb.maxSize {
		cb.size++
	}
}

// GetRecent returns the most recent N items, ordered from newest to oldest.
// If limit > size, returns all items.
func (cb *CircularBuffer[T]) GetRecent(limit int) []T {
	cb.mu.RLock()
	defer cb.mu.RUnlock()

	if limit > cb.size {
		limit = cb.size
	}

	result := make([]T, 0, limit)

	// Iterate backwards from most recent
	for i := 0; i < limit; i++ {
		idx := (cb.head - 1 - i + cb.maxSize) % cb.maxSize
		result = append(result, cb.buffer[idx])
	}

	return result
}

// GetAll returns all items in the buffer, ordered from newest to oldest.
func (cb *CircularBuffer[T]) GetAll() []T {
	return cb.GetRecent(cb.size)
}

// Size returns the current number of items in the buffer.
func (cb *CircularBuffer[T]) Size() int {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.size
}

// Capacity returns the maximum capacity of the buffer.
func (cb *CircularBuffer[T]) Capacity() int {
	return cb.maxSize
}

// Clear removes all items from the buffer.
func (cb *CircularBuffer[T]) Clear() {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	cb.head = 0
	cb.size = 0
	cb.buffer = make([]T, cb.maxSize)
}

// IsFull returns true if the buffer is at maximum capacity.
func (cb *CircularBuffer[T]) IsFull() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.size == cb.maxSize
}

// IsEmpty returns true if the buffer contains no items.
func (cb *CircularBuffer[T]) IsEmpty() bool {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.size == 0
}
