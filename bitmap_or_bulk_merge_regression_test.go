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

func bitmapOrBulkMergeReadFromKeys(t *testing.T, keys []uint16) *Bitmap {
	t.Helper()

	values := make([]uint32, len(keys))
	for index := range values {
		values[index] = uint32(index+1)<<16 | 1
	}
	source := BitmapOf(values...)
	serialized, err := source.ToBytes()
	if err != nil {
		t.Fatalf("serialize source: %v", err)
	}
	if binary.LittleEndian.Uint32(serialized[:4]) != serialCookieNoRunContainer {
		t.Fatal("unexpected serialized source layout")
	}
	for index, key := range keys {
		binary.LittleEndian.PutUint16(serialized[8+index*4:], key)
	}

	decoded := NewBitmap()
	if _, err := decoded.ReadFrom(bytes.NewReader(serialized)); err != nil {
		t.Fatalf("deserialize keys: %v", err)
	}
	if err := decoded.Validate(); err != ErrKeySortOrder {
		t.Fatalf("unexpected validation error: %v", err)
	}
	return decoded
}

func bitmapOrBulkMergeForwardOr(receiver, source *Bitmap) {
	pos1 := 0
	pos2 := 0
	length1 := receiver.highlowcontainer.size()
	length2 := source.highlowcontainer.size()
main:
	for (pos1 < length1) && (pos2 < length2) {
		s1 := receiver.highlowcontainer.getKeyAtIndex(pos1)
		s2 := source.highlowcontainer.getKeyAtIndex(pos2)

		for {
			if s1 < s2 {
				pos1++
				if pos1 == length1 {
					break main
				}
				s1 = receiver.highlowcontainer.getKeyAtIndex(pos1)
			} else if s1 > s2 {
				receiver.highlowcontainer.insertNewKeyValueAt(pos1, s2, source.highlowcontainer.getContainerAtIndex(pos2).clone())
				pos1++
				length1++
				pos2++
				if pos2 == length2 {
					break main
				}
				s2 = source.highlowcontainer.getKeyAtIndex(pos2)
			} else {
				newContainer := receiver.highlowcontainer.getUnionedWritableContainer(pos1, source.highlowcontainer.getContainerAtIndex(pos2))
				receiver.highlowcontainer.replaceKeyAndContainerAtIndex(pos1, s1, newContainer, false)
				pos1++
				pos2++
				if pos1 == length1 || pos2 == length2 {
					break main
				}
				s1 = receiver.highlowcontainer.getKeyAtIndex(pos1)
				s2 = source.highlowcontainer.getKeyAtIndex(pos2)
			}
		}
	}
	if pos1 == length1 {
		receiver.highlowcontainer.appendCopyMany(source.highlowcontainer, pos2, length2)
	}
}

func assertBitmapOrBulkMergeState(t *testing.T, got, want *Bitmap) {
	t.Helper()

	if !got.highlowcontainer.equals(want.highlowcontainer) {
		t.Fatalf("metadata differs: got keys %v, want %v", got.highlowcontainer.keys, want.highlowcontainer.keys)
	}
	if got.highlowcontainer.copyOnWrite != want.highlowcontainer.copyOnWrite {
		t.Fatalf("copy-on-write state differs: got %t, want %t", got.highlowcontainer.copyOnWrite, want.highlowcontainer.copyOnWrite)
	}
	if len(got.highlowcontainer.needCopyOnWrite) != len(want.highlowcontainer.needCopyOnWrite) {
		t.Fatalf("copy-on-write marker length differs: got %d, want %d", len(got.highlowcontainer.needCopyOnWrite), len(want.highlowcontainer.needCopyOnWrite))
	}
	for index, gotMarker := range got.highlowcontainer.needCopyOnWrite {
		if wantMarker := want.highlowcontainer.needCopyOnWrite[index]; gotMarker != wantMarker {
			t.Fatalf("copy-on-write marker %d differs: got %t, want %t", index, gotMarker, wantMarker)
		}
	}
	if gotErr, wantErr := got.Validate(), want.Validate(); gotErr != wantErr {
		t.Fatalf("validation differs: got %v, want %v", gotErr, wantErr)
	}
}

func TestBitmapOrBulkMergeFallsBackForUnsortedKeys(t *testing.T) {
	tests := []struct {
		name     string
		receiver *Bitmap
		source   *Bitmap
	}{
		{
			name:     "source",
			receiver: BitmapOf(uint32(2)<<16|2, uint32(4)<<16|2),
			source:   bitmapOrBulkMergeReadFromKeys(t, []uint16{1, 2, 4, 1}),
		},
		{
			name:     "receiver",
			receiver: bitmapOrBulkMergeReadFromKeys(t, []uint16{2, 4, 2}),
			source: BitmapOf(
				uint32(1)<<16|2,
				uint32(2)<<16|2,
				uint32(4)<<16|2,
				uint32(5)<<16|2,
			),
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			want := test.receiver.Clone()
			bitmapOrBulkMergeForwardOr(want, test.source)
			got := test.receiver.Clone()
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("Bitmap.Or panicked: %v", recovered)
				}
			}()
			got.Or(test.source)
			assertBitmapOrBulkMergeState(t, got, want)
		})
	}
}

func TestBitmapOrBulkMergeReadFromPropagatesUnorderedKeys(t *testing.T) {
	source := BitmapOf(
		uint32(1)<<16|1,
		uint32(2)<<16|1,
		uint32(3)<<16|1,
	)
	serialized, err := source.ToBytes()
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint32(serialized[:4]) != serialCookieNoRunContainer {
		t.Fatal("unexpected serialized source layout")
	}
	ordered := NewBitmap()
	if _, err := ordered.ReadFrom(bytes.NewReader(serialized)); err != nil {
		t.Fatal(err)
	}
	if ordered.highlowcontainer.keysMayBeUnordered {
		t.Fatal("ordered decoded keys retained the unordered-key state")
	}
	binary.LittleEndian.PutUint16(serialized[8+4:], 3)
	binary.LittleEndian.PutUint16(serialized[8+8:], 1)

	decoded := NewBitmap()
	if _, err := decoded.ReadFrom(bytes.NewReader(serialized)); err != nil {
		t.Fatal(err)
	}
	if err := decoded.Validate(); err != ErrKeySortOrder {
		t.Fatalf("unexpected validation error: %v", err)
	}

	receiver := BitmapOf(uint32(2)<<16 | 2)
	receiver.Or(decoded)
	if !receiver.highlowcontainer.keysMayBeUnordered {
		t.Fatal("receiver lost the unordered-key state")
	}

	wantKeys := []uint16{1, 2, 3, 1}
	wantValues := []uint16{1, 2, 1, 1}
	if receiver.highlowcontainer.size() != len(wantKeys) {
		t.Fatalf("unexpected container count: got %d, want %d", receiver.highlowcontainer.size(), len(wantKeys))
	}
	for index, wantKey := range wantKeys {
		if gotKey := receiver.highlowcontainer.getKeyAtIndex(index); gotKey != wantKey {
			t.Fatalf("unexpected key at %d: got %d, want %d", index, gotKey, wantKey)
		}
		container := receiver.highlowcontainer.getContainerAtIndex(index)
		if container.getCardinality() != 1 || !container.contains(wantValues[index]) {
			t.Fatalf("unexpected container at %d", index)
		}
		if receiver.highlowcontainer.needsCopyOnWrite(index) {
			t.Fatalf("unexpected copy-on-write marker at %d", index)
		}
	}
	if err := receiver.Validate(); err != ErrKeySortOrder {
		t.Fatalf("unexpected receiver validation error: %v", err)
	}
}
