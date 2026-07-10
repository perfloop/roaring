package roaring

// to run just these tests: go test -run TestFastAggregations*

import (
	"container/heap"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestFastAggregationsSize(t *testing.T) {
	rb1 := NewBitmap()
	rb2 := NewBitmap()
	rb3 := NewBitmap()
	for i := uint32(0); i < 1000000; i += 3 {
		rb1.Add(i)
	}
	for i := uint32(0); i < 1000000; i += 7 {
		rb2.Add(i)
	}
	for i := uint32(0); i < 1000000; i += 1001 {
		rb3.Add(i)
	}
	pq := make(priorityQueue, 3)
	pq[0] = &item{rb1, 0}
	pq[1] = &item{rb2, 1}
	pq[2] = &item{rb3, 2}
	heap.Init(&pq)

	assert.Equal(t, rb3.GetSizeInBytes(), heap.Pop(&pq).(*item).value.GetSizeInBytes())
	assert.Equal(t, rb2.GetSizeInBytes(), heap.Pop(&pq).(*item).value.GetSizeInBytes())
	assert.Equal(t, rb1.GetSizeInBytes(), heap.Pop(&pq).(*item).value.GetSizeInBytes())
}

func TestFastAggregationsCont(t *testing.T) {
	rb1 := NewBitmap()
	rb2 := NewBitmap()
	rb3 := NewBitmap()
	for i := uint32(0); i < 10; i += 3 {
		rb1.Add(i)
	}
	for i := uint32(0); i < 10; i += 7 {
		rb2.Add(i)
	}
	for i := uint32(0); i < 10; i += 1001 {
		rb3.Add(i)
	}
	for i := uint32(1000000); i < 1000000+10; i += 1001 {
		rb1.Add(i)
	}
	for i := uint32(1000000); i < 1000000+10; i += 7 {
		rb2.Add(i)
	}
	for i := uint32(1000000); i < 1000000+10; i += 3 {
		rb3.Add(i)
	}
	rb1.Add(500000)
	pq := make(containerPriorityQueue, 3)
	pq[0] = &containeritem{rb1, 0, 0}
	pq[1] = &containeritem{rb2, 0, 1}
	pq[2] = &containeritem{rb3, 0, 2}
	heap.Init(&pq)
	expected := []int{6, 4, 5, 6, 5, 4, 6}
	counter := 0
	for pq.Len() > 0 {
		x1 := heap.Pop(&pq).(*containeritem)
		assert.EqualValues(t, expected[counter], x1.value.GetCardinality())

		counter++
		x1.keyindex++
		if x1.keyindex < x1.value.highlowcontainer.size() {
			heap.Push(&pq, x1)
		}
	}
}

func TestFastAggregationsAdvanced_run(t *testing.T) {
	rb1 := NewBitmap()
	rb2 := NewBitmap()
	rb3 := NewBitmap()
	for i := uint32(500); i < 75000; i++ {
		rb1.Add(i)
	}
	for i := uint32(0); i < 1000000; i += 7 {
		rb2.Add(i)
	}
	for i := uint32(0); i < 1000000; i += 1001 {
		rb3.Add(i)
	}
	for i := uint32(1000000); i < 2000000; i += 1001 {
		rb1.Add(i)
	}
	for i := uint32(1000000); i < 2000000; i += 3 {
		rb2.Add(i)
	}
	for i := uint32(1000000); i < 2000000; i += 7 {
		rb3.Add(i)
	}
	rb1.RunOptimize()
	rb1.Or(rb2)
	rb1.Or(rb3)
	bigand := And(And(rb1, rb2), rb3)
	bigxor := Xor(Xor(rb1, rb2), rb3)

	assert.True(t, FastOr(rb1, rb2, rb3).Equals(rb1))
	assert.True(t, HeapOr(rb1, rb2, rb3).Equals(rb1))
	assert.Equal(t, rb1.GetCardinality(), HeapOr(rb1, rb2, rb3).GetCardinality())
	assert.True(t, HeapXor(rb1, rb2, rb3).Equals(bigxor))
	assert.True(t, FastAnd(rb1, rb2, rb3).Equals(bigand))
}

func TestFastAggregationsXOR(t *testing.T) {
	rb1 := NewBitmap()
	rb2 := NewBitmap()
	rb3 := NewBitmap()

	for i := uint32(0); i < 40000; i++ {
		rb1.Add(i)
	}
	for i := uint32(0); i < 40000; i += 4000 {
		rb2.Add(i)
	}
	for i := uint32(0); i < 40000; i += 5000 {
		rb3.Add(i)
	}

	assert.EqualValues(t, 40000, rb1.GetCardinality())

	xor1 := Xor(rb1, rb2)
	xor1alt := Xor(rb2, rb1)
	assert.True(t, xor1alt.Equals(xor1))
	assert.True(t, HeapXor(rb1, rb2).Equals(xor1))

	xor2 := Xor(rb2, rb3)
	xor2alt := Xor(rb3, rb2)
	assert.True(t, xor2alt.Equals(xor2))
	assert.True(t, HeapXor(rb2, rb3).Equals(xor2))

	bigxor := Xor(Xor(rb1, rb2), rb3)
	bigxoralt1 := Xor(rb1, Xor(rb2, rb3))
	bigxoralt2 := Xor(rb1, Xor(rb3, rb2))
	bigxoralt3 := Xor(rb3, Xor(rb1, rb2))
	bigxoralt4 := Xor(Xor(rb1, rb2), rb3)

	assert.True(t, bigxoralt2.Equals(bigxor))
	assert.True(t, bigxoralt1.Equals(bigxor))
	assert.True(t, bigxoralt3.Equals(bigxor))
	assert.True(t, bigxoralt4.Equals(bigxor))

	assert.True(t, HeapXor(rb1, rb2, rb3).Equals(bigxor))
}

func TestFastAggregationsXOR_run(t *testing.T) {
	rb1 := NewBitmap()
	rb2 := NewBitmap()
	rb3 := NewBitmap()

	for i := uint32(0); i < 40000; i++ {
		rb1.Add(i)
	}
	rb1.RunOptimize()
	for i := uint32(0); i < 40000; i += 4000 {
		rb2.Add(i)
	}
	for i := uint32(0); i < 40000; i += 5000 {
		rb3.Add(i)
	}

	assert.EqualValues(t, 40000, rb1.GetCardinality())

	xor1 := Xor(rb1, rb2)
	xor1alt := Xor(rb2, rb1)
	assert.True(t, xor1alt.Equals(xor1))
	assert.True(t, HeapXor(rb1, rb2).Equals(xor1))

	xor2 := Xor(rb2, rb3)
	xor2alt := Xor(rb3, rb2)
	assert.True(t, xor2alt.Equals(xor2))
	assert.True(t, HeapXor(rb2, rb3).Equals(xor2))

	bigxor := Xor(Xor(rb1, rb2), rb3)
	bigxoralt1 := Xor(rb1, Xor(rb2, rb3))
	bigxoralt2 := Xor(rb1, Xor(rb3, rb2))
	bigxoralt3 := Xor(rb3, Xor(rb1, rb2))
	bigxoralt4 := Xor(Xor(rb1, rb2), rb3)

	assert.True(t, bigxoralt2.Equals(bigxor))
	assert.True(t, bigxoralt1.Equals(bigxor))
	assert.True(t, bigxoralt3.Equals(bigxor))
	assert.True(t, bigxoralt4.Equals(bigxor))

	assert.True(t, HeapXor(rb1, rb2, rb3).Equals(bigxor))
}

func TestFastAggregationsAndAny(t *testing.T) {
	base := NewBitmap()
	rb1 := NewBitmap()
	rb2 := NewBitmap()
	rb3 := NewBitmap()
	// only one filter has some values
	from := uint32(maxCapacity * 4)
	for i := from; i < from+100; i += 2 {
		rb1.Add(i)
	}
	// only base has values
	from = maxCapacity * 7
	for i := from; i < from+100; i += 2 {
		base.Add(i)
	}
	// base and one of filters have same values
	from = maxCapacity * 8
	for i := from; i < from+100; i += 2 {
		base.Add(i)
		rb1.Add(i)
	}
	// small union
	from = maxCapacity * 10
	for i := from; i < from+1000; i += 10 {
		base.Add(i)
		base.Add(i + i%3)

		rb1.Add(i)
		rb1.Add(i + 1)

		rb2.Add(i + 2)
		rb2.Add(i + i%7)

		rb3.Add(200 + i)
	}
	// run filters
	from = maxCapacity * 10
	for i := from; i < from+1000; i += 3 {
		base.Add(i)
	}
	for i := from; i < from+100; i++ {
		rb1.Add(i)
		rb2.Add(i + 333)
		rb3.Add(i + 433)
	}
	// large union
	from = maxCapacity * 16
	for i := from; i < from+arrayDefaultMaxSize*10; i += 3 {
		base.Add(i)
		base.Add(i + i%2 + 1)
		rb2.Add(i)
		rb3.Add(i + 1)
	}

	// some extra base values
	from = maxCapacity * 17
	for i := from; i < from+1000; i++ {
		base.Add(i)
	}

	base.RunOptimize()
	rb1.RunOptimize()
	rb2.RunOptimize()
	rb3.RunOptimize()

	orFirst := base.Clone()
	orFirst.And(FastOr(rb1, rb2, rb3))

	fast := base.Clone()
	fast.AndAny(rb1, rb2, rb3)

	assert.True(t, fast.Equals(orFirst))
}

func sequentialXor(bitmaps ...*Bitmap) *Bitmap {
	if len(bitmaps) == 0 {
		return NewBitmap()
	}
	res := bitmaps[0].Clone()
	for _, bm := range bitmaps[1:] {
		res = Xor(res, bm)
	}
	return res
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
		{"binary", sequentialXor},
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

			resBinary := sequentialXor(bms...)
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
	expected := sequentialXor(rb_sparse_clone, rb_dense_clone)
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
