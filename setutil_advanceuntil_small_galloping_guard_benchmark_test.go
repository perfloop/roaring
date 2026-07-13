package roaring

import (
	"sort"
	"testing"
)

type smallGallopingBenchmarkCase struct {
	small  []uint16
	large  []uint16
	buffer []uint16
	want   int
}

func loadRealDataSmallGallopingCases(b *testing.B) []smallGallopingBenchmarkCase {
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

	cases := make([]smallGallopingBenchmarkCase, 0)
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
				if len(small) == 0 || len(small) >= 16 || len(small)*64 >= len(large) {
					continue
				}

				buffer := make([]uint16, len(small))
				want := linearIntersectionCardinality(small, large)
				if n := intersection2by2(small, large, buffer); n != want {
					b.Fatalf("intersection cardinality = %d, want %d", n, want)
				}
				cases = append(cases, smallGallopingBenchmarkCase{
					small:  small,
					large:  large,
					buffer: buffer,
					want:   want,
				})
			}
		}
	}
	if len(cases) == 0 {
		b.Fatal("real census-income fixture has no 1..15-element >64:1 array-container pairs")
	}
	return cases
}

var advanceUntilSmallGallopingBenchmarkSink int

func BenchmarkAdvanceUntilGallopingRealDataSmall(b *testing.B) {
	b.StopTimer()

	// This guard covers the real >64:1 dispatch-valid cases excluded by the
	// primary selector because their small array contains only 1..15 values.
	cases := loadRealDataSmallGallopingCases(b)

	b.ReportAllocs()
	b.StartTimer()
	actual := 0
	expected := 0
	for i := 0; i < b.N; i++ {
		c := &cases[i%len(cases)]
		actual += intersection2by2(c.small, c.large, c.buffer)
		expected += c.want
	}
	b.StopTimer()
	if actual != expected {
		b.Fatalf("intersection cardinality = %d, want %d", actual, expected)
	}
	advanceUntilSmallGallopingBenchmarkSink = actual
}
