package roaring

import (
	"container/heap"
)

// Or function that requires repairAfterLazy
func lazyOR(x1, x2 *Bitmap) *Bitmap {
	answer := NewBitmap()
	pos1 := 0
	pos2 := 0
	length1 := x1.highlowcontainer.size()
	length2 := x2.highlowcontainer.size()
main:
	for (pos1 < length1) && (pos2 < length2) {
		s1 := x1.highlowcontainer.getKeyAtIndex(pos1)
		s2 := x2.highlowcontainer.getKeyAtIndex(pos2)

		for {
			if s1 < s2 {
				answer.highlowcontainer.appendCopy(x1.highlowcontainer, pos1)
				pos1++
				if pos1 == length1 {
					break main
				}
				s1 = x1.highlowcontainer.getKeyAtIndex(pos1)
			} else if s1 > s2 {
				answer.highlowcontainer.appendCopy(x2.highlowcontainer, pos2)
				pos2++
				if pos2 == length2 {
					break main
				}
				s2 = x2.highlowcontainer.getKeyAtIndex(pos2)
			} else {
				c1 := x1.highlowcontainer.getContainerAtIndex(pos1)
				answer.highlowcontainer.appendContainer(s1, c1.lazyOR(x2.highlowcontainer.getContainerAtIndex(pos2)), false)
				pos1++
				pos2++
				if (pos1 == length1) || (pos2 == length2) {
					break main
				}
				s1 = x1.highlowcontainer.getKeyAtIndex(pos1)
				s2 = x2.highlowcontainer.getKeyAtIndex(pos2)
			}
		}
	}
	if pos1 == length1 {
		answer.highlowcontainer.appendCopyMany(x2.highlowcontainer, pos2, length2)
	} else if pos2 == length2 {
		answer.highlowcontainer.appendCopyMany(x1.highlowcontainer, pos1, length1)
	}
	return answer
}

// In-place Or function that requires repairAfterLazy
func (x1 *Bitmap) lazyOR(x2 *Bitmap) *Bitmap {
	pos1 := 0
	pos2 := 0
	length1 := x1.highlowcontainer.size()
	length2 := x2.highlowcontainer.size()
main:
	for (pos1 < length1) && (pos2 < length2) {
		s1 := x1.highlowcontainer.getKeyAtIndex(pos1)
		s2 := x2.highlowcontainer.getKeyAtIndex(pos2)

		for {
			if s1 < s2 {
				pos1++
				if pos1 == length1 {
					break main
				}
				s1 = x1.highlowcontainer.getKeyAtIndex(pos1)
			} else if s1 > s2 {
				x1.highlowcontainer.insertNewKeyValueAt(pos1, s2, x2.highlowcontainer.getContainerAtIndex(pos2).clone())
				pos2++
				pos1++
				length1++
				if pos2 == length2 {
					break main
				}
				s2 = x2.highlowcontainer.getKeyAtIndex(pos2)
			} else {
				c1 := x1.highlowcontainer.getWritableContainerAtIndex(pos1)
				// runContainer16.lazyIOR falls back to a slow ior path
				// (O(N log R) per merged element); promote to bitmapContainer
				// first, whose lazy union is O(1024) regardless of cardinality.
				// See https://github.com/RoaringBitmap/roaring/issues/81.
				if rc, ok := c1.(*runContainer16); ok && !rc.isFull() {
					c1 = rc.toBitmapContainer()
				}
				x1.highlowcontainer.containers[pos1] = c1.lazyIOR(x2.highlowcontainer.getContainerAtIndex(pos2))
				x1.highlowcontainer.needCopyOnWrite[pos1] = false
				pos1++
				pos2++
				if (pos1 == length1) || (pos2 == length2) {
					break main
				}
				s1 = x1.highlowcontainer.getKeyAtIndex(pos1)
				s2 = x2.highlowcontainer.getKeyAtIndex(pos2)
			}
		}
	}
	if pos1 == length1 {
		x1.highlowcontainer.appendCopyMany(x2.highlowcontainer, pos2, length2)
	}
	return x1
}

