package roaring

import "testing"

func BenchmarkOrCardinality(b *testing.B) {
	b.Run("array-array", func(b *testing.B) {
		bm1 := NewBitmap()
		bm2 := NewBitmap()
		for i := uint32(0); i < 1000; i++ {
			bm1.Add(i * 2)
			bm2.Add(i*2 + 1)
		}
		b.ResetTimer()
		b.ReportAllocs()
		var card uint64
		for i := 0; i < b.N; i++ {
			card = bm1.OrCardinality(bm2)
		}
		_ = card
	})

	b.Run("bitmap-bitmap", func(b *testing.B) {
		bm1 := NewBitmap()
		bm2 := NewBitmap()
		for i := uint32(0); i < 6000; i++ {
			bm1.Add(i * 2)
			bm2.Add(i*2 + 1)
		}
		b.ResetTimer()
		b.ReportAllocs()
		var card uint64
		for i := 0; i < b.N; i++ {
			card = bm1.OrCardinality(bm2)
		}
		_ = card
	})

	b.Run("run-run", func(b *testing.B) {
		bm1 := NewBitmap()
		bm2 := NewBitmap()
		bm1.AddRange(0, 2000)
		bm1.AddRange(3000, 5000)
		bm2.AddRange(1000, 4000)
		bm1.RunOptimize()
		bm2.RunOptimize()
		b.ResetTimer()
		b.ReportAllocs()
		var card uint64
		for i := 0; i < b.N; i++ {
			card = bm1.OrCardinality(bm2)
		}
		_ = card
	})
}
