//go:build perfloopprobe

package roaring

import "testing"

func TestBitmapOrBulkMergeMetadataPath(t *testing.T) {
	fixture := bitmapOrBulkMergeInterleavedFixture(4096, false)
	bitmapOrBulkMergeProbeInsertCalls = 0
	bitmapOrBulkMergeProbeBulkCalls = 0

	receiver := fixture.lefts[0].Clone()
	receiver.Or(fixture.rights[0])
	if !receiver.Equals(fixture.wants[0]) {
		t.Fatal("unexpected interleaved union")
	}
	if bitmapOrBulkMergeProbeInsertCalls == 0 && bitmapOrBulkMergeProbeBulkCalls == 0 {
		t.Fatal("interleaved union did not execute a metadata placement path")
	}
	if bitmapOrBulkMergeProbeInsertCalls != 0 && bitmapOrBulkMergeProbeBulkCalls != 0 {
		t.Fatal("interleaved union mixed insertion and bulk metadata placement")
	}
	t.Logf("metadata path counts: inserts=%d bulk-merges=%d", bitmapOrBulkMergeProbeInsertCalls, bitmapOrBulkMergeProbeBulkCalls)
}
