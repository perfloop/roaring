package roaring

import (
	"math/rand"
	"testing"
)

// Helper to generate a random bitmap with a mix of sparse, dense, and run containers
func generateMixBitmap(r *rand.Rand) *Bitmap {
	bm := NewBitmap()
	// High density range
	for j := 0; j < 5000; j++ {
		bm.Add(uint32(r.Intn(100000)))
	}
	// Some runs
	for j := uint32(200000); j < 250000; j++ {
		if r.Float64() < 0.95 {
			bm.Add(j)
		}
	}
	// Sparse range
	for j := 0; j < 200; j++ {
		bm.Add(uint32(300000 + r.Intn(10000000)))
	}
	return bm
}

func BenchmarkHeapXor(b *testing.B) {
	b.StopTimer()
	r := rand.New(rand.NewSource(42))
	bms := make([]*Bitmap, 32)
	for i := range bms {
		bm := generateMixBitmap(r)
		if i%3 == 0 {
			bm.RunOptimize()
		}
		bms[i] = bm
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := HeapXor(bms...)
			_ = res.GetCardinality()
		}
	})
}

func BenchmarkHeapXorLow(b *testing.B) {
	b.StopTimer()
	r := rand.New(rand.NewSource(42))
	bms := make([]*Bitmap, 3)
	for i := range bms {
		bm := generateMixBitmap(r)
		if i%3 == 0 {
			bm.RunOptimize()
		}
		bms[i] = bm
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := HeapXor(bms...)
			_ = res.GetCardinality()
		}
	})
}

func BenchmarkHeapXorLarge(b *testing.B) {
	b.StopTimer()
	r := rand.New(rand.NewSource(42))
	bms := make([]*Bitmap, 128)
	for i := range bms {
		bm := generateMixBitmap(r)
		if i%3 == 0 {
			bm.RunOptimize()
		}
		bms[i] = bm
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := HeapXor(bms...)
			_ = res.GetCardinality()
		}
	})
}

func BenchmarkHeapXorSparse(b *testing.B) {
	b.StopTimer()
	r := rand.New(rand.NewSource(42))
	bms := make([]*Bitmap, 32)
	for i := range bms {
		bm := NewBitmap()
		// Purely sparse arrayContainers
		for j := 0; j < 100; j++ {
			bm.Add(uint32(r.Intn(10000000)))
		}
		bms[i] = bm
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := HeapXor(bms...)
			_ = res.GetCardinality()
		}
	})
}

func BenchmarkHeapXorDense(b *testing.B) {
	b.StopTimer()
	r := rand.New(rand.NewSource(42))
	bms := make([]*Bitmap, 32)
	for i := range bms {
		bm := NewBitmap()
		// Purely dense bitmapContainers (> 4096 per chunk)
		base := uint32(i) * 65536
		for j := 0; j < 6000; j++ {
			bm.Add(base + uint32(r.Intn(65536)))
		}
		bms[i] = bm
	}

	b.StartTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			res := HeapXor(bms...)
			_ = res.GetCardinality()
		}
	})
}
