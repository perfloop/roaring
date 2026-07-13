package roaring

import (
	"sort"
	"testing"
)

func linearAdvanceUntil(array []uint16, pos int, min uint16) int {
	for i := pos + 1; i < len(array); i++ {
		if array[i] >= min {
			return i
		}
	}
	return len(array)
}

func linearIntersectionCardinality(left, right []uint16) int {
	leftIndex, rightIndex, cardinality := 0, 0, 0
	for leftIndex < len(left) && rightIndex < len(right) {
		if left[leftIndex] < right[rightIndex] {
			leftIndex++
		} else if right[rightIndex] < left[leftIndex] {
			rightIndex++
		} else {
			cardinality++
			leftIndex++
			rightIndex++
		}
	}
	return cardinality
}

func TestAdvanceUntilAgainstLinearSearch(t *testing.T) {
	testArrays := [][]uint16{
		nil,
		{0},
		{0, 2},
		{0, 2, 4, 6, 8, 10, 12, 14, 16},
		{0, 1, 4, 7, 8, 15, 16, 31, 63},
	}
	mins := []uint16{0, 1, 2, 3, 7, 8, 9, 15, 16, 17, 31, 32, 63, 64, 65535}

	for _, array := range testArrays {
		for pos := -1; pos < len(array); pos++ {
			for _, min := range mins {
				got := advanceUntil(array, pos, len(array), min)
				want := linearAdvanceUntil(array, pos, min)
				if got != want {
					t.Fatalf("advanceUntil(%v, %d, %d) = %d, want %d", array, pos, min, got, want)
				}
			}
		}
	}
}

type realDataArrayContainer struct {
	bitmap int
	values []uint16
}

type gallopingIntersectionBenchmarkCase struct {
	small  []uint16
	large  []uint16
	buffer []uint16
	want   int
}

// advanceUntilBenchmarkSink keeps the result of the timed calls observable.
var advanceUntilBenchmarkSink int

func BenchmarkAdvanceUntilGallopingRealData(b *testing.B) {
	b.StopTimer()

	// census-income is a checked-in real-roaring-datasets fixture. The cases
	// below are actual array-container pairs from distinct input bitmaps whose
	// cardinalities select the library's >64:1 galloping path.
	bitmaps, err := retrieveRealDataBitmaps("census-income", false)
	if err != nil {
		b.Fatal(err)
	}
	byKey := make(map[uint16][]realDataArrayContainer)
	for bitmapIndex, bitmap := range bitmaps {
		for containerIndex, c := range bitmap.highlowcontainer.containers {
			ac, ok := c.(*arrayContainer)
			if !ok {
				continue
			}
			key := bitmap.highlowcontainer.keys[containerIndex]
			byKey[key] = append(byKey[key], realDataArrayContainer{
				bitmap: bitmapIndex,
				values: ac.content,
			})
		}
	}

	keys := make([]int, 0, len(byKey))
	for key := range byKey {
		keys = append(keys, int(key))
	}
	sort.Ints(keys)

	cases := make([]gallopingIntersectionBenchmarkCase, 0)
	for _, key := range keys {
		containers := byKey[uint16(key)]
		for left := 0; left < len(containers); left++ {
			for right := left + 1; right < len(containers); right++ {
				if containers[left].bitmap == containers[right].bitmap {
					continue
				}
				small, large := containers[left].values, containers[right].values
				if len(small) > len(large) {
					small, large = large, small
				}
				if len(small) < 16 || len(small)*64 >= len(large) {
					continue
				}

				buffer := make([]uint16, len(small))
				want := linearIntersectionCardinality(small, large)
				if n := intersection2by2(small, large, buffer); n != want {
					b.Fatalf("intersection cardinality = %d, want %d", n, want)
				}
				cases = append(cases, gallopingIntersectionBenchmarkCase{
					small:  small,
					large:  large,
					buffer: buffer,
					want:   want,
				})
			}
		}
	}
	if len(cases) == 0 {
		b.Fatal("real census-income fixture has no >64:1 array-container pairs")
	}

	b.ReportAllocs()
	b.StartTimer()
	total := 0
	expectedTotal := 0
	for i := 0; i < b.N; i++ {
		c := &cases[i%len(cases)]
		total += intersection2by2(c.small, c.large, c.buffer)
		expectedTotal += c.want
	}
	b.StopTimer()
	if total != expectedTotal {
		b.Fatalf("intersection cardinality = %d, want %d", total, expectedTotal)
	}
	advanceUntilBenchmarkSink = total
}
