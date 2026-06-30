package roaring

import (
	"testing"
)

func BenchmarkBatchEqual(b *testing.B) {
	bsi := setupLargeBSI(b)
	if bsi == nil {
		b.Skip("skipping, large BSI setup failed")
		return
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := bsi.BatchEqual(0, []int64{55, 57})
		_ = res
	}
}
