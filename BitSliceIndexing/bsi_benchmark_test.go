package roaring

import (
	"testing"

	"github.com/stretchr/testify/assert"
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

func TestBatchEqualEdgeCases(t *testing.T) {
	// 1. Empty or Nil inputs
	bsi := NewDefaultBSI()
	res := bsi.BatchEqual(0, nil)
	assert.True(t, res.IsEmpty())

	res = bsi.BatchEqual(0, []int64{})
	assert.True(t, res.IsEmpty())

	// 2. Set some values
	bsi.SetValue(10, 42)
	bsi.SetValue(20, 100)
	bsi.SetValue(30, 42)
	bsi.SetValue(40, -5)

	// Test matching positive values
	res = bsi.BatchEqual(0, []int64{42})
	assert.Equal(t, uint64(2), res.GetCardinality())
	assert.True(t, res.Contains(10))
	assert.True(t, res.Contains(30))

	// Test matching multiple values including non-existent and duplicates
	res = bsi.BatchEqual(0, []int64{42, 100, 42, 999})
	assert.Equal(t, uint64(3), res.GetCardinality())
	assert.True(t, res.Contains(10))
	assert.True(t, res.Contains(20))
	assert.True(t, res.Contains(30))

	// Test negative value
	res = bsi.BatchEqual(0, []int64{-5})
	assert.Equal(t, uint64(1), res.GetCardinality())
	assert.True(t, res.Contains(40))
}
