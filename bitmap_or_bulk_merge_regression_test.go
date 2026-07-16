package roaring

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestBitmapOrBulkMergeCopyOnWriteTailOwnership(t *testing.T) {
	fixture := bitmapOrBulkMergeInterleavedFixture(64, true)
	left := fixture.lefts[0]
	right := fixture.rights[0]
	receiver := left.Clone()
	receiver.Or(right)

	tailIndex := right.highlowcontainer.size() - 1
	tailKey := right.highlowcontainer.getKeyAtIndex(tailIndex)
	receiverTailIndex := receiver.highlowcontainer.getIndex(tailKey)
	if receiverTailIndex < 0 {
		t.Fatal("receiver is missing the source-only tail container")
	}
	if !right.highlowcontainer.needsCopyOnWrite(tailIndex) {
		t.Fatal("source-only tail container was not marked copy-on-write")
	}
	if !receiver.highlowcontainer.needsCopyOnWrite(receiverTailIndex) {
		t.Fatal("receiver tail container was not marked copy-on-write")
	}
	if receiver.highlowcontainer.getContainerAtIndex(receiverTailIndex) != right.highlowcontainer.getContainerAtIndex(tailIndex) {
		t.Fatal("source-only tail container was not shared")
	}

	receiverValue := uint32(tailKey)<<16 | 10
	sourceValue := uint32(tailKey)<<16 | 11
	receiver.Add(receiverValue)
	if right.Contains(receiverValue) {
		t.Fatal("receiver tail mutation changed the source")
	}
	right.Add(sourceValue)
	if receiver.Contains(sourceValue) {
		t.Fatal("source tail mutation changed the receiver")
	}

	if err := receiver.Validate(); err != nil {
		t.Fatalf("receiver became invalid after tail mutations: %v", err)
	}
	if err := right.Validate(); err != nil {
		t.Fatalf("source became invalid after tail mutations: %v", err)
	}
}

func TestBitmapOrBulkMergeFallsBackForUnsortedSource(t *testing.T) {
	source := BitmapOf(
		uint32(1)<<16|1,
		uint32(2)<<16|1,
		uint32(4)<<16|1,
		uint32(5)<<16|1,
	)
	serialized, err := source.ToBytes()
	if err != nil {
		t.Fatalf("serialize source: %v", err)
	}
	if binary.LittleEndian.Uint32(serialized[:4]) != serialCookieNoRunContainer {
		t.Fatal("unexpected serialized source layout")
	}
	binary.LittleEndian.PutUint16(serialized[8+3*4:], 1)

	decoded := NewBitmap()
	if _, err := decoded.ReadFrom(bytes.NewReader(serialized)); err != nil {
		t.Fatalf("deserialize unsorted source: %v", err)
	}
	if err := decoded.Validate(); err != ErrKeySortOrder {
		t.Fatalf("unexpected validation error: %v", err)
	}

	receiver := BitmapOf(uint32(2)<<16|2, uint32(4)<<16|2)
	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("Bitmap.Or panicked for an unsorted decoded source: %v", recovered)
		}
	}()
	receiver.Or(decoded)
}
