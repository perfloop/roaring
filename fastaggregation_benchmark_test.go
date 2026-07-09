package roaring

import (
	"math/rand"
	"testing"
)

func BenchmarkHeapXor(b *testing.B) {
	b.StopTimer()
	r := rand.New(rand.NewSource(42))
	bms := make([]*Bitmap, 32)
	for i := range bms {
		bm := NewBitmap()
		// Let's generate a mix of sparse and dense containers
		// High density range:
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
