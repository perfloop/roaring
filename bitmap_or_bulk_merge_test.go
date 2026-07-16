package roaring

import "testing"

var bitmapOrBulkMergeSink uint64

type bitmapOrBulkMergeCase struct {
	name                string
	receiver, source    *Bitmap
	expectedCardinality uint64
}

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

func bitmapWithOneHighKey(high int) *Bitmap {
	bitmap := NewBitmap()
	bitmap.Add(uint32(high) << 16)
	return bitmap
}

func bitmapWithSparseAlternatingHighKeys(count, stride int) *Bitmap {
	bitmap := NewBitmap()
	for i := 1; i < count-1; i += stride {
		bitmap.Add(uint32(2*i+1) << 16)
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

func benchmarkBitmapOrBulkMerge(b *testing.B, receiver, source *Bitmap, expectedCardinality uint64) {
	b.Helper()
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
	bitmapOrBulkMergeSink = total
}

func runBitmapOrBulkMergeCases(b *testing.B, cases []bitmapOrBulkMergeCase) {
	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			benchmarkBitmapOrBulkMerge(b, tc.receiver, tc.source, tc.expectedCardinality)
		})
	}
}

func bitmapOrBulkMergeAdjacentCases(count int) []bitmapOrBulkMergeCase {
	left := bitmapWithAlternatingHighKeys(count, 0, 0)
	sparse := bitmapWithSparseAlternatingHighKeys(count, 256)
	return []bitmapOrBulkMergeCase{
		{
			name:                "one-insert-beginning-4096",
			receiver:            left,
			source:              bitmapWithOneHighKey(1),
			expectedCardinality: uint64(count + 1),
		},
		{
			name:                "one-insert-middle-4096",
			receiver:            left,
			source:              bitmapWithOneHighKey(2*(count/2) + 1),
			expectedCardinality: uint64(count + 1),
		},
		{
			name:                "one-insert-end-4096",
			receiver:            left,
			source:              bitmapWithOneHighKey(2*(count-2) + 1),
			expectedCardinality: uint64(count + 1),
		},
		{
			name:                "sparse-inserts-4096",
			receiver:            left,
			source:              sparse,
			expectedCardinality: uint64(count) + sparse.GetCardinality(),
		},
	}
}

func BenchmarkBitmapOrBulkMerge(b *testing.B) {
	const count = 4096

	copyOnWriteLeft := bitmapWithSequentialHighKeys(0, count, 0)
	copyOnWriteLeft.SetCopyOnWrite(true)
	copyOnWriteRight := bitmapWithSequentialHighKeys(0, count, 1)
	copyOnWriteRight.SetCopyOnWrite(true)

	runBitmapOrBulkMergeCases(b, []bitmapOrBulkMergeCase{
		{
			name:                "interleaved-64",
			receiver:            bitmapWithAlternatingHighKeys(64, 0, 0),
			source:              bitmapWithAlternatingHighKeys(64, 1, 0),
			expectedCardinality: 128,
		},
		{
			name:                "interleaved-1024",
			receiver:            bitmapWithAlternatingHighKeys(1024, 0, 0),
			source:              bitmapWithAlternatingHighKeys(1024, 1, 0),
			expectedCardinality: 2048,
		},
		{
			name:                "interleaved-4096",
			receiver:            bitmapWithAlternatingHighKeys(count, 0, 0),
			source:              bitmapWithAlternatingHighKeys(count, 1, 0),
			expectedCardinality: 2 * count,
		},
		{
			name:                "append-only-4096",
			receiver:            bitmapWithSequentialHighKeys(0, count, 0),
			source:              bitmapWithSequentialHighKeys(count, count, 0),
			expectedCardinality: 2 * count,
		},
		{
			name:                "overlapping-4096",
			receiver:            bitmapWithSequentialHighKeys(0, count, 0),
			source:              bitmapWithSequentialHighKeys(0, count, 1),
			expectedCardinality: 2 * count,
		},
		{
			name:                "copy-on-write-4096",
			receiver:            copyOnWriteLeft,
			source:              copyOnWriteRight,
			expectedCardinality: 2 * count,
		},
	})
}

func BenchmarkBitmapOrBulkMergeAdjacent(b *testing.B) {
	runBitmapOrBulkMergeCases(b, bitmapOrBulkMergeAdjacentCases(4096))
}

