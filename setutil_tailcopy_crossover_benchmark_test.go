package roaring

import (
	"slices"
	"testing"
)

type xorTailCopyCrossoverCase struct {
	left  *arrayContainer
	right *arrayContainer
	want  []uint16
}

func newXorTailCopyCrossoverCases(tailLength int) []xorTailCopyCrossoverCase {
	values := func(start, length int) []uint16 {
		result := make([]uint16, length)
		for i := range result {
			result[i] = uint16(start + i)
		}
		return result
	}

	commonLength := (arrayDefaultMaxSize - tailLength) / 2
	return []xorTailCopyCrossoverCase{
		{
			left:  &arrayContainer{content: values(0, commonLength)},
			right: &arrayContainer{content: values(0, commonLength+tailLength)},
			want:  values(commonLength, tailLength),
		},
		{
			left:  &arrayContainer{content: values(0, commonLength+tailLength)},
			right: &arrayContainer{content: values(0, commonLength)},
			want:  values(commonLength, tailLength),
		},
	}
}

var arrayContainerXorTailCopyCrossoverSink uint64

func benchmarkArrayContainerXorTailCopyCrossover(b *testing.B, tailLength int) {
	cases := newXorTailCopyCrossoverCases(tailLength)
	for _, tc := range cases {
		result, ok := tc.left.xorArray(tc.right).(*arrayContainer)
		if !ok {
			b.Fatalf("xor result type = %T, want *arrayContainer", result)
		}
		if !slices.Equal(result.content, tc.want) {
			b.Fatalf("xor result = %v, want %v", result.content, tc.want)
		}
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tc := &cases[i%len(cases)]
		result := tc.left.xorArray(tc.right).(*arrayContainer)
		arrayContainerXorTailCopyCrossoverSink += uint64(cap(result.content) + len(result.content))
	}
}

func BenchmarkArrayContainerXorTailCopyCrossover64(b *testing.B) {
	benchmarkArrayContainerXorTailCopyCrossover(b, 64)
}

func BenchmarkArrayContainerXorTailCopyCrossover512(b *testing.B) {
	benchmarkArrayContainerXorTailCopyCrossover(b, 512)
}

func BenchmarkArrayContainerXorTailCopyCrossover1024(b *testing.B) {
	benchmarkArrayContainerXorTailCopyCrossover(b, 1024)
}

func BenchmarkArrayContainerXorTailCopyCrossover1025(b *testing.B) {
	benchmarkArrayContainerXorTailCopyCrossover(b, 1025)
}

func BenchmarkArrayContainerXorTailCopyCrossover2048(b *testing.B) {
	benchmarkArrayContainerXorTailCopyCrossover(b, 2048)
}
