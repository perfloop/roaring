//go:build perfloopprobe

package roaring

var bitmapOrBulkMergeProbeInsertCalls int
var bitmapOrBulkMergeProbeBulkCalls int

func bitmapOrBulkMergeProbeInsert() {
	bitmapOrBulkMergeProbeInsertCalls++
}

func bitmapOrBulkMergeProbeBulk() {
	bitmapOrBulkMergeProbeBulkCalls++
}