func BenchmarkBitmapOrBulkMergeFresh(b *testing.B) {
	const count = 4096
	runBitmapOrBulkMergeCases(b, []bitmapOrBulkMergeCase{{
		name:                "interleaved-4096",
		receiver:            bitmapWithAlternatingHighKeys(count, 0, 0),
		source:              bitmapWithAlternatingHighKeys(count, 1, 0),
		expectedCardinality: 2 * count,
	}})
}

func BenchmarkBitmapOrBulkMergeFreshShapes(b *testing.B) {
	const count = 4096

	copyOnWriteLeft := bitmapWithSequentialHighKeys(0, count, 0)
	copyOnWriteLeft.SetCopyOnWrite(true)
	copyOnWriteRight := bitmapWithSequentialHighKeys(0, count, 1)
	copyOnWriteRight.SetCopyOnWrite(true)

	runBitmapOrBulkMergeCases(b, []bitmapOrBulkMergeCase{
		{
			name:                "interleaved-64",
			receiver:            bitmapWithAlternatingHighKeys(64, 0, 0),
			source:              bitmapWithAlternatingHighKeys(64, 1, 0),
			expectedCardinality: 128,
		},
		{
			name:                "interleaved-1024",
			receiver:            bitmapWithAlternatingHighKeys(1024, 0, 0),
			source:              bitmapWithAlternatingHighKeys(1024, 1, 0),
			expectedCardinality: 2048,
		},
		{
			name:                "append-only-4096",
			receiver:            bitmapWithSequentialHighKeys(0, count, 0),
			source:              bitmapWithSequentialHighKeys(count, count, 0),
			expectedCardinality: 2 * count,
		},
		{
			name:                "overlapping-4096",
			receiver:            bitmapWithSequentialHighKeys(0, count, 0),
			source:              bitmapWithSequentialHighKeys(0, count, 1),
			expectedCardinality: 2 * count,
		},
		{
			name:                "copy-on-write-4096",
			receiver:            copyOnWriteLeft,
			source:              copyOnWriteRight,
			expectedCardinality: 2 * count,
		},
	})
}

func BenchmarkBitmapOrBulkMergeFreshDensity(b *testing.B) {
	const count = 4096
	sparse := bitmapWithSparseAlternatingHighKeys(count, 64)
	runBitmapOrBulkMergeCases(b, []bitmapOrBulkMergeCase{{
		name:                "sparse-64-inserts-4096",
		receiver:            bitmapWithAlternatingHighKeys(count, 0, 0),
		source:              sparse,
		expectedCardinality: uint64(count) + sparse.GetCardinality(),
	}})
}

func BenchmarkBitmapOrBulkMergeFreshThreshold(b *testing.B) {
	const (
		count      = 4096
		insertions = 65
	)

	source := NewBitmap()
	for i := 1; i <= insertions; i++ {
		high := uint32(2*(i*count/(insertions+1)) + 1)
		source.Add(high << 16)
	}

	runBitmapOrBulkMergeCases(b, []bitmapOrBulkMergeCase{{
		name:                "sparse-65-inserts-4096",
		receiver:            bitmapWithAlternatingHighKeys(count, 0, 0),
		source:              source,
		expectedCardinality: count + insertions,
	}})
}

func benchmarkBitmapOrBulkMergeFreshCOWAdjacent(b *testing.B, receiver *Bitmap, source *Bitmap, probe uint32, expectedSize int) {
	b.Helper()
	b.ReportAllocs()

	var total uint64
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		target := receiver.Clone()
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
	bitmapOrBulkMergeSink = total
}

func BenchmarkBitmapOrBulkMergeFreshCOWAdjacent(b *testing.B) {
	const count = 4096

	left := bitmapWithAlternatingHighKeys(count, 0, 0)
	left.SetCopyOnWrite(true)
	sparse := bitmapWithSparseAlternatingHighKeys(count, 256)
	cases := []struct {
		name   string
		source *Bitmap
		probe  uint32
	}{
		{"one-insert-beginning-4096", bitmapWithOneHighKey(1), uint32(1) << 16},
		{"one-insert-middle-4096", bitmapWithOneHighKey(2*(count/2) + 1), uint32(2*(count/2)+1) << 16},
		{"one-insert-end-4096", bitmapWithOneHighKey(2*(count-2) + 1), uint32(2*(count-2)+1) << 16},
		{"sparse-inserts-4096", sparse, uint32(3) << 16},
	}

	for _, tc := range cases {
		tc := tc
		b.Run(tc.name, func(b *testing.B) {
			benchmarkBitmapOrBulkMergeFreshCOWAdjacent(b, left, tc.source, tc.probe, count+tc.source.highlowcontainer.size())
		})
	}
}
