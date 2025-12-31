package pool

import (
	"sync"
	"sync/atomic"

	"github.com/pandeptwidyaop/grok/pkg/logger"
)

// BufferTier represents the size tier of a buffer.
type BufferTier int

const (
	// TierSmall is for buffers <= 1KB.
	TierSmall BufferTier = iota
	// TierMedium is for buffers <= 64KB.
	TierMedium
	// TierLarge is for buffers <= 4MB.
	TierLarge
)

const (
	// SmallBufferSize is 1KB.
	SmallBufferSize = 1024
	// MediumBufferSize is 64KB.
	MediumBufferSize = 64 * 1024
	// LargeBufferSize is 4MB.
	LargeBufferSize = 4 * 1024 * 1024
)

// BufferStats tracks buffer pool statistics.
type BufferStats struct {
	// SmallAllocated is the number of small buffers allocated.
	SmallAllocated atomic.Int64
	// MediumAllocated is the number of medium buffers allocated.
	MediumAllocated atomic.Int64
	// LargeAllocated is the number of large buffers allocated.
	LargeAllocated atomic.Int64
	// SmallReused is the number of small buffers reused from pool.
	SmallReused atomic.Int64
	// MediumReused is the number of medium buffers reused from pool.
	MediumReused atomic.Int64
	// LargeReused is the number of large buffers reused from pool.
	LargeReused atomic.Int64
	// TotalBytesAllocated is the total bytes allocated across all tiers.
	TotalBytesAllocated atomic.Int64
}

// ReuseRate returns the overall buffer reuse rate (0.0 - 1.0).
func (s *BufferStats) ReuseRate() float64 {
	total := s.SmallAllocated.Load() + s.MediumAllocated.Load() + s.LargeAllocated.Load()
	if total == 0 {
		return 0
	}
	reused := s.SmallReused.Load() + s.MediumReused.Load() + s.LargeReused.Load()
	return float64(reused) / float64(total+reused)
}

// MemorySavings returns estimated memory savings in bytes compared to always using large buffers.
func (s *BufferStats) MemorySavings() int64 {
	// If we always used large buffers
	totalRequests := s.SmallAllocated.Load() + s.MediumAllocated.Load() + s.LargeAllocated.Load()
	wouldUse := totalRequests * LargeBufferSize

	// What we actually allocated
	actualUse := s.TotalBytesAllocated.Load()

	return wouldUse - actualUse
}

// Buffer wraps a byte slice with metadata.
type Buffer struct {
	data []byte
	tier BufferTier
	pool *AdaptiveBufferPool
}

// Bytes returns the underlying byte slice.
func (b *Buffer) Bytes() []byte {
	return b.data
}

// Len returns the length of the buffer.
func (b *Buffer) Len() int {
	return len(b.data)
}

// Cap returns the capacity of the buffer.
func (b *Buffer) Cap() int {
	return cap(b.data)
}

// Tier returns the buffer tier.
func (b *Buffer) Tier() BufferTier {
	return b.tier
}

// Release returns the buffer to the appropriate pool.
func (b *Buffer) Release() {
	if b.pool == nil {
		return
	}

	// Reset slice to full capacity
	b.data = b.data[:cap(b.data)]

	switch b.tier {
	case TierSmall:
		b.pool.small.Put(&b.data)
	case TierMedium:
		b.pool.medium.Put(&b.data)
	case TierLarge:
		b.pool.large.Put(&b.data)
	}

	logger.DebugEvent().
		Str("tier", b.tierName()).
		Int("size", cap(b.data)).
		Msg("Buffer returned to pool")
}

func (b *Buffer) tierName() string {
	switch b.tier {
	case TierSmall:
		return "small"
	case TierMedium:
		return "medium"
	case TierLarge:
		return "large"
	default:
		return "unknown"
	}
}

// AdaptiveBufferPool manages multiple buffer pools of different sizes.
type AdaptiveBufferPool struct {
	small  *sync.Pool
	medium *sync.Pool
	large  *sync.Pool
	stats  BufferStats
}

