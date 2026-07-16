package roaring

import "testing"

func bitmapWithOneHighKey(high int) *Bitmap {
	bitmap := NewBitmap()
	bitmap.Add(uint32(high) << 16)
	return bitmap
}

func BenchmarkBitmapOrBulkMergeAdjacent(b *testing.B) {
	const count = 4096
	left := bitmapWithAlternatingHighKeys(count, 0, 0)

	cases := []struct {
		name   string
		source *Bitmap
	}{
		{
			name:   "one-insert-beginning-4096",
			source: bitmapWithOneHighKey(1),
		},
		{
			name:   "one-insert-middle-4096",
			source: bitmapWithOneHighKey(2*(count/2) + 1),
		},
		{
			name:   "one-insert-end-4096",
			source: bitmapWithOneHighKey(2*(count-2) + 1),
		},
		{
			name: "sparse-inserts-4096",
			source: func() *Bitmap {
				bitmap := NewBitmap()
				for i := 1; i < count-1; i += 256 {
					bitmap.Add(uint32(2*i+1) << 16)
				}
				return bitmap
			}(),
		},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkBitmapOrBulkMerge(b, left.Clone, tc.source, uint64(count)+tc.source.GetCardinality())
		})
	}
}
