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
	// Use roaring's local or aliased heap package
	origHeapInit(&pq)

	for pq.Len() > 1 {
		x1 := origHeapPop(&pq)
		x2 := origHeapPop(&pq)
		origHeapPush(&pq, &origItem{Xor(x1.value, x2.value), 0})
	}
	return origHeapPop(&pq).value
}

// Since container/heap might be imported, let's define our own min-heap functions for origPriorityQueue to be completely self-contained and fast.
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
		i := (j - 1) / 2 // parent
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
		if j1 >= n || j1 < 0 { // j1 < 0 after int overflow
			break
		}
		j := j1 // left child
		if j2 := j1 + 1; j2 < n && (*pq).Less(j2, j1) {
			j = j2 // = 2*i + 2  // right child
		}
		if !(*pq).Less(j, i) {
			break
		}
		(*pq).Swap(i, j)
		i = j
	}
	return i > i0
}

type compXorKeyIter struct {
	ra  *roaringArray
	pos int
}

type compXorHeapItem struct {
	key uint16
	it  *compXorKeyIter
}

func compHeapifyXor(heap []compXorHeapItem, i int) {
	n := len(heap)
	temp := heap[i]
	for {
		child := 2*i + 1
		if child >= n {
			break
		}
		if child+1 < n && heap[child+1].key < heap[child].key {
			child++
		}
		if temp.key <= heap[child].key {
			break
		}
		heap[i] = heap[child]
		i = child
	}
	heap[i] = temp
}

func xorArrayContainers(matching []container) container {
	var buf1 [4096]uint16
	var buf2 [4096]uint16

	ac0 := matching[0].(*arrayContainer)
	s1 := buf1[:len(ac0.content)]
	copy(s1, ac0.content)

	curBuf := 1
	isBitmap := false
	var bc *bitmapContainer

	for i := 1; i < len(matching); i++ {
		acNext := matching[i].(*arrayContainer)
		if isBitmap {
			bc.ixorArray(acNext)
			continue
		}

		if len(s1)+len(acNext.content) > 4096 {
			bc = newBitmapContainer()
			for _, v := range s1 {
				bc.bitmap[uint(v)>>6] ^= (uint64(1) << (v % 64))
			}
			bc.cardinality = len(s1)
			bc.ixorArray(acNext)
			isBitmap = true
			continue
		}

		if curBuf == 1 {
			s2 := buf2[:]
			n := exclusiveUnion2by2(s1, acNext.content, s2)
			s1 = buf2[:n]
			curBuf = 2
		} else {
			s2 := buf1[:]
			n := exclusiveUnion2by2(s1, acNext.content, s2)
			s1 = buf1[:n]
			curBuf = 1
		}
	}

	if isBitmap {
		if bc.cardinality <= arrayDefaultMaxSize {
			return bc.toArrayContainer()
		}
		return bc
	}

	ans := newArrayContainerSize(len(s1))
	copy(ans.content, s1)
	return ans
}

func xorManyContainers(matching []container) container {
	if len(matching) == 0 {
		return nil
	}
	if len(matching) == 1 {
		return matching[0].clone()
	}

	allArray := true
	for _, c := range matching {
		if _, ok := c.(*arrayContainer); !ok {
			allArray = false
			break
		}
	}

	if allArray {
		return xorArrayContainers(matching)
	}

	c := matching[0].clone()
	for _, nextC := range matching[1:] {
		if _, ok := nextC.(*bitmapContainer); ok {
			if _, ok = c.(*bitmapContainer); !ok {
				switch ac := c.(type) {
				case *arrayContainer:
					c = ac.toBitmapContainer()
				case *runContainer16:
					c = ac.toBitmapContainer()
				}
			}
		}
		c = c.ixor(nextC)
	}
	return c
}

func typedHeapMerge(bitmaps ...*Bitmap) *Bitmap {
	if len(bitmaps) == 0 {
		return NewBitmap()
	} else if len(bitmaps) == 1 {
		if bitmaps[0] == nil {
			return nil
		}
		return bitmaps[0].Clone()
	}

	nonEmptyCount := 0
	for _, bm := range bitmaps {
		if bm != nil && bm.highlowcontainer.size() > 0 {
			nonEmptyCount++
		}
	}

	if nonEmptyCount == 0 {
		return NewBitmap()
	} else if nonEmptyCount == 1 {
		for _, bm := range bitmaps {
			if bm != nil && bm.highlowcontainer.size() > 0 {
				return bm.Clone()
			}
		}
	}

	iters := make([]compXorKeyIter, 0, nonEmptyCount)
	heap := make([]compXorHeapItem, 0, nonEmptyCount)

	for _, bm := range bitmaps {
		if bm != nil && bm.highlowcontainer.size() > 0 {
			ra := &bm.highlowcontainer
			iters = append(iters, compXorKeyIter{
				ra:  ra,
				pos: 0,
			})
		}
	}

	for i := range iters {
		heap = append(heap, compXorHeapItem{
			key: iters[i].ra.keys[0],
			it:  &iters[i],
		})
	}

	for i := len(heap)/2 - 1; i >= 0; i-- {
		compHeapifyXor(heap, i)
	}

	answer := NewBitmap()
	matching := make([]container, 0, nonEmptyCount)
	matchingIters := make([]*compXorKeyIter, 0, nonEmptyCount)

	for len(heap) > 0 {
		minKey := heap[0].key
		matching = matching[:0]
		matchingIters = matchingIters[:0]

		for len(heap) > 0 && heap[0].key == minKey {
			item := &heap[0]
			it := item.it
			matching = append(matching, it.ra.containers[it.pos])
			matchingIters = append(matchingIters, it)

			it.pos++
			if it.pos < len(it.ra.keys) {
				item.key = it.ra.keys[it.pos]
				compHeapifyXor(heap, 0)
			} else {
				heap[0] = heap[len(heap)-1]
				heap = heap[:len(heap)-1]
				if len(heap) > 0 {
					compHeapifyXor(heap, 0)
				}
			}
		}

		if len(matching) == 1 {
			it := matchingIters[0]
			answer.highlowcontainer.appendCopy(*it.ra, it.pos-1)
		} else if len(matching) > 1 {
			c := xorManyContainers(matching)
			if !c.isEmpty() {
				answer.highlowcontainer.appendContainer(minKey, c, false)
			}
		}
	}

	return answer
}

