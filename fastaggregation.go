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

type heapXorKeyIterator struct {
	array *roaringArray
	pos   int
}

type heapXorItem struct {
	key      uint16
	iterator *heapXorKeyIterator
}

func heapXorHeapify(items []heapXorItem, index int) {
	item := items[index]
	for {
		child := 2*index + 1
		if child >= len(items) {
			break
		}
		if child+1 < len(items) && items[child+1].key < items[child].key {
			child++
		}
		if item.key <= items[child].key {
			break
		}
		items[index] = items[child]
		index = child
	}
	items[index] = item
}

func heapXorMergeContainers(matching []container) container {
	allArrays := true
	for _, current := range matching {
		if _, ok := current.(*arrayContainer); !ok {
			allArrays = false
			break
		}
	}
	if allArrays {
		return heapXorArrayContainers(matching)
	}

	result := matching[0].clone()
	for _, next := range matching[1:] {
		if _, nextIsBitmap := next.(*bitmapContainer); nextIsBitmap {
			if _, resultIsBitmap := result.(*bitmapContainer); !resultIsBitmap {
				switch current := result.(type) {
				case *arrayContainer:
					result = current.toBitmapContainer()
				case *runContainer16:
					result = current.toBitmapContainer()
				}
			}
		}
		result = result.ixor(next)
	}
	return result
}

func heapXorArrayContainers(matching []container) container {
	first := matching[0].(*arrayContainer)
	var bufferA [arrayDefaultMaxSize]uint16
	var bufferB [arrayDefaultMaxSize]uint16
	current := bufferA[:len(first.content)]
	copy(current, first.content)
	useA := true

	for index := 1; index < len(matching); index++ {
		next := matching[index].(*arrayContainer)
		if len(current)+len(next.content) > arrayDefaultMaxSize {
			bitmap := newBitmapContainer()
			for _, value := range current {
				bitmap.bitmap[uint(value)>>6] ^= uint64(1) << (value % 64)
			}
			bitmap.cardinality = len(current)
			result := container(bitmap).ixor(next)
			for _, remaining := range matching[index+1:] {
				result = result.ixor(remaining)
			}
			return result
		}

		var destination []uint16
		if useA {
			destination = bufferB[:]
		} else {
			destination = bufferA[:]
		}
		count := exclusiveUnion2by2(current, next.content, destination)
		current = destination[:count]
		useA = !useA
	}

	result := newArrayContainerSize(len(current))
	copy(result.content, current)
	return result
}

// HeapXor computes the symmetric difference between many bitmaps quickly (as opposed to calling Xor repeated).
// It merges the sorted high keys of all inputs and only materializes final result containers.
func HeapXor(bitmaps ...*Bitmap) *Bitmap {
	switch len(bitmaps) {
	case 0:
		return NewBitmap()
	case 1:
		return bitmaps[0]
	case 2:
		return Xor(bitmaps[0], bitmaps[1])
	}

	nonEmptyCount := 0
	for _, bitmap := range bitmaps {
		if bitmap != nil && bitmap.highlowcontainer.size() > 0 {
			nonEmptyCount++
		}
	}
	if nonEmptyCount == 0 {
		return NewBitmap()
	}
	if nonEmptyCount == 1 {
		for _, bitmap := range bitmaps {
			if bitmap != nil && bitmap.highlowcontainer.size() > 0 {
				return bitmap.Clone()
			}
		}
	}

	iterators := make([]heapXorKeyIterator, 0, nonEmptyCount)
	for _, bitmap := range bitmaps {
		if bitmap != nil && bitmap.highlowcontainer.size() > 0 {
			iterators = append(iterators, heapXorKeyIterator{array: &bitmap.highlowcontainer})
		}
	}

	items := make([]heapXorItem, 0, len(iterators))
	for index := range iterators {
		items = append(items, heapXorItem{
			key:      iterators[index].array.keys[0],
			iterator: &iterators[index],
		})
	}
	for index := len(items)/2 - 1; index >= 0; index-- {
		heapXorHeapify(items, index)
	}

	answer := NewBitmap()
	matching := make([]container, 0, len(items))
	for len(items) > 0 {
		key := items[0].key
		matching = matching[:0]
		var source *roaringArray
		sourceIndex := 0

		for len(items) > 0 && items[0].key == key {
			item := &items[0]
			iterator := item.iterator
			if len(matching) == 0 {
				source = iterator.array
				sourceIndex = iterator.pos
			}
			matching = append(matching, iterator.array.containers[iterator.pos])

			iterator.pos++
			if iterator.pos < len(iterator.array.keys) {
				item.key = iterator.array.keys[iterator.pos]
				heapXorHeapify(items, 0)
			} else {
				items[0] = items[len(items)-1]
				items = items[:len(items)-1]
				if len(items) > 0 {
					heapXorHeapify(items, 0)
				}
			}
		}

		if len(matching) == 1 {
			answer.highlowcontainer.appendCopy(*source, sourceIndex)
			continue
		}
		result := heapXorMergeContainers(matching)
		if !result.isEmpty() {
			answer.highlowcontainer.appendContainer(key, result, false)
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
