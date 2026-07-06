package roaring

var IsCandidate = true

type activeCont struct {
	c     container
	shift uint
}

// ParallelBSIScanHelper processes a batch of column IDs (cols) against the given bitplanes (bA)
// and returns a new roaring.Bitmap containing the matching columns using inline binary search.
func ParallelBSIScanHelper(cols []uint32, bA []*Bitmap, bitCount int, vals []uint64) *Bitmap {
	// Guard the sorted column ID assumption
	for i := 1; i < len(cols); i++ {
		if cols[i] < cols[i-1] {
			panic("ParallelBSIScanHelper: input cols must be sorted in ascending order")
		}
	}

	out := NewBitmap()
	nCols := len(cols)
	if nCols == 0 {
		return out
	}

	var curIndex []int
	if bitCount <= 128 {
		var curIndexBuf [128]int
		curIndex = curIndexBuf[:bitCount]
	} else {
		curIndex = make([]int, bitCount)
	}

	var iCol int
	for iCol < nCols {
		col := cols[iCol]
		hb := uint16(col >> 16)

		// Fetch and pre-filter non-nil containers for this hb across all planes
		var activeBuf [128]activeCont
		var active []activeCont
		if bitCount <= 128 {
			active = activeBuf[:0]
		} else {
			active = make([]activeCont, 0, bitCount)
		}

		for p := 0; p < bitCount; p++ {
			ra := bA[p].highlowcontainer
			idx := ra.binarySearch(int64(curIndex[p]), int64(len(ra.keys)), hb)
			if idx >= 0 {
				curIndex[p] = idx
				active = append(active, activeCont{
					c:     ra.containers[idx],
					shift: uint(p),
				})
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
			overflow := false
			lb := uint16(currCol & 0xffff)
			for p := 0; p < len(active); p++ {
				ac := &active[p]
				if ac.c.contains(lb) {
					if ac.shift >= 64 {
						overflow = true
						break
					}
					val |= uint64(1) << ac.shift
				}
			}

			if !overflow {
				// Binary search inline on vals
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
			}
			iCol++
		}
	}
	return out
}
