package roaring

import "testing"

var bitmapOrBulkMergeFreshSink uint64

func benchmarkBitmapOrBulkMergeFresh(b *testing.B, receiver func() *Bitmap, source *Bitmap, expectedCardinality uint64) {
	b.ReportAllocs()
	var total uint64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := receiver()
		target.Or(source)
		total += target.GetCardinality()
	}
	b.StopTimer()

	if total != uint64(b.N)*expectedCardinality {
		b.Fatalf("unexpected cardinality total: got %d, want %d", total, uint64(b.N)*expectedCardinality)
	}
	bitmapOrBulkMergeFreshSink = total
}

func BenchmarkBitmapOrBulkMergeFresh(b *testing.B) {
	const count = 4096
	left := bitmapWithAlternatingHighKeys(count, 0, 0)
	right := bitmapWithAlternatingHighKeys(count, 1, 0)

	b.Run("interleaved-4096", func(b *testing.B) {
		benchmarkBitmapOrBulkMergeFresh(b, left.Clone, right, 2*count)
	})
}
