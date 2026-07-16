package roaring

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestBitmapOrBulkMergeMalformedSourceDoesNotPanic(t *testing.T) {
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
