package roaring

import (
	"math/rand"
	"testing"
)

func TestAndAnyCorrectness(t *testing.T) {
	// 1. Empty base bitmap
	{
		base := NewBitmap()
		filters := []*Bitmap{
			BitmapOf(1, 2, 3),
			BitmapOf(4, 5, 6),
		}
		expected := base.Clone()
		expected.And(FastOr(filters...))

		actual := base.Clone()
		actual.AndAny(filters...)

		if !actual.Equals(expected) {
			t.Errorf("Empty base failed: expected %v, got %v", expected, actual)
		}
	}

	// 2. All empty input bitmaps
	{
		base := BitmapOf(1, 2, 3, 4, 5)
		filters := []*Bitmap{
			NewBitmap(),
			NewBitmap(),
		}
		expected := base.Clone()
		expected.And(FastOr(filters...))

		actual := base.Clone()
		actual.AndAny(filters...)

		if !actual.Equals(expected) {
			t.Errorf("All empty filters failed: expected %v, got %v", expected, actual)
		}
	}

	// 3. Mixed empty and non-empty input bitmaps (crucial correctness test!)
	{
		base := BitmapOf(1, 2, 3, 4, 5)
		filters := []*Bitmap{
			BitmapOf(1, 2),
			NewBitmap(), // empty filter
			BitmapOf(4, 5),
		}
		expected := base.Clone()
		expected.And(FastOr(filters...))

		actual := base.Clone()
		actual.AndAny(filters...)

		if !actual.Equals(expected) {
			t.Errorf("Mixed empty filters failed: expected %v, got %v", expected, actual)
		}
	}

	// 4. Randomised fuzz-like correctness comparison
	r := rand.New(rand.NewSource(42))
	for i := 0; i < 100; i++ {
		baseSize := r.Intn(100)
		base := NewBitmap()
		for j := 0; j < baseSize; j++ {
			base.Add(uint32(r.Intn(1000)))
		}

		numFilters := r.Intn(5) + 1
		filters := make([]*Bitmap, numFilters)
		for j := 0; j < numFilters; j++ {
			filters[j] = NewBitmap()
			if r.Float64() > 0.2 { // 80% chance of being non-empty
				filterSize := r.Intn(100)
				for k := 0; k < filterSize; k++ {
					filters[j].Add(uint32(r.Intn(1000)))
				}
			}
		}

		expected := base.Clone()
		expected.And(FastOr(filters...))

		actual := base.Clone()
		actual.AndAny(filters...)

		if !actual.Equals(expected) {
			t.Fatalf("Randomised check %d failed", i)
		}
	}
}
