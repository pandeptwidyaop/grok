package metrics

import (
	"sort"
	"sync"
	"time"
)

// Histogram tracks latency distribution for calculating percentiles.
type Histogram struct {
	samples    []time.Duration
	mu         sync.RWMutex
	maxSamples int
}

// NewHistogram creates a new latency histogram.
func NewHistogram() *Histogram {
	return &Histogram{
		samples:    make([]time.Duration, 0, 10000),
		maxSamples: 10000, // Keep last 10k samples
	}
}

// Record adds a latency sample to the histogram.
func (h *Histogram) Record(duration time.Duration) {
	h.mu.Lock()
	defer h.mu.Unlock()

	// If full, evict oldest 20% to make room
	if len(h.samples) >= h.maxSamples {
		evictCount := h.maxSamples / 5 // 20%
		copy(h.samples, h.samples[evictCount:])
		h.samples = h.samples[:h.maxSamples-evictCount]
	}

	h.samples = append(h.samples, duration)
}

// Percentile calculates the pth percentile (0.0 to 1.0).
// For example, p=0.95 returns the 95th percentile.
func (h *Histogram) Percentile(p float64) time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.samples) == 0 {
		return 0
	}

	// Create sorted copy
	sorted := make([]time.Duration, len(h.samples))
	copy(sorted, h.samples)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i] < sorted[j]
	})

	// Calculate index
	idx := int(float64(len(sorted)) * p)
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}

	return sorted[idx]
}

// Mean calculates the average latency.
func (h *Histogram) Mean() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.samples) == 0 {
		return 0
	}

	var sum time.Duration
	for _, s := range h.samples {
		sum += s
	}

	return sum / time.Duration(len(h.samples))
}

// Min returns the minimum latency.
func (h *Histogram) Min() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.samples) == 0 {
		return 0
	}

	minVal := h.samples[0]
	for _, s := range h.samples[1:] {
		if s < minVal {
			minVal = s
		}
	}

	return minVal
}

// Max returns the maximum latency.
func (h *Histogram) Max() time.Duration {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if len(h.samples) == 0 {
		return 0
	}

	maxVal := h.samples[0]
	for _, s := range h.samples[1:] {
		if s > maxVal {
			maxVal = s
		}
	}

	return maxVal
}

// Count returns the number of samples.
func (h *Histogram) Count() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.samples)
}

// Clear removes all samples.
func (h *Histogram) Clear() {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.samples = make([]time.Duration, 0, h.maxSamples)
}
