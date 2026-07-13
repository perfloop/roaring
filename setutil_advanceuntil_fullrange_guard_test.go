package roaring

import (
	"sort"
	"testing"
)

type advanceUntilFullRangeCase struct {
	array []uint16
	pos   int
	min   uint16
	want  int
}

func legacyAdvanceUntilFullRange(array []uint16, pos int, min uint16) (int, bool) {
	lower := pos + 1
	if lower >= len(array) || array[lower] >= min {
		return lower, false
	}

	spansize := 1
	for lower+spansize < len(array) && array[lower+spansize] < min {
		spansize *= 2
	}
	upper := len(array) - 1
	if lower+spansize < len(array) {
		upper = lower + spansize
	}
	if array[upper] == min {
		return upper, false
	}
	if array[upper] < min {
		return len(array), false
	}

	lower += spansize >> 1
	enteredBinarySearch := false
	for lower+1 != upper {
		enteredBinarySearch = true
		mid := (lower + upper) >> 1
		if array[mid] == min {
			return mid, enteredBinarySearch
		} else if array[mid] < min {
			lower = mid
		} else {
			upper = mid
		}
	}
	return upper, enteredBinarySearch
}

func fullRangeAdvanceUntilArrays() [][]uint16 {
	base := make([]uint16, 0, 261)
	for i := 0; i < 256; i++ {
		base = append(base, uint16(i*257))
	}
	base = append(base, 0, 32767, 32768, 65534, 65535)
	sort.Slice(base, func(i, j int) bool { return base[i] < base[j] })

	duplicates := append([]uint16(nil), base...)
	duplicates = append(duplicates, 32767, 32768, 65534)
	sort.Slice(duplicates, func(i, j int) bool { return duplicates[i] < duplicates[j] })

	return [][]uint16{base, duplicates}
}

var fullRangeAdvanceUntilMins = []uint16{
	0, 1, 256, 257, 32766, 32767, 32768, 32769, 65533, 65534, 65535,
}

func TestAdvanceUntilFullUint16Differential(t *testing.T) {
	binarySearchMins := make(map[uint16]bool)
	for _, array := range fullRangeAdvanceUntilArrays() {
		if err := (&arrayContainer{content: array}).validate(); err != nil {
			t.Fatalf("full-range array should validate: %v", err)
		}
		for pos := -1; pos < len(array); pos++ {
			for _, min := range fullRangeAdvanceUntilMins {
				want, enteredBinarySearch := legacyAdvanceUntilFullRange(array, pos, min)
				got := advanceUntil(array, pos, len(array), min)
				if got != want {
					t.Fatalf("advanceUntil full-range array at pos %d/min %d = %d, want legacy index %d", pos, min, got, want)
				}
				if enteredBinarySearch {
					binarySearchMins[min] = true
				}
			}
		}
	}

	for _, min := range []uint16{32767, 32768, 65534} {
		if !binarySearchMins[min] {
			t.Fatalf("min %d did not reach the binary-search loop", min)
		}
	}
}

func fullRangeAdvanceUntilBenchmarkCases(b *testing.B) []advanceUntilFullRangeCase {
	cases := make([]advanceUntilFullRangeCase, 0)
	for _, array := range fullRangeAdvanceUntilArrays() {
		for pos := -1; pos < len(array); pos++ {
			for _, min := range fullRangeAdvanceUntilMins {
				want, enteredBinarySearch := legacyAdvanceUntilFullRange(array, pos, min)
				if enteredBinarySearch {
					cases = append(cases, advanceUntilFullRangeCase{
						array: array,
						pos:   pos,
						min:   min,
						want:  want,
					})
				}
			}
		}
	}
	if len(cases) == 0 {
		b.Fatal("full-range fixture has no binary-search cases")
	}
	return cases
}

var advanceUntilFullRangeBenchmarkSink int

func BenchmarkAdvanceUntilFullUint16Differential(b *testing.B) {
	b.StopTimer()
	cases := fullRangeAdvanceUntilBenchmarkCases(b)

	b.ReportAllocs()
	b.StartTimer()
	actual := 0
	expected := 0
	for i := 0; i < b.N; i++ {
		c := &cases[i%len(cases)]
		actual += advanceUntil(c.array, c.pos, len(c.array), c.min)
		expected += c.want
	}
	b.StopTimer()
	if actual != expected {
		b.Fatalf("advanceUntil total = %d, want %d", actual, expected)
	}
	advanceUntilFullRangeBenchmarkSink = actual
}
