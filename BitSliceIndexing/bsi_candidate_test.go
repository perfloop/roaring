package roaring

import (
	"testing"

	"github.com/RoaringBitmap/roaring/v2"
	"github.com/stretchr/testify/assert"
)

func TestBatchEqualParallelBSIScanHelperAssertion(t *testing.T) {
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

func TestBatchEqualParallelBSIScanHelperValsAssertion(t *testing.T) {
	unsortedVals := []uint64{10, 5, 20}
	sortedCols := []uint32{5, 10, 20}
	dummyBA := []*roaring.Bitmap{roaring.NewBitmap()}

	t.Run("ParallelBSIScanHelper_UnsortedVals", func(t *testing.T) {
		defer func() {
			r := recover()
			if r == nil {
				t.Errorf("Expected ParallelBSIScanHelper to panic on unsorted vals")
			}
			msg, ok := r.(string)
			if !ok || msg != "ParallelBSIScanHelper: input vals must be sorted in ascending order" {
				t.Errorf("Expected specific panic message, got: %v", r)
			}
		}()
		_ = roaring.ParallelBSIScanHelper(sortedCols, dummyBA, 1, unsortedVals)
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
