package roaring64

import "testing"

var orCardinality64MatrixReverseSink uint64

func BenchmarkOrCardinality64MatrixReverse(b *testing.B) {
	for _, fixture := range orCardinality64PairFixtures() {
		if fixture.name != "nested-array-run" && fixture.name != "nested-bitmap-run" {
			continue
		}
		fixture := fixture
		want := Or(fixture.left, fixture.right).GetCardinality()
		b.Run(fixture.name, func(b *testing.B) {
			b.ReportAllocs()
			var got uint64
			for b.Loop() {
				got = fixture.right.OrCardinality(fixture.left)
			}
			if got != want {
				b.Fatalf("reverse OrCardinality = %d, want %d", got, want)
			}
			orCardinality64MatrixReverseSink = got
		})
	}
}
