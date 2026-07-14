package roaring

import "testing"

func BenchmarkArrayContainerXorSingletonSmallMiddle(b *testing.B) {
	set := &arrayContainer{content: []uint16{0, 2, 4, 6, 8, 10, 12, 14}}
	cases := []arrayXorBenchmarkCase{
		{
			left:  &arrayContainer{content: []uint16{8}},
			right: set,
			want:  []uint16{0, 2, 4, 6, 10, 12, 14},
		},
		{
			left:  set,
			right: &arrayContainer{content: []uint16{8}},
			want:  []uint16{0, 2, 4, 6, 10, 12, 14},
		},
		{
			left:  &arrayContainer{content: []uint16{9}},
			right: set,
			want:  []uint16{0, 2, 4, 6, 8, 9, 10, 12, 14},
		},
		{
			left:  set,
			right: &arrayContainer{content: []uint16{9}},
			want:  []uint16{0, 2, 4, 6, 8, 9, 10, 12, 14},
		},
		{
			left:  &arrayContainer{content: []uint16{16}},
			right: set,
			want:  []uint16{0, 2, 4, 6, 8, 10, 12, 14, 16},
		},
		{
			left:  set,
			right: &arrayContainer{content: []uint16{16}},
			want:  []uint16{0, 2, 4, 6, 8, 10, 12, 14, 16},
		},
	}
	benchmarkArrayXorCases(b, cases)
}
