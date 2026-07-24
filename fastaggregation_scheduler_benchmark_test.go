package roaring

import (
	"container/heap"
	"fmt"
	"runtime"
	"testing"
)

// This file is proof-only benchmark material. It keeps the binary reduction,
// typed heap, and tournament-tree contenders independent of HeapXor so the
// same commit compares them on both proof arms. It also exercises the
// production HeapXor path over the identical matrix.

type schedulerFixture struct {
	k       int
	layout  string
	bitmaps []*Bitmap
}

type schedulerContender struct {
	name string
	fn   func(...*Bitmap) *Bitmap
}

type schedulerKeyIterator struct {
	ra  *roaringArray
	pos int
}

type schedulerHeapItem struct {
	key uint16
	it  *schedulerKeyIterator
}

type schedulerTreePlayer struct {
	key uint32
	ra  *roaringArray
	pos int
}

var schedulerBenchmarkSink uint64

var schedulerContenders = []schedulerContender{
	{name: "binary", fn: schedulerOriginalBinaryReduction},
	{name: "typedheap", fn: schedulerTypedHeapMerge},
	{name: "tournamenttree", fn: schedulerTournamentTreeMerge},
	{name: "heapxor", fn: HeapXor},
}

func schedulerOriginalBinaryReduction(bitmaps ...*Bitmap) *Bitmap {
	if len(bitmaps) == 0 {
		return NewBitmap()
	}

	// This is the pre-change HeapXor reduction expressed with the package's
	// original priorityQueue and container/heap implementation.
	pq := make(priorityQueue, len(bitmaps))
	for i, bm := range bitmaps {
		pq[i] = &item{bm, i}
	}
	heap.Init(&pq)

	for pq.Len() > 1 {
		x1 := heap.Pop(&pq).(*item)
		x2 := heap.Pop(&pq).(*item)
		heap.Push(&pq, &item{Xor(x1.value, x2.value), 0})
	}
	return heap.Pop(&pq).(*item).value
}

func schedulerActiveIterators(bitmaps ...*Bitmap) []schedulerKeyIterator {
	iters := make([]schedulerKeyIterator, 0, len(bitmaps))
	for _, bm := range bitmaps {
		if bm != nil && bm.highlowcontainer.size() > 0 {
			iters = append(iters, schedulerKeyIterator{ra: &bm.highlowcontainer})
		}
	}
	return iters
}

func schedulerTrivialResult(bitmaps ...*Bitmap) (*Bitmap, bool) {
	var only *Bitmap
	for _, bm := range bitmaps {
		if bm != nil && bm.highlowcontainer.size() > 0 {
			if only != nil {
				return nil, false
			}
			only = bm
		}
	}
	if only == nil {
		return NewBitmap(), true
	}
	return only.Clone(), true
}

func schedulerHeapify(items []schedulerHeapItem, i int) {
	temp := items[i]
	for {
		child := 2*i + 1
		if child >= len(items) {
			break
		}
		if child+1 < len(items) && items[child+1].key < items[child].key {
			child++
		}
		if temp.key <= items[child].key {
			break
		}
		items[i] = items[child]
		i = child
	}
	items[i] = temp
}

func schedulerTypedHeapMerge(bitmaps ...*Bitmap) *Bitmap {
	if len(bitmaps) == 0 {
		return NewBitmap()
	}
	if result, done := schedulerTrivialResult(bitmaps...); done {
		return result
	}

	iters := schedulerActiveIterators(bitmaps...)
	items := make([]schedulerHeapItem, len(iters))
	for i := range iters {
		items[i] = schedulerHeapItem{key: iters[i].ra.keys[0], it: &iters[i]}
	}
	for i := len(items)/2 - 1; i >= 0; i-- {
		schedulerHeapify(items, i)
	}

	answer := NewBitmap()
	matching := make([]container, 0, len(items))
	matchingIters := make([]*schedulerKeyIterator, 0, len(items))
	for len(items) > 0 {
		minKey := items[0].key
		matching = matching[:0]
		matchingIters = matchingIters[:0]

		for len(items) > 0 && items[0].key == minKey {
			current := &items[0]
			it := current.it
			matching = append(matching, it.ra.containers[it.pos])
			matchingIters = append(matchingIters, &schedulerKeyIterator{ra: it.ra, pos: it.pos})

			it.pos++
			if it.pos < len(it.ra.keys) {
				current.key = it.ra.keys[it.pos]
				schedulerHeapify(items, 0)
			} else {
				items[0] = items[len(items)-1]
				items = items[:len(items)-1]
				if len(items) > 0 {
					schedulerHeapify(items, 0)
				}
			}
		}

		schedulerAppendXorResult(answer, minKey, matching, matchingIters)
	}
	return answer
}

