package roaring

import "testing"

var bitmapOrBulkMergeFreshDensitySink uint64

func bitmapOrBulkMergeFreshDensitySequential(start, count int, low uint16) *Bitmap {
	bitmap := NewBitmap()
	for high := start; high < start+count; high++ {
		bitmap.Add(uint32(high)<<16 | uint32(low))
	}
	return bitmap
}

func bitmapOrBulkMergeFreshDensityAlternating(count, offset int, low uint16) *Bitmap {
	bitmap := NewBitmap()
	for i := 0; i < count; i++ {
		bitmap.Add(uint32(2*i+offset)<<16 | uint32(low))
	}
	return bitmap
}

func bitmapOrBulkMergeFreshDensityOne(high int) *Bitmap {
	bitmap := NewBitmap()
	bitmap.Add(uint32(high) << 16)
	return bitmap
}

func bitmapOrBulkMergeFreshDensitySparse(count, stride int) *Bitmap {
	bitmap := NewBitmap()
	for i := 1; i < count-1; i += stride {
		bitmap.Add(uint32(2*i+1) << 16)
	}
	return bitmap
}

func benchmarkBitmapOrBulkMergeFreshDensity(b *testing.B, receiver, source *Bitmap, expectedCardinality uint64) {
	b.ReportAllocs()
	var total uint64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := receiver.Clone()
		target.Or(source)
		total += target.GetCardinality()
	}
	b.StopTimer()

	if total != uint64(b.N)*expectedCardinality {
		b.Fatalf("unexpected cardinality total: got %d, want %d", total, uint64(b.N)*expectedCardinality)
	}
	bitmapOrBulkMergeFreshDensitySink = total
}

func BenchmarkBitmapOrBulkMergeFreshDensity(b *testing.B) {
	const count = 4096

	interleavedLeft := bitmapOrBulkMergeFreshDensityAlternating(count, 0, 0)
	interleavedRight := bitmapOrBulkMergeFreshDensityAlternating(count, 1, 0)
	appendLeft := bitmapOrBulkMergeFreshDensitySequential(0, count, 0)
	appendRight := bitmapOrBulkMergeFreshDensitySequential(count, count, 0)
	overlapLeft := bitmapOrBulkMergeFreshDensitySequential(0, count, 0)
	overlapRight := bitmapOrBulkMergeFreshDensitySequential(0, count, 1)
	copyOnWriteLeft := bitmapOrBulkMergeFreshDensitySequential(0, count, 0)
	copyOnWriteLeft.SetCopyOnWrite(true)
	copyOnWriteRight := bitmapOrBulkMergeFreshDensitySequential(0, count, 1)
	copyOnWriteRight.SetCopyOnWrite(true)
	adjacentLeft := bitmapOrBulkMergeFreshDensityAlternating(count, 0, 0)
	sparse16Right := bitmapOrBulkMergeFreshDensitySparse(count, 256)
	sparse64Right := bitmapOrBulkMergeFreshDensitySparse(count, 64)

	cases := []struct {
		name                string
		left, right         *Bitmap
		expectedCardinality uint64
	}{
		{"interleaved-4096", interleavedLeft, interleavedRight, 2 * count},
		{"append-only-4096", appendLeft, appendRight, 2 * count},
		{"overlapping-4096", overlapLeft, overlapRight, 2 * count},
		{"copy-on-write-4096", copyOnWriteLeft, copyOnWriteRight, 2 * count},
		{"one-insert-beginning-4096", adjacentLeft, bitmapOrBulkMergeFreshDensityOne(1), count + 1},
		{"one-insert-middle-4096", adjacentLeft, bitmapOrBulkMergeFreshDensityOne(2*(count/2) + 1), count + 1},
		{"one-insert-end-4096", adjacentLeft, bitmapOrBulkMergeFreshDensityOne(2*(count-2) + 1), count + 1},
		{"sparse-16-inserts-4096", adjacentLeft, sparse16Right, count + sparse16Right.GetCardinality()},
		{"sparse-64-inserts-4096", adjacentLeft, sparse64Right, count + sparse64Right.GetCardinality()},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkBitmapOrBulkMergeFreshDensity(b, tc.left, tc.right, tc.expectedCardinality)
		})
	}
}
