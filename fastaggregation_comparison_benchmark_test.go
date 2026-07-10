package roaring

import (
	"fmt"
	"testing"
)

type origItem struct {
	value    *Bitmap
	priority int
}

type origPriorityQueue []*origItem

func (pq origPriorityQueue) Len() int { return len(pq) }
func (pq origPriorityQueue) Less(i, j int) bool {
	return pq[i].value.GetSizeInBytes() < pq[j].value.GetSizeInBytes()
}
func (pq origPriorityQueue) Swap(i, j int) { pq[i], pq[j] = pq[j], pq[i] }
func (pq *origPriorityQueue) Push(x interface{}) {
	item := x.(*origItem)
	*pq = append(*pq, item)
}
func (pq *origPriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

func originalBinaryReduction(bitmaps ...*Bitmap) *Bitmap {
	if len(bitmaps) == 0 {
		return NewBitmap()
	} else if len(bitmaps) == 1 {
		if bitmaps[0] == nil {
			return nil
		}
		return bitmaps[0].Clone()
	}

	nonEmpty := make([]*Bitmap, 0, len(bitmaps))
	for _, bm := range bitmaps {
		if bm != nil && bm.highlowcontainer.size() > 0 {
			nonEmpty = append(nonEmpty, bm)
		}
	}
	if len(nonEmpty) == 0 {
		return NewBitmap()
	} else if len(nonEmpty) == 1 {
		return nonEmpty[0].Clone()
	}

	pq := make(origPriorityQueue, len(nonEmpty))
	for i, bm := range nonEmpty {
		pq[i] = &origItem{bm, i}
	}
	origHeapInit(&pq)

	for pq.Len() > 1 {
		x1 := origHeapPop(&pq)
		x2 := origHeapPop(&pq)
		origHeapPush(&pq, &origItem{Xor(x1.value, x2.value), 0})
	}
	return origHeapPop(&pq).value
}

func origHeapInit(pq *origPriorityQueue) {
	n := len(*pq)
	for i := n/2 - 1; i >= 0; i-- {
		origDown(pq, i, n)
	}
}

func origHeapPush(pq *origPriorityQueue, item *origItem) {
	*pq = append(*pq, item)
	origUp(pq, len(*pq)-1)
}

func origHeapPop(pq *origPriorityQueue) *origItem {
	n := len(*pq) - 1
	(*pq).Swap(0, n)
	origDown(pq, 0, n)
	item := (*pq)[n]
	*pq = (*pq)[0:n]
	return item
}

func origUp(pq *origPriorityQueue, j int) {
	for {
		i := (j - 1) / 2
		if i == j || !(*pq).Less(j, i) {
			break
		}
		(*pq).Swap(i, j)
		j = i
	}
}

func origDown(pq *origPriorityQueue, i0, n int) bool {
	i := i0
	for {
		j1 := 2*i + 1
		if j1 >= n || j1 < 0 {
			break
		}
		j := j1
		if j2 := j1 + 1; j2 < n && (*pq).Less(j2, j1) {
			j = j2
		}
		if !(*pq).Less(j, i) {
			break
		}
		(*pq).Swap(i, j)
		i = j
	}
	return i > i0
}

func makeAlignedDuplicate(k int) []*Bitmap {
	bms := make([]*Bitmap, k)
	for i := 0; i < k; i++ {
		bm := NewBitmap()
		for key := uint32(0); key < 10; key++ {
			base := key * 65536
			for j := uint32(0); j < 100; j++ {
				bm.Add(base + j*7 + uint32(i))
			}
		}
		bms[i] = bm
	}
	return bms
}

func makeDisjointInterleaved(k int) []*Bitmap {
	bms := make([]*Bitmap, k)
	for i := 0; i < k; i++ {
		bm := NewBitmap()
		for step := uint32(0); step < 10; step++ {
			key := uint32(i) + step*uint32(k)
			base := key * 65536
			for j := uint32(0); j < 100; j++ {
				bm.Add(base + j*7)
			}
		}
		bms[i] = bm
	}
	return bms
}

func makeSkewedSizes(k int) []*Bitmap {
	bms := make([]*Bitmap, k)
	for i := 0; i < k; i++ {
		bm := NewBitmap()
		numKeys := 2
		if i == 0 {
			numKeys = 40
		}
		for key := uint32(0); key < uint32(numKeys); key++ {
			base := key * 65536
			numVals := 10
			if i == 0 {
				numVals = 1000
			}
			for j := uint32(0); j < uint32(numVals); j++ {
				bm.Add(base + j*5)
			}
		}
		bms[i] = bm
	}
	return bms
}

func makeMixedContainers(k int) []*Bitmap {
	bms := make([]*Bitmap, k)
	for i := 0; i < k; i++ {
		bm := NewBitmap()
		for j := uint32(0); j < 100; j++ {
			bm.Add(j * 10)
		}
		for j := uint32(0); j < 5000; j++ {
			bm.Add(65536 + j)
		}
		for j := uint32(0); j < 1000; j++ {
			bm.Add(2*65536 + j)
		}
		bm.RunOptimize()
		bms[i] = bm
	}
	return bms
}

func BenchmarkCompare(b *testing.B) {
	ks := []int{2, 4, 8, 16, 32, 64, 128}
	layouts := []string{"aligned", "disjoint", "skewed", "mixed"}
	contenders := []struct {
		name string
		fn   func(...*Bitmap) *Bitmap
	}{
		{"binary", originalBinaryReduction},
		{"heapxor", HeapXor},
	}

	for _, k := range ks {
		for _, layout := range layouts {
			var bms []*Bitmap
			switch layout {
			case "aligned":
				bms = makeAlignedDuplicate(k)
			case "disjoint":
				bms = makeDisjointInterleaved(k)
			case "skewed":
				bms = makeSkewedSizes(k)
			case "mixed":
				bms = makeMixedContainers(k)
			}

			for _, cont := range contenders {
				subName := fmt.Sprintf("k=%d/layout=%s/contender=%s", k, layout, cont.name)
				b.Run(subName, func(b *testing.B) {
					b.ResetTimer()
					for i := 0; i < b.N; i++ {
						res := cont.fn(bms...)
						_ = res.GetCardinality()
					}
				})
			}
		}
	}
}

func TestCompareCorrectness(t *testing.T) {
	ks := []int{2, 4, 8, 16, 32, 64, 128}
	layouts := []string{"aligned", "disjoint", "skewed", "mixed"}
	for _, k := range ks {
		for _, layout := range layouts {
			var bms []*Bitmap
			switch layout {
			case "aligned":
				bms = makeAlignedDuplicate(k)
			case "disjoint":
				bms = makeDisjointInterleaved(k)
			case "skewed":
				bms = makeSkewedSizes(k)
			case "mixed":
				bms = makeMixedContainers(k)
			}

			resBinary := originalBinaryReduction(bms...)
			resHeapXor := HeapXor(bms...)

			if !resBinary.Equals(resHeapXor) {
				t.Errorf("Mismatch between binary and heapxor for k=%d layout=%s", k, layout)
			}
		}
	}
}

func TestCompareNoMutation(t *testing.T) {
	rb_sparse := NewBitmap()
	rb_sparse.Add(1)
	rb_sparse.Add(2)

	rb_dense := NewBitmap()
	for i := uint32(0); i < 5000; i++ {
		rb_dense.Add(i * 2)
	}

	rb_sparse_clone := rb_sparse.Clone()
	rb_dense_clone := rb_dense.Clone()

	res := HeapXor(rb_sparse, rb_dense)
	expected := originalBinaryReduction(rb_sparse_clone, rb_dense_clone)
	if !res.Equals(expected) {
		t.Errorf("heapxor result incorrect")
	}
	if !rb_sparse.Equals(rb_sparse_clone) {
		t.Errorf("rb_sparse was mutated by heapxor")
	}
	if !rb_dense.Equals(rb_dense_clone) {
		t.Errorf("rb_dense was mutated by heapxor")
	}
}

func TestCompareRun16Safety(t *testing.T) {
	rc := &runContainer16{
		iv: []interval16{
			{start: 65535, length: 1}, // covers [65535, 65536] (malformed interval)
		},
	}
	bc := newBitmapContainer()
	// Must execute safely and prevent out-of-bounds index panics
	_ = bc.ixorRun16(rc)
}
