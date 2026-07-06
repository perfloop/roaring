package roaring

import (
	"fmt"
	"math/rand"
	"testing"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/stretchr/testify/assert"
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

func TestParallelBSIScanHelperAssertion(t *testing.T) {
	unsortedCols := []uint32{10, 5, 20}
	sortedCols := []uint32{5, 10, 20}
	emptyCols := []uint32{}

	t.Run("ParallelBSIScanHelper_Unsorted", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected ParallelBSIScanHelper to panic on unsorted cols")
			}
			msg, ok := r.(string)
			if !ok || msg != "ParallelBSIScanHelper: input cols must be sorted in ascending order" {
				t.Errorf("Expected specific panic message, got: %v", r)
			}
		}()
		_ = roaring.ParallelBSIScanHelper(unsortedCols, nil, 0, nil)
	})

	t.Run("ParallelBSIScanHelper_SortedAndEmpty", func(t *testing.T) {
		dummyBA := []*roaring.Bitmap{roaring.NewBitmap()}
		vals := []uint64{0, 1}
		_ = roaring.ParallelBSIScanHelper(sortedCols, dummyBA, 1, vals)
		_ = roaring.ParallelBSIScanHelper(emptyCols, dummyBA, 1, vals)
	})
}

func TestBatchEqualManyBitplanes(t *testing.T) {
	// Create a BSI with 70 bitplanes (more than 64!)
	bsi := NewDefaultBSI()

	bsi.eBM.Add(1)
	bsi.eBM.Add(2)
	bsi.eBM.Add(3)
	bsi.eBM.Add(4)

	bsi.bA = make([]*roaring.Bitmap, 70)
	for i := range bsi.bA {
		bsi.bA[i] = roaring.NewBitmap()
	}

	// Column 1: value is 1<<65 (so only plane 65 has it)
	bsi.bA[65].Add(1)
	// Column 2: value is 1<<3 (so only plane 3 has it)
	bsi.bA[3].Add(2)
	// Column 3: value is (1<<65) | (1<<3)
	bsi.bA[65].Add(3)
	bsi.bA[3].Add(3)
	// Column 4: value is 1<<3
	bsi.bA[3].Add(4)

	query := []int64{8}

	// Test Trie Path
	resTrie := bsi.BatchEqual(0, query)
	assert.True(t, resTrie.Contains(2))
	assert.True(t, resTrie.Contains(4))
	assert.False(t, resTrie.Contains(1))
	assert.False(t, resTrie.Contains(3))

	// Test Parallel Scan Path
	if roaring.IsCandidate {
		vals := []uint64{8}
		resScan := bsi.parallelBatchEqualScan(1, vals)
		assert.True(t, resScan.Contains(2))
		assert.True(t, resScan.Contains(4))
		assert.False(t, resScan.Contains(1))
		assert.False(t, resScan.Contains(3))
	}
}

func BenchmarkBatchEqualM128ScatteredLargeValues(b *testing.B) {
	rg := rand.New(rand.NewSource(12345))
	bsi := NewDefaultBSI()
	numCols := 50000
	for col := 0; col < numCols; col++ {
		if rg.Float64() < 0.8 {
			val := rg.Int63n(100000) + 1048500
			bsi.SetValue(uint64(col), val)
		}
	}

	vals := make([]int64, 128)
	for i := range vals {
		vals[i] = rg.Int63n(100000) + 1048500
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		res := bsi.BatchEqual(0, vals)
		_ = res
	}
}

func BenchmarkBatchEqualSweepBranchCount(b *testing.B) {
	bsi := setupLargeBSI(b)
	if bsi == nil {
		b.Skip("skipping, large BSI setup failed")
		return
	}

	for _, count := range []int{2, 4, 8, 12, 14, 15, 16, 18, 20, 24, 32} {
		b.Run(fmt.Sprintf("BranchCount_%d", count), func(b *testing.B) {
			vals := make([]int64, count)
			for i := range vals {
				vals[i] = int64(i) * 5
			}
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				res := bsi.BatchEqual(0, vals)
				_ = res
			}
		})
	}
}
