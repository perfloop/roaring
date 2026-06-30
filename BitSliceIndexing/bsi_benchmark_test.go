package roaring

import (
	"math/rand"
	"testing"

	"github.com/RoaringBitmap/roaring/v2"
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

	// Test 1<<62 edge case explicitly
	bsi62 := NewBSI(1<<62, 0)
	bsi62.SetValue(10, 5)
	res = bsi62.BatchEqual(0, []int64{5})
	assert.Equal(t, uint64(1), res.GetCardinality())
	assert.True(t, res.Contains(10))
}

func TestBatchEqualConsistentWithGetValue(t *testing.T) {
	rg := rand.New(rand.NewSource(42))
	for run := 0; run < 15; run++ {
		// Create a randomized BSI
		bsi := NewDefaultBSI()
		numCols := rg.Intn(1000) + 10
		for col := 0; col < numCols; col++ {
			if rg.Float64() < 0.8 {
				val := rg.Int63n(500) - 250 // Mix of positive, zero, and negative values
				bsi.SetValue(uint64(col), val)
			}
		}

		// Generate query values (small, medium, and large list sizes to test the hybrid threshold)
		querySizes := []int{rg.Intn(10) + 1, rg.Intn(50) + 50, rg.Intn(200) + 100}
		for _, querySize := range querySizes {
			query := make([]int64, querySize)
			for i := range query {
				query[i] = rg.Int63n(600) - 300
			}

			// Ground truth
			expected := roaring.NewBitmap()
			valMap := make(map[int64]bool)
			for _, q := range query {
				valMap[q] = true
			}
			iter := bsi.GetExistenceBitmap().Iterator()
			for iter.HasNext() {
				col := iter.Next()
				val, ok := bsi.GetValue(uint64(col))
				if ok && valMap[val] {
					expected.Add(col)
				}
			}

			// Test different parallelism settings
			for _, parallelism := range []int{0, 1, 2, 4} {
				actual := bsi.BatchEqual(parallelism, query)
				if !actual.Equals(expected) {
					t.Fatalf("Mismatch in run %d querySize %d parallelism %d. Query: %v. Expected: %v, Got: %v", run, querySize, parallelism, query, expected.ToArray(), actual.ToArray())
				}
			}
		}
	}
}