// NewAdaptiveBufferPool creates a new adaptive buffer pool.
func NewAdaptiveBufferPool() *AdaptiveBufferPool {
	p := &AdaptiveBufferPool{
		small: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, SmallBufferSize)
				return &buf
			},
		},
		medium: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, MediumBufferSize)
				return &buf
			},
		},
		large: &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, LargeBufferSize)
				return &buf
			},
		},
	}

	logger.InfoEvent().
		Int("small_size", SmallBufferSize).
		Int("medium_size", MediumBufferSize).
		Int("large_size", LargeBufferSize).
		Msg("Adaptive buffer pool initialized")

	return p
}

// Get returns an optimal buffer for the given estimated size.
// If estimatedSize is 0 or unknown, returns a medium buffer as safe default.
func (p *AdaptiveBufferPool) Get(estimatedSize int) *Buffer {
	var data *[]byte
	var tier BufferTier

	switch {
	case estimatedSize > 0 && estimatedSize <= SmallBufferSize:
		// Small buffer (1KB)
		tier = TierSmall
		pooled := p.small.Get()
		data = pooled.(*[]byte)
		p.stats.SmallAllocated.Add(1)
		p.stats.TotalBytesAllocated.Add(SmallBufferSize)

	case estimatedSize > SmallBufferSize && estimatedSize <= MediumBufferSize:
		// Medium buffer (64KB)
		tier = TierMedium
		pooled := p.medium.Get()
		data = pooled.(*[]byte)
		p.stats.MediumAllocated.Add(1)
		p.stats.TotalBytesAllocated.Add(MediumBufferSize)

	default:
		// Large buffer (4MB) or unknown size (use medium as safe default)
		if estimatedSize == 0 || estimatedSize <= MediumBufferSize {
			// Unknown size or small enough - use medium buffer
			tier = TierMedium
			pooled := p.medium.Get()
			data = pooled.(*[]byte)
			p.stats.MediumAllocated.Add(1)
			p.stats.TotalBytesAllocated.Add(MediumBufferSize)
		} else {
			// Large buffer needed
			tier = TierLarge
			pooled := p.large.Get()
			data = pooled.(*[]byte)
			p.stats.LargeAllocated.Add(1)
			p.stats.TotalBytesAllocated.Add(LargeBufferSize)
		}
	}

	buffer := &Buffer{
		data: *data,
		tier: tier,
		pool: p,
	}

	logger.DebugEvent().
		Str("tier", buffer.tierName()).
		Int("size", cap(buffer.data)).
		Int("estimated_size", estimatedSize).
		Msg("Got buffer from pool")

	return buffer
}

// GetSmall returns a small buffer (1KB).
func (p *AdaptiveBufferPool) GetSmall() *Buffer {
	return p.Get(SmallBufferSize)
}

// GetMedium returns a medium buffer (64KB).
func (p *AdaptiveBufferPool) GetMedium() *Buffer {
	return p.Get(MediumBufferSize)
}

// GetLarge returns a large buffer (4MB).
func (p *AdaptiveBufferPool) GetLarge() *Buffer {
	return p.Get(LargeBufferSize)
}

// Stats returns a snapshot of buffer pool statistics.
func (p *AdaptiveBufferPool) Stats() BufferStats {
	return p.stats
}

// StatsMap returns buffer pool statistics as a map.
func (p *AdaptiveBufferPool) StatsMap() map[string]interface{} {
	return map[string]interface{}{
		"small_allocated":       p.stats.SmallAllocated.Load(),
		"medium_allocated":      p.stats.MediumAllocated.Load(),
		"large_allocated":       p.stats.LargeAllocated.Load(),
		"small_reused":          p.stats.SmallReused.Load(),
		"medium_reused":         p.stats.MediumReused.Load(),
		"large_reused":          p.stats.LargeReused.Load(),
		"total_bytes_allocated": p.stats.TotalBytesAllocated.Load(),
		"reuse_rate":            p.stats.ReuseRate(),
		"memory_savings_bytes":  p.stats.MemorySavings(),
	}
}

// SelectTier determines the optimal buffer tier for a given size.
func SelectTier(size int) BufferTier {
	switch {
	case size <= SmallBufferSize:
		return TierSmall
	case size <= MediumBufferSize:
		return TierMedium
	default:
		return TierLarge
	}
}

// TierSize returns the buffer size for a given tier.
func TierSize(tier BufferTier) int {
	switch tier {
	case TierSmall:
		return SmallBufferSize
	case TierMedium:
		return MediumBufferSize
	case TierLarge:
		return LargeBufferSize
	default:
		return MediumBufferSize
	}
}
