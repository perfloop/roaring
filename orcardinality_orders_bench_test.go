package roaring

import "testing"

var orCardinalityMatrixReverseSink uint64

func BenchmarkOrCardinalityMatrixReverse(b *testing.B) {
	for _, group := range orCardinalityFixtureGroups() {
		if group.name != "array-run" && group.name != "bitmap-run" {
			continue
		}
		group := group
		b.Run(group.name, func(b *testing.B) {
			b.ReportAllocs()
			var got uint64
			for b.Loop() {
				got = 0
				for _, fixture := range group.fixtures {
					got += fixture.right.OrCardinality(fixture.left)
				}
			}
			if got != group.want {
				b.Fatalf("reverse OrCardinality total = %d, want %d", got, group.want)
			}
			orCardinalityMatrixReverseSink = got
		})
	}
}
