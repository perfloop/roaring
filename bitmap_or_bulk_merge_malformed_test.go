package roaring

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestBitmapOrBulkMergeMalformedSourceDoesNotPanic(t *testing.T) {
	// Leave a malformed suffix after the threshold to exercise mergeOrFrom's fallback.
	const sourceCount = bulkMergeMinInsertions + 4

	source := NewBitmap()
	for i := 0; i < sourceCount; i++ {
		source.Add(uint32(2*i+1) << 16)
	}
	data, err := source.ToBytes()
	if err != nil {
		t.Fatalf("serialize source: %v", err)
	}

	for i := 0; i < sourceCount; i++ {
		key := uint16(2*i + 1)
		if i == sourceCount-1 {
			key = uint16(2*(sourceCount-3) + 1)
		}
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
	for i := 0; i <= sourceCount; i++ {
		receiver.Add(uint32(2*i) << 16)
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Or panicked for malformed source metadata after the merge threshold: %v", recovered)
		}
	}()
	receiver.Or(malformed)
}
