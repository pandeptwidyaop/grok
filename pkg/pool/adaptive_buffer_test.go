package pool

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewAdaptiveBufferPool tests pool initialization.
func TestNewAdaptiveBufferPool(t *testing.T) {
	pool := NewAdaptiveBufferPool()
	require.NotNil(t, pool)
	require.NotNil(t, pool.small)
	require.NotNil(t, pool.medium)
	require.NotNil(t, pool.large)
}

// TestAdaptiveBufferPoolGetSmall tests getting small buffers.
func TestAdaptiveBufferPoolGetSmall(t *testing.T) {
	pool := NewAdaptiveBufferPool()

	// Get small buffer with size hint
	buf := pool.Get(512) // 512 bytes
	require.NotNil(t, buf)
	assert.Equal(t, TierSmall, buf.Tier())
	assert.Equal(t, SmallBufferSize, buf.Cap())

	buf.Release()

	// Get again
	buf2 := pool.Get(1024) // 1KB
	require.NotNil(t, buf2)
	assert.Equal(t, TierSmall, buf2.Tier())

	buf2.Release()
}

// TestAdaptiveBufferPoolGetMedium tests getting medium buffers.
func TestAdaptiveBufferPoolGetMedium(t *testing.T) {
	pool := NewAdaptiveBufferPool()

	// Get medium buffer
	buf := pool.Get(32 * 1024) // 32KB
	require.NotNil(t, buf)
	assert.Equal(t, TierMedium, buf.Tier())
	assert.Equal(t, MediumBufferSize, buf.Cap())

	buf.Release()

	// Get with unknown size (should default to medium)
	buf2 := pool.Get(0)
	require.NotNil(t, buf2)
	assert.Equal(t, TierMedium, buf2.Tier())

	buf2.Release()
}

// TestAdaptiveBufferPoolGetLarge tests getting large buffers.
func TestAdaptiveBufferPoolGetLarge(t *testing.T) {
	pool := NewAdaptiveBufferPool()

	// Get large buffer
	buf := pool.Get(1024 * 1024) // 1MB
	require.NotNil(t, buf)
	assert.Equal(t, TierLarge, buf.Tier())
	assert.Equal(t, LargeBufferSize, buf.Cap())

	buf.Release()

	// Get again
	buf2 := pool.Get(2 * 1024 * 1024) // 2MB
	require.NotNil(t, buf2)
	assert.Equal(t, TierLarge, buf2.Tier())

	buf2.Release()
}

// TestAdaptiveBufferPoolTierSelection tests automatic tier selection.
func TestAdaptiveBufferPoolTierSelection(t *testing.T) {
	tests := []struct {
		name         string
		size         int
		expectedTier BufferTier
	}{
		{"tiny", 100, TierSmall},
		{"small boundary", 1024, TierSmall},
		{"just above small", 1025, TierMedium},
		{"medium", 32 * 1024, TierMedium},
		{"medium boundary", 64 * 1024, TierMedium},
		{"just above medium", 64*1024 + 1, TierLarge},
		{"large", 1024 * 1024, TierLarge},
		{"very large", 10 * 1024 * 1024, TierLarge},
		{"unknown (0)", 0, TierMedium}, // Default to medium
	}

	pool := NewAdaptiveBufferPool()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := pool.Get(tt.size)
			assert.Equal(t, tt.expectedTier, buf.Tier())
			buf.Release()
		})
	}
}

// TestAdaptiveBufferPoolReuse tests buffer reuse.
func TestAdaptiveBufferPoolReuse(t *testing.T) {
	pool := NewAdaptiveBufferPool()

	// Get and release multiple times
	for i := 0; i < 10; i++ {
		buf := pool.Get(10 * 1024) // 10KB - medium tier
		require.NotNil(t, buf)
		assert.Equal(t, TierMedium, buf.Tier())
		buf.Release()
	}

	// Check that buffers were allocated
	stats := pool.Stats()
	totalMedium := stats.MediumAllocated.Load() + stats.MediumReused.Load()
	assert.Greater(t, totalMedium, int64(0))
	assert.GreaterOrEqual(t, stats.ReuseRate(), 0.0)
	assert.LessOrEqual(t, stats.ReuseRate(), 1.0)
}

// TestAdaptiveBufferPoolConcurrent tests concurrent access.
func TestAdaptiveBufferPoolConcurrent(t *testing.T) {
	pool := NewAdaptiveBufferPool()

	concurrency := 100
	iterations := 100

	var wg sync.WaitGroup
	wg.Add(concurrency)

	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer wg.Done()

			for j := 0; j < iterations; j++ {
				// Vary sizes to test all tiers
				size := (id*iterations + j) % (5 * 1024 * 1024)
				buf := pool.Get(size)
				require.NotNil(t, buf)

				// Simulate some work
				_ = buf.Bytes()

				buf.Release()
			}
		}(i)
	}

	wg.Wait()

	// Verify stats - just check that buffers were used
	stats := pool.Stats()
	totalAllocated := stats.SmallAllocated.Load() + stats.MediumAllocated.Load() + stats.LargeAllocated.Load()

	assert.Greater(t, totalAllocated, int64(0))
	assert.GreaterOrEqual(t, stats.ReuseRate(), 0.0)
}