func schedulerTournamentLess(players []schedulerTreePlayer, left, right int) bool {
	if left < 0 {
		return false
	}
	if right < 0 {
		return true
	}
	if players[left].key != players[right].key {
		return players[left].key < players[right].key
	}
	return left < right
}

func schedulerTournamentWinner(players []schedulerTreePlayer, left, right int) int {
	if schedulerTournamentLess(players, left, right) {
		return left
	}
	return right
}

func schedulerTournamentBuild(players []schedulerTreePlayer) ([]int, int) {
	leafCount := 1
	for leafCount < len(players) {
		leafCount *= 2
	}
	tree := make([]int, 2*leafCount)
	for i := range tree {
		tree[i] = -1
	}
	for i := range players {
		tree[leafCount+i] = i
	}
	for i := leafCount - 1; i > 0; i-- {
		tree[i] = schedulerTournamentWinner(players, tree[2*i], tree[2*i+1])
	}
	return tree, leafCount
}

func schedulerTournamentUpdate(tree []int, leafCount int, players []schedulerTreePlayer, player int) {
	for node := (leafCount + player) / 2; node > 0; node /= 2 {
		tree[node] = schedulerTournamentWinner(players, tree[2*node], tree[2*node+1])
	}
}

func schedulerTournamentTreeMerge(bitmaps ...*Bitmap) *Bitmap {
	if len(bitmaps) == 0 {
		return NewBitmap()
	}
	if result, done := schedulerTrivialResult(bitmaps...); done {
		return result
	}

	iters := schedulerActiveIterators(bitmaps...)
	players := make([]schedulerTreePlayer, len(iters))
	for i := range iters {
		players[i] = schedulerTreePlayer{key: uint32(iters[i].ra.keys[0]), ra: iters[i].ra}
	}
	tree, leafCount := schedulerTournamentBuild(players)

	answer := NewBitmap()
	matching := make([]container, 0, len(players))
	matchingIters := make([]*schedulerKeyIterator, 0, len(players))
	for {
		winner := tree[1]
		minKey := players[winner].key
		if minKey == ^uint32(0) {
			break
		}

		matching = matching[:0]
		matchingIters = matchingIters[:0]
		for players[tree[1]].key == minKey {
			current := tree[1]
			player := &players[current]
			matching = append(matching, player.ra.containers[player.pos])
			matchingIters = append(matchingIters, &schedulerKeyIterator{ra: player.ra, pos: player.pos})

			player.pos++
			if player.pos < len(player.ra.keys) {
				player.key = uint32(player.ra.keys[player.pos])
			} else {
				player.key = ^uint32(0)
			}
			schedulerTournamentUpdate(tree, leafCount, players, current)
		}

		schedulerAppendXorResult(answer, uint16(minKey), matching, matchingIters)
	}
	return answer
}

func schedulerAppendXorResult(answer *Bitmap, key uint16, matching []container, matchingIters []*schedulerKeyIterator) {
	if len(matching) == 1 {
		it := matchingIters[0]
		answer.highlowcontainer.appendCopy(*it.ra, it.pos)
		return
	}
	if len(matching) > 1 {
		result := schedulerXorManyContainers(matching)
		if !result.isEmpty() {
			answer.highlowcontainer.appendContainer(key, result, false)
		}
	}
}

