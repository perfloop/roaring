package roaring

import (
	"slices"
	"testing"
)

type xorShortTailCase struct {
	left  *arrayContainer
	right *arrayContainer
	want  []uint16
}

func newXorShortTailCopyCases() []xorShortTailCase {
	values := func(start, length int) []uint16 {
		result := make([]uint16, length)
		for i := range result {
			result[i] = uint16(start + i)
		}
		return result
	}

	return []xorShortTailCase{
		{
			left:  &arrayContainer{content: values(0, 2048)},
			right: &arrayContainer{content: values(0, 2048)},
			want:  nil,
		},
		{
			left:  &arrayContainer{content: values(0, 2047)},
			right: &arrayContainer{content: values(0, 2048)},
			want:  values(2047, 1),
		},
		{
			left:  &arrayContainer{content: values(0, 2048)},
			right: &arrayContainer{content: values(0, 2047)},
			want:  values(2047, 1),
		},
		{
			left:  &arrayContainer{content: values(0, 2046)},
			right: &arrayContainer{content: values(0, 2050)},
			want:  values(2046, 4),
		},
		{
			left:  &arrayContainer{content: values(0, 2050)},
			right: &arrayContainer{content: values(0, 2046)},
			want:  values(2046, 4),
		},
	}
}

var arrayContainerXorShortTailCopySink uint64

func BenchmarkArrayContainerXorTailCopyShort(b *testing.B) {
	cases := newXorShortTailCopyCases()
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
		arrayContainerXorShortTailCopySink += uint64(len(result.content))
		if len(result.content) > 0 {
			arrayContainerXorShortTailCopySink += uint64(result.content[0])
		}
	}
}
