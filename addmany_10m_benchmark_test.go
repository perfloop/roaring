package roaring

import (
	"math/rand"
	"testing"
)

func BenchmarkAddManyUnsorted10M(b *testing.B) {
	rng := rand.New(rand.NewSource(42))
	size := 10000000
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

func BenchmarkAddManyAdversarial(b *testing.B) {
	size := 10000000
	dat := make([]uint32, size) // all 0
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		rb := NewBitmap()
		rb.AddMany(dat)
	}
}

func TestAddManyAdversarial(t *testing.T) {
	// Sorted duplicates of size 10M
	dat := make([]uint32, 10000000) // all 0
	rb := NewBitmap()
	rb.AddMany(dat)
	if rb.GetCardinality() != 1 {
		t.Errorf("expected cardinality 1, got %d", rb.GetCardinality())
	}
	// Check that the container's capacity is small
	pos := rb.highlowcontainer.getIndex(0)
	if pos < 0 {
		t.Errorf("expected container at key 0")
	} else {
		c := rb.highlowcontainer.getContainerAtIndex(pos)
		if ac, ok := c.(*arrayContainer); ok {
			if cap(ac.content) > 64 {
				t.Errorf("expected small capacity, got %d", cap(ac.content))
			}
		} else {
			t.Errorf("expected arrayContainer, got %T", c)
		}
	}

	// Unsorted duplicates of size 10M (duplicates of 0 are both sorted and unsorted)
	rb2 := NewBitmap()
	rb2.AddMany(dat)
	if rb2.GetCardinality() != 1 {
		t.Errorf("expected cardinality 1, got %d", rb2.GetCardinality())
	}
}
