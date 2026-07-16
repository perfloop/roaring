package roaring

import "testing"

func BenchmarkBitmapOrBulkMergeFreshThreshold(b *testing.B) {
	const (
		count      = 4096
		insertions = 65
	)

	left := bitmapWithAlternatingHighKeys(count, 0, 0)
	source := NewBitmap()
	for i := 1; i <= insertions; i++ {
		high := uint32(2*(i*count/(insertions+1)) + 1)
		source.Add(high << 16)
	}

	b.Run("sparse-65-inserts-4096", func(b *testing.B) {
		benchmarkBitmapOrBulkMergeFresh(b, left.Clone, source, count+insertions)
	})
}
