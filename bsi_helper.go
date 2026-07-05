package roaring

type activeCont struct {
	c     container
	shift uint
}

// ParallelBSIScanHelper processes a batch of column IDs (cols) against the given bitplanes (bA)
// and returns a new roaring.Bitmap containing the matching columns.
// This is extremely optimized for BSI because it avoids redundant binary searches on the container arrays.
func ParallelBSIScanHelper(cols []uint32, bA []*Bitmap, bitCount int, denseLookup []bool, maxVal uint64) *Bitmap {
	out := NewBitmap()
	nCols := len(cols)
	var iCol int
	var curIndex [64]int
	for iCol < nCols {
		col := cols[iCol]
		hb := uint16(col >> 16)

		// Fetch and pre-filter non-nil containers for this hb across all planes
		var active [64]activeCont
		var activeCount int
		for p := 0; p < bitCount && p < 64; p++ {
			ra := bA[p].highlowcontainer
			idx := ra.binarySearch(int64(curIndex[p]), int64(len(ra.keys)), hb)
			if idx >= 0 {
				curIndex[p] = idx
				active[activeCount] = activeCont{
					c:     ra.containers[idx],
					shift: uint(p),
				}
				activeCount++
			} else {
				curIndex[p] = -idx - 1
			}
		}

		// Process all columns in the batch that share this hb
		for iCol < nCols {
			currCol := cols[iCol]
			currHb := uint16(currCol >> 16)
			if currHb != hb {
				break
			}

			val := uint64(0)
			lb := uint16(currCol & 0xffff)
			for p := 0; p < activeCount; p++ {
				ac := &active[p]
				if ac.c.contains(lb) {
					val |= uint64(1) << ac.shift
				}
			}

			if val <= maxVal && denseLookup[val] {
				out.Add(currCol)
			}
			iCol++
		}
	}
	return out
}

// ParallelBSIScanHelperNoLookup processes a batch of column IDs (cols) against the given bitplanes (bA)
// when denseLookup is not available, using inline binary search on vals.
func ParallelBSIScanHelperNoLookup(cols []uint32, bA []*Bitmap, bitCount int, vals []uint64) *Bitmap {
	out := NewBitmap()
	nCols := len(cols)
	var iCol int
	var curIndex [64]int
	for iCol < nCols {
		col := cols[iCol]
		hb := uint16(col >> 16)

		// Fetch and pre-filter non-nil containers for this hb across all planes
		var active [64]activeCont
		var activeCount int
		for p := 0; p < bitCount && p < 64; p++ {
			ra := bA[p].highlowcontainer
			idx := ra.binarySearch(int64(curIndex[p]), int64(len(ra.keys)), hb)
			if idx >= 0 {
				curIndex[p] = idx
				active[activeCount] = activeCont{
					c:     ra.containers[idx],
					shift: uint(p),
				}
				activeCount++
			} else {
				curIndex[p] = -idx - 1
			}
		}

		// Process all columns in the batch that share this hb
		for iCol < nCols {
			currCol := cols[iCol]
			currHb := uint16(currCol >> 16)
			if currHb != hb {
				break
			}

			val := uint64(0)
			lb := uint16(currCol & 0xffff)
			for p := 0; p < activeCount; p++ {
				ac := &active[p]
				if ac.c.contains(lb) {
					val |= uint64(1) << ac.shift
				}
			}

			// Binary search inline
			l, r := 0, len(vals)-1
			found := false
			for l <= r {
				m := (l + r) >> 1
				v := vals[m]
				if v == val {
					found = true
					break
				}
				if v < val {
					l = m + 1
				} else {
					r = m - 1
				}
			}
			if found {
				out.Add(currCol)
			}
			iCol++
		}
	}
	return out
}
