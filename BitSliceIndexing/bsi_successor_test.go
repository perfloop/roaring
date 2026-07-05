package roaring

import (
	"math/rand"
	"testing"

	"github.com/RoaringBitmap/roaring/v2"
)

func TestBatchEqualLargeQueryValues(t *testing.T) {
	rg := rand.New(rand.NewSource(12345))
	for run := 0; run < 10; run++ {
		// Create a randomized BSI with large values >= 1,048,576
		bsi := NewDefaultBSI()
		numCols := rg.Intn(500) + 50
		for col := 0; col < numCols; col++ {
			if rg.Float64() < 0.8 {
				// Generate some large positive values around 1,048,576
				val := rg.Int63n(100000) + 1048500
				bsi.SetValue(uint64(col), val)
			}
		}

		// Generate query values containing values >= 1,048,576 (above the dense threshold)
		querySize := rg.Intn(100) + 20
		query := make([]int64, querySize)
		for i := range query {
			query[i] = rg.Int63n(100100) + 1048500
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
				t.Fatalf("Mismatch with large query values in run %d parallelism %d. Expected: %v, Got: %v", run, parallelism, expected.ToArray(), actual.ToArray())
			}
		}
	}
}
