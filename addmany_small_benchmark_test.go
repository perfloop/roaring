package roaring

import (
	"math/rand"
	"testing"
)

func BenchmarkAddManySmall(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	size := 3
	dat := make([]uint32, size)
	val := uint32(0)
	for i := 0; i < size; i++ {
		val += uint32(rng.Intn(5) + 1)
		dat[i] = val
	}
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rb := NewBitmap()
		rb.AddMany(dat)
	}
}
