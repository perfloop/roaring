package roaring

import (
	"bytes"
	"encoding/binary"
	"testing"
)

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

func TestBitmapOrBulkMergeMalformedSourceFallback(t *testing.T) {
	source := NewBitmap()
	for _, high := range []uint32{1, 2, 3, 4} {
		source.Add(high << 16)
	}
	data, err := source.ToBytes()
	if err != nil {
		t.Fatalf("serialize source: %v", err)
	}

	for i, key := range []uint16{1, 3, 4, 2} {
		binary.LittleEndian.PutUint16(data[8+i*4:], key)
	}

	malformed := NewBitmap()
	if _, err := malformed.ReadFrom(bytes.NewReader(data)); err != nil {
		t.Fatalf("ReadFrom rejected the modified descriptor keys: %v", err)
	}
	if err := malformed.Validate(); err == nil {
		t.Fatal("modified descriptor keys unexpectedly validated")
	}

	receiver := NewBitmap()
	for _, high := range []uint32{2, 3, 4} {
		receiver.Add(high << 16)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Or panicked for malformed source metadata: %v", recovered)
		}
	}()
	receiver.Or(malformed)
}
