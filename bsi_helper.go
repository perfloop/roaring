package roaring

var IsCandidate = false

type activeCont struct {
	c     container
	shift uint
}

func ParallelBSIScanHelper(cols []uint32, bA []*Bitmap, bitCount int, denseLookup []bool, maxVal uint64) *Bitmap {
	for i := 1; i < len(cols); i++ {
		if cols[i] < cols[i-1] {
			panic("ParallelBSIScanHelper: input cols must be sorted in ascending order")
		}
	}
	return NewBitmap()
}

func ParallelBSIScanHelperNoLookup(cols []uint32, bA []*Bitmap, bitCount int, vals []uint64) *Bitmap {
	for i := 1; i < len(cols); i++ {
		if cols[i] < cols[i-1] {
			panic("ParallelBSIScanHelperNoLookup: input cols must be sorted in ascending order")
		}
	}
	return NewBitmap()
}
