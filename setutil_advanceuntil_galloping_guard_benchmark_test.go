package roaring

import (
	"sort"
	"testing"
)

type gallopingGuardPair struct {
	small       []uint16
	large       []uint16
	cardinality int
}

func loadRealDataGallopingGuardPairs(b *testing.B) []gallopingGuardPair {
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

	pairs := make([]gallopingGuardPair, 0)
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

				cardinality := linearIntersectionCardinality(small, large)
				if got := intersection2by2Cardinality(small, large); got != cardinality {
					b.Fatalf("intersection cardinality = %d, want %d", got, cardinality)
				}
				if got := intersects2by2(small, large); got != (cardinality > 0) {
					b.Fatalf("intersection exists = %t, want %t", got, cardinality > 0)
				}
				pairs = append(pairs, gallopingGuardPair{
					small:       small,
					large:       large,
					cardinality: cardinality,
				})
			}
		}
	}
	if len(pairs) == 0 {
		b.Fatal("real census-income fixture has no >64:1 array-container pairs")
	}
	return pairs
}

var advanceUntilCardinalityBenchmarkSink int

func BenchmarkAdvanceUntilGallopingRealDataCardinality(b *testing.B) {
	b.StopTimer()
	pairs := loadRealDataGallopingGuardPairs(b)

	b.ReportAllocs()
	b.StartTimer()
	actual := 0
	expected := 0
	for i := 0; i < b.N; i++ {
		pair := &pairs[i%len(pairs)]
		actual += intersection2by2Cardinality(pair.small, pair.large)
		expected += pair.cardinality
	}
	b.StopTimer()
	if actual != expected {
		b.Fatalf("intersection cardinality = %d, want %d", actual, expected)
	}
	advanceUntilCardinalityBenchmarkSink = actual
}

var advanceUntilBoolBenchmarkSink int

func BenchmarkAdvanceUntilGallopingRealDataBool(b *testing.B) {
	b.StopTimer()
	pairs := loadRealDataGallopingGuardPairs(b)

	b.ReportAllocs()
	b.StartTimer()
	actual := 0
	expected := 0
	for i := 0; i < b.N; i++ {
		pair := &pairs[i%len(pairs)]
		if intersects2by2(pair.small, pair.large) {
			actual++
		}
		if pair.cardinality > 0 {
			expected++
		}
	}
	b.StopTimer()
	if actual != expected {
		b.Fatalf("intersection existence count = %d, want %d", actual, expected)
	}
	advanceUntilBoolBenchmarkSink = actual
}
