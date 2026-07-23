package roaring

import (
	"math/rand"
	"testing"
)

func BenchmarkAddManySorted(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	size := 50000
	dat := make([]uint32, size)
	val := uint32(0)
	for i := 0; i < size; i++ {
		if rng.Float64() < 0.95 {
			val += uint32(rng.Intn(5) + 1)
		} else {
			val += uint32(rng.Intn(100000) + 65536)
		}
		dat[i] = val
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rb := NewBitmap()
		rb.AddMany(dat)
	}
}

func BenchmarkAddManyUnsorted(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	size := 50000
	dat := make([]uint32, size)
	val := uint32(0)
	for i := 0; i < size; i++ {
		if rng.Float64() < 0.95 {
			val += uint32(rng.Intn(5) + 1)
		} else {
			val += uint32(rng.Intn(100000) + 65536)
		}
		dat[i] = val
	}
	rng.Shuffle(len(dat), func(i, j int) {
		dat[i], dat[j] = dat[j], dat[i]
	})
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rb := NewBitmap()
		rb.AddMany(dat)
	}
}