// to be called after lazy aggregates
func (x1 *Bitmap) repairAfterLazy() {
	for pos := 0; pos < x1.highlowcontainer.size(); pos++ {
		c := x1.highlowcontainer.getContainerAtIndex(pos)
		switch c.(type) {
		case *bitmapContainer:
			if c.(*bitmapContainer).cardinality == invalidCardinality {
				c = x1.highlowcontainer.getWritableContainerAtIndex(pos)
				c.(*bitmapContainer).computeCardinality()
				if c.(*bitmapContainer).getCardinality() <= arrayDefaultMaxSize {
					x1.highlowcontainer.setContainerAtIndex(pos, c.(*bitmapContainer).toArrayContainer())
				} else if c.(*bitmapContainer).isFull() {
					x1.highlowcontainer.setContainerAtIndex(pos, newRunContainer16Range(0, MaxUint16))
				}
			}
		}
	}
}

// FastAnd computes the intersection between many bitmaps quickly
// Compared to the And function, it can take many bitmaps as input, thus saving the trouble
// of manually calling "And" many times.
//
// Performance hints: if you have very large and tiny bitmaps,
// it may be beneficial performance-wise to put a tiny bitmap
// in first position.
func FastAnd(bitmaps ...*Bitmap) *Bitmap {
	if len(bitmaps) == 0 {
		return NewBitmap()
	} else if len(bitmaps) == 1 {
		return bitmaps[0].Clone()
	}
	answer := And(bitmaps[0], bitmaps[1])
	for _, bm := range bitmaps[2:] {
		answer.And(bm)
	}
	return answer
}

// FastOr computes the union between many bitmaps quickly, as opposed to having to call Or repeatedly.
// It might also be faster than calling Or repeatedly.
func FastOr(bitmaps ...*Bitmap) *Bitmap {
	if len(bitmaps) == 0 {
		return NewBitmap()
	} else if len(bitmaps) == 1 {
		return bitmaps[0].Clone()
	}
	answer := lazyOR(bitmaps[0], bitmaps[1])
	for _, bm := range bitmaps[2:] {
		answer = answer.lazyOR(bm)
	}
	// here is where repairAfterLazy is called.
	answer.repairAfterLazy()
	return answer
}

// HeapOr computes the union between many bitmaps quickly using a heap.
// It might be faster than calling Or repeatedly.
func HeapOr(bitmaps ...*Bitmap) *Bitmap {
	if len(bitmaps) == 0 {
		return NewBitmap()
	}
	// TODO:  for better speed, we could do the operation lazily, see Java implementation
	pq := make(priorityQueue, len(bitmaps))
	for i, bm := range bitmaps {
		pq[i] = &item{bm, i}
	}
	heap.Init(&pq)

	for pq.Len() > 1 {
		x1 := heap.Pop(&pq).(*item)
		x2 := heap.Pop(&pq).(*item)
		heap.Push(&pq, &item{Or(x1.value, x2.value), 0})
	}
	return heap.Pop(&pq).(*item).value
}

type fastLoserPlayerXor struct {
	key uint32
	pos int
	ra  *roaringArray
}

