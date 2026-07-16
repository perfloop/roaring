package roaring

import "testing"

var bitmapOrBulkMergeFreshCOWAdjacentSink uint64

// benchmarkBitmapOrBulkMergeFreshCOWAdjacent measures a complete COW-clone and
// Or operation while observing the metadata size and an inserted source key.
// The COW receiver makes the per-operation reset proportional to the metadata
// arrays instead of cloning every unchanged container, so low-insertion Or work
// remains visible without charging a full cardinality scan to the benchmark.
func benchmarkBitmapOrBulkMergeFreshCOWAdjacent(b *testing.B, receiver func() *Bitmap, source *Bitmap, probe uint32, expectedSize int) {
	b.Helper()
	b.ReportAllocs()

	var total uint64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := receiver()
		target.Or(source)
		if target.highlowcontainer.size() != expectedSize || !target.Contains(probe) {
			b.StopTimer()
			b.Fatalf("unexpected union: size=%d, probe=%d", target.highlowcontainer.size(), probe)
		}
		total += uint64(target.highlowcontainer.size())
	}
	b.StopTimer()

	if total != uint64(b.N)*uint64(expectedSize) {
		b.Fatalf("unexpected metadata-size total: got %d, want %d", total, uint64(b.N)*uint64(expectedSize))
	}
	bitmapOrBulkMergeFreshCOWAdjacentSink = total
}

func BenchmarkBitmapOrBulkMergeFreshCOWAdjacent(b *testing.B) {
	const count = 4096

	left := bitmapWithAlternatingHighKeys(count, 0, 0)
	left.SetCopyOnWrite(true)
	sparseRight := NewBitmap()
	for i := 1; i < count-1; i += 256 {
		sparseRight.Add(uint32(2*i+1) << 16)
	}

	cases := []struct {
		name   string
		source *Bitmap
		probe  uint32
	}{
		{"one-insert-beginning-4096", bitmapWithOneHighKey(1), uint32(1) << 16},
		{"one-insert-middle-4096", bitmapWithOneHighKey(2*(count/2) + 1), uint32(2*(count/2)+1) << 16},
		{"one-insert-end-4096", bitmapWithOneHighKey(2*(count-2) + 1), uint32(2*(count-2)+1) << 16},
		{"sparse-inserts-4096", sparseRight, uint32(3) << 16},
	}

	for _, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			benchmarkBitmapOrBulkMergeFreshCOWAdjacent(b, left.Clone, tc.source, tc.probe, count+tc.source.highlowcontainer.size())
		})
	}
}
