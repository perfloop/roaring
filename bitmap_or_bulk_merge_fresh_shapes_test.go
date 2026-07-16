package roaring

import "testing"

func BenchmarkBitmapOrBulkMergeFreshShapes(b *testing.B) {
	const count = 4096

	interleaved64Left := bitmapWithAlternatingHighKeys(64, 0, 0)
	interleaved64Right := bitmapWithAlternatingHighKeys(64, 1, 0)
	interleaved1024Left := bitmapWithAlternatingHighKeys(1024, 0, 0)
	interleaved1024Right := bitmapWithAlternatingHighKeys(1024, 1, 0)
	appendLeft := bitmapWithSequentialHighKeys(0, count, 0)
	appendRight := bitmapWithSequentialHighKeys(count, count, 0)
	overlapLeft := bitmapWithSequentialHighKeys(0, count, 0)
	overlapRight := bitmapWithSequentialHighKeys(0, count, 1)
	copyOnWriteLeft := bitmapWithSequentialHighKeys(0, count, 0)
	copyOnWriteLeft.SetCopyOnWrite(true)
	copyOnWriteRight := bitmapWithSequentialHighKeys(0, count, 1)
	copyOnWriteRight.SetCopyOnWrite(true)

	adjacentLeft := bitmapWithAlternatingHighKeys(count, 0, 0)
	sparseRight := NewBitmap()
	for i := 1; i < count-1; i += 256 {
		sparseRight.Add(uint32(2*i+1) << 16)
	}

	cases := []struct {
		name                string
		left, right         *Bitmap
		expectedCardinality uint64
	}{
		{"interleaved-64", interleaved64Left, interleaved64Right, 128},
		{"interleaved-1024", interleaved1024Left, interleaved1024Right, 2048},
		{"append-only-4096", appendLeft, appendRight, 2 * count},
		{"overlapping-4096", overlapLeft, overlapRight, 2 * count},
		{"copy-on-write-4096", copyOnWriteLeft, copyOnWriteRight, 2 * count},
		{"one-insert-beginning-4096", adjacentLeft, bitmapWithOneHighKey(1), count + 1},
		{"one-insert-middle-4096", adjacentLeft, bitmapWithOneHighKey(2*(count/2) + 1), count + 1},
		{"one-insert-end-4096", adjacentLeft, bitmapWithOneHighKey(2*(count-2) + 1), count + 1},
		{"sparse-inserts-4096", adjacentLeft, sparseRight, count + sparseRight.GetCardinality()},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkBitmapOrBulkMergeFresh(b, tc.left.Clone, tc.right, tc.expectedCardinality)
		})
	}
}
