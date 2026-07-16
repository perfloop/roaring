package roaring

import (
	"fmt"
	"testing"
)

var bitmapOrBulkMergeSink uint64

func bitmapWithSequentialHighKeys(start, count int, low uint16) *Bitmap {
	bitmap := NewBitmap()
	for high := start; high < start+count; high++ {
		bitmap.Add(uint32(high)<<16 | uint32(low))
	}
	return bitmap
}

func bitmapWithAlternatingHighKeys(count, offset int, low uint16) *Bitmap {
	bitmap := NewBitmap()
	for i := 0; i < count; i++ {
		bitmap.Add(uint32(2*i+offset)<<16 | uint32(low))
	}
	return bitmap
}

func assertBitmapOrBulkMergeValid(t *testing.T, bitmap *Bitmap) {
	t.Helper()
	if err := bitmap.Validate(); err != nil {
		t.Fatalf("bitmap failed validation: %v", err)
	}
}

func TestBitmapOrBulkMerge(t *testing.T) {
	t.Run("interleaved-source-containers-are-not-aliased", func(t *testing.T) {
		receiver := bitmapWithAlternatingHighKeys(128, 0, 0)
		source := bitmapWithAlternatingHighKeys(128, 1, 0)
		expected := Or(receiver, source)
		sourceBefore := source.Clone()

		receiver.Or(source)

		if !receiver.Equals(expected) {
			t.Fatal("interleaved union differs from non-mutating Or")
		}
		if !source.Equals(sourceBefore) {
			t.Fatal("Or modified its source bitmap")
		}
		assertBitmapOrBulkMergeValid(t, receiver)

		const sourceOnlyHighKey = 1
		receiver.Add(uint32(sourceOnlyHighKey)<<16 | 1)
		if source.Contains(uint32(sourceOnlyHighKey)<<16 | 1) {
			t.Fatal("source-only container was aliased into the receiver")
		}
	})

	t.Run("append-only", func(t *testing.T) {
		receiver := bitmapWithSequentialHighKeys(0, 128, 0)
		source := bitmapWithSequentialHighKeys(128, 128, 0)
		expected := Or(receiver, source)

		receiver.Or(source)

		if !receiver.Equals(expected) {
			t.Fatal("append-only union differs from non-mutating Or")
		}
		assertBitmapOrBulkMergeValid(t, receiver)
	})

	t.Run("overlapping", func(t *testing.T) {
		receiver := bitmapWithSequentialHighKeys(0, 128, 0)
		source := bitmapWithSequentialHighKeys(0, 128, 1)
		expected := Or(receiver, source)

		receiver.Or(source)

		if !receiver.Equals(expected) {
			t.Fatal("overlapping union differs from non-mutating Or")
		}
		assertBitmapOrBulkMergeValid(t, receiver)
	})

	t.Run("copy-on-write", func(t *testing.T) {
		receiver := bitmapWithAlternatingHighKeys(128, 0, 0)
		receiver.SetCopyOnWrite(true)
		alias := receiver.Clone()
		aliasBefore := alias.Clone()
		source := bitmapWithAlternatingHighKeys(128, 1, 0)
		source.Add(1)
		expected := Or(receiver, source)
		sourceBefore := source.Clone()

		receiver.Or(source)

		if !receiver.Equals(expected) {
			t.Fatal("copy-on-write union differs from non-mutating Or")
		}
		if !alias.Equals(aliasBefore) {
			t.Fatal("Or modified a copy-on-write alias")
		}
		if !source.Equals(sourceBefore) {
			t.Fatal("Or modified its copy-on-write source")
		}
		assertBitmapOrBulkMergeValid(t, receiver)

		receiver.Add(2)
		if alias.Contains(2) {
			t.Fatal("receiver retained a shared receiver-only container")
		}
		receiver.Add(uint32(1)<<16 | 1)
		if source.Contains(uint32(1)<<16 | 1) {
			t.Fatal("receiver retained a shared source-only container")
		}
	})
}

func benchmarkBitmapOrBulkMerge(b *testing.B, receiver func() *Bitmap, source *Bitmap, expectedCardinality uint64) {
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
	bitmapOrBulkMergeSink = total
}

func BenchmarkBitmapOrBulkMerge(b *testing.B) {
	for _, count := range []int{64, 1024, 4096} {
		left := bitmapWithAlternatingHighKeys(count, 0, 0)
		right := bitmapWithAlternatingHighKeys(count, 1, 0)
		b.Run(fmt.Sprintf("interleaved-%d", count), func(b *testing.B) {
			benchmarkBitmapOrBulkMerge(b, left.Clone, right, uint64(2*count))
		})
	}

	const count = 4096

	appendLeft := bitmapWithSequentialHighKeys(0, count, 0)
	appendRight := bitmapWithSequentialHighKeys(count, count, 0)
	b.Run("append-only-4096", func(b *testing.B) {
		benchmarkBitmapOrBulkMerge(b, appendLeft.Clone, appendRight, 2*count)
	})

	overlapLeft := bitmapWithSequentialHighKeys(0, count, 0)
	overlapRight := bitmapWithSequentialHighKeys(0, count, 1)
	b.Run("overlapping-4096", func(b *testing.B) {
		benchmarkBitmapOrBulkMerge(b, overlapLeft.Clone, overlapRight, 2*count)
	})

	copyOnWriteLeft := bitmapWithSequentialHighKeys(0, count, 0)
	copyOnWriteLeft.SetCopyOnWrite(true)
	copyOnWriteRight := bitmapWithSequentialHighKeys(0, count, 1)
	copyOnWriteRight.SetCopyOnWrite(true)
	b.Run("copy-on-write-4096", func(b *testing.B) {
		benchmarkBitmapOrBulkMerge(b, copyOnWriteLeft.Clone, copyOnWriteRight, 2*count)
	})
}