func schedulerXorManyContainers(matching []container) container {
	allArray := true
	for _, current := range matching {
		if _, ok := current.(*arrayContainer); !ok {
			allArray = false
			break
		}
	}
	if allArray {
		return schedulerXorArrayContainers(matching)
	}

	result := matching[0].clone()
	for _, next := range matching[1:] {
		// ixor on a run or array receiver may choose the bitmap operand as
		// its writable destination. Convert the private left clone first so
		// that neither scheduler is allowed to mutate a fixture container.
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

func schedulerXorArrayContainers(matching []container) container {
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

func schedulerComparisonFixtures() []schedulerFixture {
	ks := []int{2, 4, 8, 16, 32, 64, 128}
	layouts := []string{"aligned", "disjoint", "interleaved", "skewed", "mixed"}
	fixtures := make([]schedulerFixture, 0, len(ks)*len(layouts))
	for _, k := range ks {
		for _, layout := range layouts {
			fixtures = append(fixtures, schedulerFixture{
				k:       k,
				layout:  layout,
				bitmaps: schedulerFixtureBitmaps(k, layout),
			})
		}
	}
	return fixtures
}

func schedulerFixtureBitmaps(k int, layout string) []*Bitmap {
	switch layout {
	case "aligned":
		return schedulerAlignedBitmaps(k)
	case "disjoint":
		return schedulerDisjointBitmaps(k)
	case "interleaved":
		return schedulerInterleavedBitmaps(k)
	case "skewed":
		return schedulerSkewedBitmaps(k)
	case "mixed":
		return schedulerMixedBitmaps(k)
	default:
		panic("unknown scheduler fixture layout: " + layout)
	}
}

func schedulerAlignedBitmaps(k int) []*Bitmap {
	bitmaps := make([]*Bitmap, k)
	for i := range bitmaps {
		bitmap := NewBitmap()
		for key := uint32(0); key < 10; key++ {
			base := key * 65536
			for value := uint32(0); value < 160; value++ {
				bitmap.Add(base + value*17 + uint32(i*7))
			}
		}
		bitmaps[i] = bitmap
	}
	return bitmaps
}

func schedulerDisjointBitmaps(k int) []*Bitmap {
	bitmaps := make([]*Bitmap, k)
	for i := range bitmaps {
		bitmap := NewBitmap()
		for offset := uint32(0); offset < 10; offset++ {
			base := (uint32(i)*10 + offset) * 65536
			for value := uint32(0); value < 120; value++ {
				bitmap.Add(base + value*19 + uint32(i))
			}
		}
		bitmaps[i] = bitmap
	}
	return bitmaps
}

func schedulerInterleavedBitmaps(k int) []*Bitmap {
	bitmaps := make([]*Bitmap, k)
	for i := range bitmaps {
		bitmap := NewBitmap()
		for step := uint32(0); step < 10; step++ {
			base := (step*uint32(k) + uint32(i)) * 65536
			for value := uint32(0); value < 120; value++ {
				bitmap.Add(base + value*19 + uint32(i))
			}
		}
		bitmaps[i] = bitmap
	}
	return bitmaps
}

func schedulerSkewedBitmaps(k int) []*Bitmap {
	bitmaps := make([]*Bitmap, k)
	for i := range bitmaps {
		bitmap := NewBitmap()
		keyCount := 3
		valueCount := 60
		if i == 0 {
			keyCount = 48
			valueCount = 700
		}
		for offset := 0; offset < keyCount; offset++ {
			key := uint32(offset)
			if i != 0 {
				key = uint32((i*3 + offset) % 48)
			}
			base := key * 65536
			for value := 0; value < valueCount; value++ {
				bitmap.Add(base + uint32(value*23+i*5))
			}
		}
		bitmaps[i] = bitmap
	}
	return bitmaps
}

func schedulerMixedBitmaps(k int) []*Bitmap {
	bitmaps := make([]*Bitmap, k)
	for i := range bitmaps {
		bitmap := NewBitmap()
		for value := uint32(0); value < 120; value++ {
			bitmap.Add(value*29 + uint32(i))
		}
		for value := uint32(0); value < 5000; value++ {
			bitmap.Add(65536 + (value*13+uint32(i*37))%65536)
		}
		for value := uint32(0); value < 1000; value++ {
			bitmap.Add(2*65536 + value + uint32(i*3))
		}
		bitmap.RunOptimize()
		bitmaps[i] = bitmap
	}
	return bitmaps
}

func BenchmarkSchedulerComparison(b *testing.B) {
	fixtures := schedulerComparisonFixtures()
	for _, fixture := range fixtures {
		fixture := fixture
		for _, contender := range schedulerContenders {
			contender := contender
			name := fmt.Sprintf("k=%d/layout=%s/contender=%s", fixture.k, fixture.layout, contender.name)
			b.Run(name, func(b *testing.B) {
				var cardinality uint64
				for b.Loop() {
					result := contender.fn(fixture.bitmaps...)
					cardinality ^= result.GetCardinality()
				}
				schedulerBenchmarkSink = cardinality
				runtime.KeepAlive(fixture.bitmaps)
			})
		}
	}
}

// BenchmarkSchedulerComparisonAggregate executes every fan-in and layout for a
// single contender per benchmark operation. The HeapXor row measures the
// production implementation over the whole matrix; BenchmarkSchedulerComparison
// retains every per-regime row for scheduler comparison and CPU profiling.
func BenchmarkSchedulerComparisonAggregate(b *testing.B) {
	fixtures := schedulerComparisonFixtures()
	for _, contender := range schedulerContenders {
		contender := contender
		b.Run("contender="+contender.name, func(b *testing.B) {
			var cardinality uint64
			for b.Loop() {
				for _, fixture := range fixtures {
					result := contender.fn(fixture.bitmaps...)
					cardinality ^= result.GetCardinality()
				}
			}
			schedulerBenchmarkSink = cardinality
			runtime.KeepAlive(fixtures)
		})
	}
}

func TestHeapXorSchedulerCorrectness(t *testing.T) {
	for _, fixture := range schedulerComparisonFixtures() {
		fixture := fixture
		referenceInputs := schedulerCloneBitmaps(fixture.bitmaps)
		reference := schedulerOriginalBinaryReduction(fixture.bitmaps...)
		if !schedulerBitmapsEqual(fixture.bitmaps, referenceInputs) {
			t.Fatalf("binary reduction mutated inputs for k=%d layout=%s", fixture.k, fixture.layout)
		}

		for _, contender := range schedulerContenders[1:] {
			before := schedulerCloneBitmaps(fixture.bitmaps)
			actual := contender.fn(fixture.bitmaps...)
			if !actual.Equals(reference) {
				t.Errorf("%s differs from binary reduction for k=%d layout=%s", contender.name, fixture.k, fixture.layout)
			}
			if !schedulerBitmapsEqual(fixture.bitmaps, before) {
				t.Errorf("%s mutated inputs for k=%d layout=%s", contender.name, fixture.k, fixture.layout)
			}
		}
	}
}

func TestHeapXorSchedulerMixedContainers(t *testing.T) {
	bitmap := schedulerMixedBitmaps(2)[0]
	hasArray := false
	hasBitmap := false
	hasRun := false
	for _, current := range bitmap.highlowcontainer.containers {
		switch current.(type) {
		case *arrayContainer:
			hasArray = true
		case *bitmapContainer:
			hasBitmap = true
		case *runContainer16:
			hasRun = true
		}
	}
	if !hasArray || !hasBitmap || !hasRun {
		t.Fatalf("mixed fixture types: array=%t bitmap=%t run=%t", hasArray, hasBitmap, hasRun)
	}
}

func TestHeapXorSchedulerBufferBacked(t *testing.T) {
	inputs := schedulerMixedBitmaps(8)
	buffers := make([][]byte, len(inputs))
	backed := make([]*Bitmap, len(inputs))
	for i, input := range inputs {
		data, err := input.MarshalBinary()
		if err != nil {
			t.Fatalf("marshal input %d: %v", i, err)
		}
		bitmap := NewBitmap()
		if _, err := bitmap.FromBuffer(data); err != nil {
			t.Fatalf("load input %d from buffer: %v", i, err)
		}
		if err := bitmap.Validate(); err != nil {
			t.Fatalf("validate input %d from buffer: %v", i, err)
		}
		buffers[i] = data
		backed[i] = bitmap
	}

	reference := schedulerOriginalBinaryReduction(backed...)
	for _, contender := range schedulerContenders[1:] {
		before := schedulerCloneBitmaps(backed)
		actual := contender.fn(backed...)
		if !actual.Equals(reference) {
			t.Errorf("%s differs from binary reduction for buffer-backed inputs", contender.name)
		}
		if !schedulerBitmapsEqual(backed, before) {
			t.Errorf("%s mutated buffer-backed inputs", contender.name)
		}
	}
	runtime.KeepAlive(buffers)
}

func schedulerCloneBitmaps(bitmaps []*Bitmap) []*Bitmap {
	clones := make([]*Bitmap, len(bitmaps))
	for i, bitmap := range bitmaps {
		clones[i] = bitmap.Clone()
	}
	return clones
}

func schedulerBitmapsEqual(left, right []*Bitmap) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if !left[i].Equals(right[i]) {
			return false
		}
	}
	return true
}