type compLoserPlayer struct {
	key uint32
	it  *compXorKeyIter
}

func loserTreeMerge(bitmaps ...*Bitmap) *Bitmap {
	if len(bitmaps) == 0 {
		return NewBitmap()
	} else if len(bitmaps) == 1 {
		if bitmaps[0] == nil {
			return nil
		}
		return bitmaps[0].Clone()
	}

	nonEmptyCount := 0
	for _, bm := range bitmaps {
		if bm != nil && bm.highlowcontainer.size() > 0 {
			nonEmptyCount++
		}
	}

	if nonEmptyCount == 0 {
		return NewBitmap()
	} else if nonEmptyCount == 1 {
		for _, bm := range bitmaps {
			if bm != nil && bm.highlowcontainer.size() > 0 {
				return bm.Clone()
			}
		}
	}

	iters := make([]compXorKeyIter, 0, nonEmptyCount)
	players := make([]compLoserPlayer, 0, nonEmptyCount)

	for _, bm := range bitmaps {
		if bm != nil && bm.highlowcontainer.size() > 0 {
			ra := &bm.highlowcontainer
			iters = append(iters, compXorKeyIter{
				ra:  ra,
				pos: 0,
			})
		}
	}

	for i := range iters {
		players = append(players, compLoserPlayer{
			key: uint32(iters[i].ra.keys[0]),
			it:  &iters[i],
		})
	}

	k := len(players)
	ls := make([]int, k)
	for i := range ls {
		ls[i] = -1
	}

	for i := 0; i < k; i++ {
		p := i
		parent := (p + k) / 2
		for parent > 0 {
			loser := ls[parent]
			if loser == -1 {
				ls[parent] = p
				break
			}
			if players[p].key > players[loser].key {
				ls[parent] = p
				p = loser
			}
			parent /= 2
		}
		if parent == 0 {
			ls[0] = p
		}
	}

	answer := NewBitmap()
	matching := make([]container, 0, k)
	matchingIters := make([]*compXorKeyIter, 0, k)

	for {
		winner := ls[0]
		minKey := players[winner].key
		if minKey == 0xFFFFFFFF {
			break
		}

		matching = matching[:0]
		matchingIters = matchingIters[:0]

		for players[ls[0]].key == minKey {
			w := ls[0]
			it := players[w].it
			matching = append(matching, it.ra.containers[it.pos])
			matchingIters = append(matchingIters, it)

			it.pos++
			if it.pos < len(it.ra.keys) {
				players[w].key = uint32(it.ra.keys[it.pos])
			} else {
				players[w].key = 0xFFFFFFFF
			}

			p := w
			parent := (p + k) / 2
			for parent > 0 {
				loser := ls[parent]
				if players[p].key > players[loser].key {
					ls[parent] = p
					p = loser
				}
				parent /= 2
			}
			ls[0] = p
		}

		if len(matching) == 1 {
			it := matchingIters[0]
			answer.highlowcontainer.appendCopy(*it.ra, it.pos-1)
		} else if len(matching) > 1 {
			c := xorManyContainers(matching)
			if !c.isEmpty() {
				answer.highlowcontainer.appendContainer(uint16(minKey), c, false)
			}
		}
	}

	return answer
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
		{"typedheap", typedHeapMerge},
		{"losertree", loserTreeMerge},
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
			resTypedHeap := typedHeapMerge(bms...)
			resLoserTree := loserTreeMerge(bms...)
			resHeapXor := HeapXor(bms...)

			if !resBinary.Equals(resTypedHeap) {
				t.Errorf("Mismatch between binary and typedheap for k=%d layout=%s", k, layout)
			}
			if !resBinary.Equals(resLoserTree) {
				t.Errorf("Mismatch between binary and losertree for k=%d layout=%s", k, layout)
			}
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

	res1 := typedHeapMerge(rb_sparse, rb_dense)
	expected1 := originalBinaryReduction(rb_sparse_clone, rb_dense_clone)
	if !res1.Equals(expected1) {
		t.Errorf("typedheap result incorrect")
	}
	if !rb_sparse.Equals(rb_sparse_clone) {
		t.Errorf("rb_sparse was mutated by typedheap")
	}
	if !rb_dense.Equals(rb_dense_clone) {
		t.Errorf("rb_dense was mutated by typedheap")
	}

	res2 := loserTreeMerge(rb_sparse, rb_dense)
	if !res2.Equals(expected1) {
		t.Errorf("losertree result incorrect")
	}
	if !rb_sparse.Equals(rb_sparse_clone) {
		t.Errorf("rb_sparse was mutated by losertree")
	}
	if !rb_dense.Equals(rb_dense_clone) {
		t.Errorf("rb_dense was mutated by losertree")
	}
}