func fastXorArrayContainers(matching []container) container {
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

func fastXorManyContainers(matching []container) container {
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
		return fastXorArrayContainers(matching)
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

// HeapXor computes the symmetric difference between many bitmaps quickly (as opposed to calling Xor repeatedly).
// Internally, this function uses a loser tree to perform key-by-key multi-way merge.
func HeapXor(bitmaps ...*Bitmap) *Bitmap {
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

	players := make([]fastLoserPlayerXor, 0, nonEmptyCount)

	for _, bm := range bitmaps {
		if bm != nil && bm.highlowcontainer.size() > 0 {
			ra := &bm.highlowcontainer
			players = append(players, fastLoserPlayerXor{
				key: uint32(ra.keys[0]),
				pos: 0,
				ra:  ra,
			})
		}
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
	matchingIters := make([]*fastLoserPlayerXor, 0, k)

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
			p := &players[w]
			matching = append(matching, p.ra.containers[p.pos])
			matchingIters = append(matchingIters, p)

			p.pos++
			if p.pos < len(p.ra.keys) {
				p.key = uint32(p.ra.keys[p.pos])
			} else {
				p.key = 0xFFFFFFFF
			}

			curr := w
			parent := (curr + k) / 2
			for parent > 0 {
				loser := ls[parent]
				if players[curr].key > players[loser].key {
					ls[parent] = curr
					curr = loser
				}
				parent /= 2
			}
			ls[0] = curr
		}

		if len(matching) == 1 {
			p := matchingIters[0]
			answer.highlowcontainer.appendCopy(*p.ra, p.pos-1)
		} else if len(matching) > 1 {
			c := fastXorManyContainers(matching)
			if !c.isEmpty() {
				answer.highlowcontainer.appendContainer(uint16(minKey), c, false)
			}
		}
	}

	return answer
}

// AndAny provides a result equivalent to x1.And(FastOr(bitmaps)).
// It's optimized to minimize allocations. It also might be faster than separate calls.
func (x1 *Bitmap) AndAny(bitmaps ...*Bitmap) {
	if len(bitmaps) == 0 {
		return
	} else if len(bitmaps) == 1 {
		x1.And(bitmaps[0])
		return
	}

	type withPos struct {
		bitmap *roaringArray
		pos    int
		key    uint16
	}
	filters := make([]withPos, 0, len(bitmaps))

	for _, b := range bitmaps {
		if b.highlowcontainer.size() > 0 {
			filters = append(filters, withPos{
				bitmap: &b.highlowcontainer,
				pos:    0,
				key:    b.highlowcontainer.getKeyAtIndex(0),
			})
		}
	}

	basePos := 0
	intersections := 0
	keyContainers := make([]container, 0, len(filters))
	var (
		tmpArray   *arrayContainer
		tmpBitmap  *bitmapContainer
		minNextKey uint16
	)

	for basePos < x1.highlowcontainer.size() && len(filters) > 0 {
		baseKey := x1.highlowcontainer.getKeyAtIndex(basePos)

		// accumulate containers for current key, find next minimal key in filters
		// and exclude filters that do not have related values anymore
		i := 0
		maxPossibleOr := 0
		minNextKey = MaxUint16
		for _, f := range filters {
			if f.key < baseKey {
				f.pos = f.bitmap.advanceUntil(baseKey, f.pos)
				if f.pos == f.bitmap.size() {
					continue
				}
				f.key = f.bitmap.getKeyAtIndex(f.pos)
			}

			if f.key == baseKey {
				cont := f.bitmap.getContainerAtIndex(f.pos)
				keyContainers = append(keyContainers, cont)
				maxPossibleOr += cont.getCardinality()

				f.pos++
				if f.pos == f.bitmap.size() {
					continue
				}
				f.key = f.bitmap.getKeyAtIndex(f.pos)
			}

			minNextKey = minOfUint16(minNextKey, f.key)
			filters[i] = f
			i++
		}
		filters = filters[:i]

		if len(keyContainers) == 0 {
			basePos = x1.highlowcontainer.advanceUntil(minNextKey, basePos)
			continue
		}

		var ored container

		if len(keyContainers) == 1 {
			ored = keyContainers[0]
		} else {
			//TODO: special case for run containers?
			if maxPossibleOr > arrayDefaultMaxSize {
				if tmpBitmap == nil {
					tmpBitmap = newBitmapContainer()
				}
				tmpBitmap.resetTo(keyContainers[0])
				ored = tmpBitmap
			} else {
				if tmpArray == nil {
					tmpArray = newArrayContainerCapacity(maxPossibleOr)
				}
				tmpArray.realloc(maxPossibleOr)
				tmpArray.resetTo(keyContainers[0])
				ored = tmpArray
			}
			for _, c := range keyContainers[1:] {
				ored = ored.ior(c)
			}
		}

		result := x1.highlowcontainer.getWritableContainerAtIndex(basePos).iand(ored)
		if !result.isEmpty() {
			x1.highlowcontainer.replaceKeyAndContainerAtIndex(intersections, baseKey, result, false)
			intersections++
		}

		keyContainers = keyContainers[:0]
		basePos = x1.highlowcontainer.advanceUntil(minNextKey, basePos)
	}

	x1.highlowcontainer.resize(intersections)
}
