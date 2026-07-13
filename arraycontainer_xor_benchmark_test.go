package roaring

import (
	"slices"
	"testing"
)

type arrayXorBenchmarkCase struct {
	left  *arrayContainer
	right *arrayContainer
	want  []uint16
}

func verifyArrayXorCases(b *testing.B, cases []arrayXorBenchmarkCase) {
	b.Helper()
	for _, tc := range cases {
		result, ok := tc.left.xorArray(tc.right).(*arrayContainer)
		if !ok {
			b.Fatalf("xor result type = %T, want *arrayContainer", result)
		}
		if !slices.Equal(result.content, tc.want) {
			b.Fatalf("xor result = %v, want %v", result.content, tc.want)
		}
	}
}

var arrayXorBenchmarkSink uint64

func benchmarkArrayXorCases(b *testing.B, cases []arrayXorBenchmarkCase) {
	verifyArrayXorCases(b, cases)

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tc := &cases[i%len(cases)]
		result := tc.left.xorArray(tc.right).(*arrayContainer)
		arrayXorBenchmarkSink += uint64(cap(result.content)) + uint64(result.content[len(result.content)-1])
	}
}

func BenchmarkArrayContainerXorSmallBalanced(b *testing.B) {
	cases := []arrayXorBenchmarkCase{
		{
			left:  &arrayContainer{content: []uint16{0, 2}},
			right: &arrayContainer{content: []uint16{1, 3}},
			want:  []uint16{0, 1, 2, 3},
		},
		{
			left:  &arrayContainer{content: []uint16{0, 4, 8, 12}},
			right: &arrayContainer{content: []uint16{1, 5, 9, 13}},
			want:  []uint16{0, 1, 4, 5, 8, 9, 12, 13},
		},
		{
			left:  &arrayContainer{content: []uint16{0, 2, 4, 6, 8, 10, 12, 14}},
			right: &arrayContainer{content: []uint16{1, 3, 5, 7, 9, 11, 13, 15}},
			want:  []uint16{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15},
		},
	}
	benchmarkArrayXorCases(b, cases)
}

func BenchmarkArrayContainerXorSingletonMiddle(b *testing.B) {
	set := make([]uint16, 2048)
	for i := range set {
		set[i] = uint16(i * 2)
	}
	withoutMiddle := append([]uint16{}, set[:1024]...)
	withoutMiddle = append(withoutMiddle, set[1025:]...)
	withMiddle := append([]uint16{}, set[:1025]...)
	withMiddle = append(withMiddle, uint16(2049))
	withMiddle = append(withMiddle, set[1025:]...)

	present := &arrayContainer{content: []uint16{2048}}
	absent := &arrayContainer{content: []uint16{2049}}
	array := &arrayContainer{content: set}
	cases := []arrayXorBenchmarkCase{
		{left: present, right: array, want: withoutMiddle},
		{left: array, right: present, want: withoutMiddle},
		{left: absent, right: array, want: withMiddle},
		{left: array, right: absent, want: withMiddle},
	}
	benchmarkArrayXorCases(b, cases)
}