// TestAdaptiveBufferPoolMemorySavings tests memory savings calculation.
func TestAdaptiveBufferPoolMemorySavings(t *testing.T) {
	pool := NewAdaptiveBufferPool()

	// Get mostly small and medium buffers
	for i := 0; i < 100; i++ {
		buf := pool.Get(1024) // Small buffer
		buf.Release()
	}

	for i := 0; i < 50; i++ {
		buf := pool.Get(32 * 1024) // Medium buffer
		buf.Release()
	}

	// Get a few large buffers
	for i := 0; i < 5; i++ {
		buf := pool.Get(1024 * 1024) // Large buffer
		buf.Release()
	}

	stats := pool.Stats()
	savings := stats.MemorySavings()

	// Should have savings if we allocated any small/medium buffers
	// Note: sync.Pool might not reuse buffers, so savings could be 0
	assert.GreaterOrEqual(t, savings, int64(0))

	// Verify we allocated buffers
	totalAllocated := stats.SmallAllocated.Load() + stats.MediumAllocated.Load() + stats.LargeAllocated.Load()
	assert.Greater(t, totalAllocated, int64(0))

	t.Logf("Memory savings: %d bytes (%.2f MB)", savings, float64(savings)/(1024*1024))
}

// TestAdaptiveBufferPoolStats tests statistics tracking.
func TestAdaptiveBufferPoolStats(t *testing.T) {
	pool := NewAdaptiveBufferPool()

	// Allocate different tier buffers multiple times to ensure allocation
	for i := 0; i < 10; i++ {
		small := pool.GetSmall()
		medium := pool.GetMedium()
		large := pool.GetLarge()

		small.Release()
		medium.Release()
		large.Release()
	}

	stats := pool.Stats()

	// Verify we allocated and/or reused buffers
	totalSmall := stats.SmallAllocated.Load() + stats.SmallReused.Load()
	totalMedium := stats.MediumAllocated.Load() + stats.MediumReused.Load()
	totalLarge := stats.LargeAllocated.Load() + stats.LargeReused.Load()

	assert.Greater(t, totalSmall, int64(0))
	assert.Greater(t, totalMedium, int64(0))
	assert.Greater(t, totalLarge, int64(0))

	// Check stats map
	statsMap := pool.StatsMap()
	assert.NotNil(t, statsMap)
	assert.Contains(t, statsMap, "small_allocated")
	assert.Contains(t, statsMap, "reuse_rate")
	assert.Contains(t, statsMap, "memory_savings_bytes")

	t.Logf("Stats: %+v", statsMap)
}

// TestSelectTier tests tier selection helper.
func TestSelectTier(t *testing.T) {
	tests := []struct {
		size         int
		expectedTier BufferTier
	}{
		{100, TierSmall},
		{1024, TierSmall},
		{1025, TierMedium},
		{64 * 1024, TierMedium},
		{64*1024 + 1, TierLarge},
		{1024 * 1024, TierLarge},
	}

	for _, tt := range tests {
		tier := SelectTier(tt.size)
		assert.Equal(t, tt.expectedTier, tier)
	}
}

// TestTierSize tests tier size helper.
func TestTierSize(t *testing.T) {
	assert.Equal(t, SmallBufferSize, TierSize(TierSmall))
	assert.Equal(t, MediumBufferSize, TierSize(TierMedium))
	assert.Equal(t, LargeBufferSize, TierSize(TierLarge))
	assert.Equal(t, MediumBufferSize, TierSize(BufferTier(999))) // Unknown defaults to medium
}

// BenchmarkAdaptiveBufferPoolGet benchmarks buffer allocation.
func BenchmarkAdaptiveBufferPoolGet(b *testing.B) {
	pool := NewAdaptiveBufferPool()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			buf := pool.Get(10 * 1024) // 10KB
			buf.Release()
		}
	})
}

// BenchmarkAdaptiveBufferPoolGetSmall benchmarks small buffer allocation.
func BenchmarkAdaptiveBufferPoolGetSmall(b *testing.B) {
	pool := NewAdaptiveBufferPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.GetSmall()
		buf.Release()
	}
}

// BenchmarkAdaptiveBufferPoolGetMedium benchmarks medium buffer allocation.
func BenchmarkAdaptiveBufferPoolGetMedium(b *testing.B) {
	pool := NewAdaptiveBufferPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.GetMedium()
		buf.Release()
	}
}

// BenchmarkAdaptiveBufferPoolGetLarge benchmarks large buffer allocation.
func BenchmarkAdaptiveBufferPoolGetLarge(b *testing.B) {
	pool := NewAdaptiveBufferPool()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buf := pool.GetLarge()
		buf.Release()
	}
}

// BenchmarkAdaptiveVsFixed compares adaptive vs fixed buffer allocation.
func BenchmarkAdaptiveVsFixed(b *testing.B) {
	b.Run("Adaptive", func(b *testing.B) {
		pool := NewAdaptiveBufferPool()
		sizes := []int{512, 10 * 1024, 100 * 1024, 1024 * 1024}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			size := sizes[i%len(sizes)]
			buf := pool.Get(size)
			buf.Release()
		}
	})

	b.Run("Fixed4MB", func(b *testing.B) {
		pool := &sync.Pool{
			New: func() interface{} {
				buf := make([]byte, 4*1024*1024)
				return &buf
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := pool.Get().(*[]byte)
			pool.Put(buf)
		}
	})
}
