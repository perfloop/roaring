package roaring

import (
	"testing"
)

func BenchmarkBatchEqual(b *testing.B) {
	bsi := setupLargeBSI(b)
	if bsi == nil {
		b.Skip("Large BSI not available")
	}
	values := []int64{55, 57, 59, 61, 63}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = bsi.BatchEqual(0, values)
	}
}
