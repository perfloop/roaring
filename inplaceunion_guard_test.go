package roaring

import (
	"runtime"
	"slices"
	"testing"
)

func TestRunContainer16InplaceUnionMultiRunReference(t *testing.T) {
	overlappingLeft := []interval16{
		newInterval16Range(10, 20),
		newInterval16Range(40, 45),
		newInterval16Range(70, 80),
	}
	overlappingRight := []interval16{
		newInterval16Range(15, 30),
		newInterval16Range(40, 50),
		newInterval16Range(90, 95),
	}
	disjointLeft := []interval16{
		newInterval16Range(10, 10),
		newInterval16Range(30, 30),
		newInterval16Range(50, 50),
	}
	disjointRight := []interval16{
		newInterval16Range(15, 15),
		newInterval16Range(35, 35),
		newInterval16Range(55, 55),
	}

	assertInplaceUnionMatchesReference(t, "OverlappingExactAllocation", overlappingLeft, overlappingRight, len(overlappingLeft))
	assertInplaceUnionMatchesReference(t, "OverlappingSpareCapacity", overlappingLeft, overlappingRight, len(overlappingLeft)+len(overlappingRight))
	assertInplaceUnionMatchesReference(t, "DisjointExactAllocation", disjointLeft, disjointRight, len(disjointLeft))
	assertInplaceUnionMatchesReference(t, "DisjointSpareCapacity", disjointLeft, disjointRight, len(disjointLeft)+len(disjointRight))
}

func assertInplaceUnionMatchesReference(t *testing.T, name string, left, right []interval16, capacity int) {
	t.Helper()

	t.Run(name, func(t *testing.T) {
		receiver := newInplaceUnionGuardContainer(left, capacity)
		result := receiver.inplaceUnion(&runContainer16{iv: right})

		if err := result.validate(); err != nil {
			t.Fatalf("union result is invalid: %v", err)
		}
		if got, want := inplaceUnionGuardValues(result), inplaceUnionGuardReference(left, right); !slices.Equal(got, want) {
			t.Fatalf("union values = %v, want %v", got, want)
		}
	})
}

func inplaceUnionGuardReference(left, right []interval16) []uint16 {
	reference := newRunContainer16()
	for _, intervals := range [][]interval16{left, right} {
		for _, interval := range intervals {
			for value, last := interval.start, interval.last(); value <= last; value++ {
				reference.Add(value)
			}
		}
	}
	return inplaceUnionGuardValues(reference)
}

func inplaceUnionGuardValues(c container) []uint16 {
	values := make([]uint16, 0, c.getCardinality())
	iterator := c.getShortIterator()
	for iterator.hasNext() {
		values = append(values, iterator.next())
	}
	return values
}

func newInplaceUnionGuardContainer(intervals []interval16, capacity int) *runContainer16 {
	container := &runContainer16{iv: make([]interval16, len(intervals), capacity)}
	copy(container.iv, intervals)
	return container
}

func BenchmarkRunContainerInplaceUnionGuards(b *testing.B) {
	overlappingLeft := inplaceUnionGuardRuns(100, 128, 8, 24)
	overlappingRight := inplaceUnionGuardRuns(104, 128, 8, 24)
	sparseReceiver := inplaceUnionGuardRuns(100, 256, 1, 9)
	middleAdditions := []interval16{
		newInterval16Range(1005, 1005),
		newInterval16Range(1505, 1505),
	}
	appendAdditions := []interval16{
		newInterval16Range(2700, 2700),
		newInterval16Range(2720, 2720),
	}

	b.Run("EqualRunsOverlappingRealloc", func(b *testing.B) {
		benchmarkInplaceUnionGuard(b, overlappingLeft, overlappingRight, len(overlappingLeft))
	})
	b.Run("EqualRunsOverlappingSpareCapacity", func(b *testing.B) {
		benchmarkInplaceUnionGuard(b, overlappingLeft, overlappingRight, len(overlappingLeft)+len(overlappingRight))
	})
	b.Run("SparseMultiRunMiddleRealloc", func(b *testing.B) {
		benchmarkInplaceUnionGuard(b, sparseReceiver, middleAdditions, len(sparseReceiver))
	})
	b.Run("SparseMultiRunMiddleSpareCapacity", func(b *testing.B) {
		benchmarkInplaceUnionGuard(b, sparseReceiver, middleAdditions, len(sparseReceiver)+len(middleAdditions))
	})
	b.Run("SparseMultiRunAppendRealloc", func(b *testing.B) {
		benchmarkInplaceUnionGuard(b, sparseReceiver, appendAdditions, len(sparseReceiver))
	})
	b.Run("SparseMultiRunAppendSpareCapacity", func(b *testing.B) {
		benchmarkInplaceUnionGuard(b, sparseReceiver, appendAdditions, len(sparseReceiver)+len(appendAdditions))
	})
}

func benchmarkInplaceUnionGuard(b *testing.B, left, right []interval16, capacity int) {
	b.Helper()

	check := newInplaceUnionGuardContainer(left, capacity)
	result := check.inplaceUnion(&runContainer16{iv: right})
	if err := result.validate(); err != nil {
		b.Fatalf("union result is invalid: %v", err)
	}
	if got, want := inplaceUnionGuardValues(result), inplaceUnionGuardReference(left, right); !slices.Equal(got, want) {
		b.Fatalf("union values = %v, want %v", got, want)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		receiver := newInplaceUnionGuardContainer(left, capacity)
		result := receiver.inplaceUnion(&runContainer16{iv: right})
		runtime.KeepAlive(result)
	}
}

func inplaceUnionGuardRuns(start uint16, count, length, gap int) []interval16 {
	intervals := make([]interval16, count)
	step := length + gap
	for i := range intervals {
		first := start + uint16(i*step)
		intervals[i] = newInterval16Range(first, first+uint16(length-1))
	}
	return intervals
}
