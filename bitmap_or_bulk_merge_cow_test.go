package roaring

import "testing"

func TestBitmapOrBulkMergeCopyOnWriteTrailingSource(t *testing.T) {
	const count = 128

	receiver := bitmapWithAlternatingHighKeys(count, 0, 0)
	receiver.SetCopyOnWrite(true)
	receiverAlias := receiver.Clone()

	source := bitmapWithAlternatingHighKeys(count, 1, 0)
	source.SetCopyOnWrite(true)
	sourceAlias := source.Clone()

	expected := Or(receiver, source)
	receiver.Or(source)

	if !receiver.Equals(expected) {
		t.Fatal("copy-on-write union with a trailing source key differs from Or")
	}
	if receiverAlias.Contains(uint32(1) << 16) {
		t.Fatal("Or modified the receiver copy-on-write alias")
	}
	if !source.Equals(sourceAlias) {
		t.Fatal("Or modified the source copy-on-write alias")
	}
	assertBitmapOrBulkMergeValid(t, receiver)

	trailingValue := uint32(2*count-1) << 16
	receiver.Add(trailingValue | 1)
	if source.Contains(trailingValue|1) || sourceAlias.Contains(trailingValue|1) {
		t.Fatal("receiver mutation reached the trailing source container")
	}

	source.Add(trailingValue | 2)
	if receiver.Contains(trailingValue|2) || sourceAlias.Contains(trailingValue|2) {
		t.Fatal("source mutation reached the imported trailing container")
	}
	assertBitmapOrBulkMergeValid(t, receiver)
	assertBitmapOrBulkMergeValid(t, source)
}
