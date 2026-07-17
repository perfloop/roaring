package roaring

import (
	"testing"
)

func BenchmarkAndAnyFastEmptyBase(b *testing.B) {
	base := NewBitmap()
	filters := []*Bitmap{
		BitmapOf(1, 2, 3),
		BitmapOf(4, 5, 6),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		base.AndAny(filters...)
	}
}

func BenchmarkAndAnyFastEmptyFilters(b *testing.B) {
	base := BitmapOf(1, 2, 3, 4, 5)
	filters := []*Bitmap{
		NewBitmap(),
		NewBitmap(),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clone := base.Clone()
		clone.AndAny(filters...)
	}
}

func BenchmarkAndAnyFastAllocPool(b *testing.B) {
	base := BitmapOf(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	filters := []*Bitmap{
		BitmapOf(1, 2, 3),
		BitmapOf(3, 4, 5),
		BitmapOf(5, 6, 7),
		BitmapOf(7, 8, 9),
		BitmapOf(9, 10, 1),
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clone := base.Clone()
		clone.AndAny(filters...)
	}
}

func BenchmarkAndAnySingleFilter(b *testing.B) {
	base := BitmapOf(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	filter := BitmapOf(5, 6, 7, 8, 9, 10, 11, 12, 13, 14)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clone := base.Clone()
		clone.AndAny(filter)
	}
}

func BenchmarkAndAnyFastAboveThreshold(b *testing.B) {
	base := BitmapOf(1, 2, 3, 4, 5, 6, 7, 8, 9, 10)
	var filters []*Bitmap
	for i := 0; i < 20; i++ {
		filters = append(filters, BitmapOf(uint32(i), uint32(i+1), uint32(i+2)))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		clone := base.Clone()
		clone.AndAny(filters...)
	}
}
