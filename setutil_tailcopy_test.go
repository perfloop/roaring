package roaring

import (
	"slices"
	"testing"
)

type xorTailCopyCase struct {
	name  string
	left  *arrayContainer
	right *arrayContainer
	want  []uint16
}

func newXorTailCopyCases(totalCardinality int) []xorTailCopyCase {
	values := func(start, length int) []uint16 {
		result := make([]uint16, length)
		for i := range result {
			result[i] = uint16(start + i)
		}
		return result
	}

	return []xorTailCopyCase{
		{
			name:  "set1-exhausted-after-less-than",
			left:  &arrayContainer{content: values(0, 1)},
			right: &arrayContainer{content: values(1, totalCardinality-1)},
			want:  values(0, totalCardinality),
		},
		{
			name:  "set1-exhausted-after-equal",
			left:  &arrayContainer{content: values(0, 1)},
			right: &arrayContainer{content: values(0, totalCardinality-1)},
			want:  values(1, totalCardinality-2),
		},
		{
			name:  "set2-exhausted-after-equal",
			left:  &arrayContainer{content: values(0, totalCardinality-1)},
			right: &arrayContainer{content: values(0, 1)},
			want:  values(1, totalCardinality-2),
		},
		{
			name:  "set2-exhausted-after-greater-than",
			left:  &arrayContainer{content: values(1, totalCardinality-1)},
			right: &arrayContainer{content: values(0, 1)},
			want:  values(0, totalCardinality),
		},
	}
}

func TestExclusiveUnion2by2TailCopy(t *testing.T) {
	tests := []struct {
		name             string
		totalCardinality int
	}{
		// This creates tail lengths on both sides of the 1024-value cutoff.
		{name: "crossover", totalCardinality: 1026},
		{name: "maximum", totalCardinality: arrayDefaultMaxSize},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			for _, tc := range newXorTailCopyCases(test.totalCardinality) {
				t.Run(tc.name, func(t *testing.T) {
					buffer := make([]uint16, len(tc.left.content)+len(tc.right.content))
					length := exclusiveUnion2by2(tc.left.content, tc.right.content, buffer)
					if got := buffer[:length]; !slices.Equal(got, tc.want) {
						t.Fatalf("exclusive union = %v, want %v", got, tc.want)
					}

					result, ok := tc.left.xorArray(tc.right).(*arrayContainer)
					if !ok {
						t.Fatalf("xor result type = %T, want *arrayContainer", result)
					}
					if !slices.Equal(result.content, tc.want) {
						t.Fatalf("xor result = %v, want %v", result.content, tc.want)
					}

					t.Run("undersized-buffer", func(t *testing.T) {
						buffer := make([]uint16, len(tc.want)-1)
						requirePanic(t, func() {
							exclusiveUnion2by2(tc.left.content, tc.right.content, buffer)
						})
					})
				})
			}
		})
	}
}

func requirePanic(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic for undersized output buffer")
		}
	}()
	f()
}

var arrayContainerXorTailCopySink uint64

func benchmarkArrayContainerXorTailCopy(b *testing.B, totalCardinality int) {
	cases := newXorTailCopyCases(totalCardinality)
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
		tc := &cases[i&3]
		result := tc.left.xorArray(tc.right).(*arrayContainer)
		arrayContainerXorTailCopySink += uint64(result.content[len(result.content)-1])
	}
}

func BenchmarkArrayContainerXorTailCopyLarge(b *testing.B) {
	benchmarkArrayContainerXorTailCopy(b, arrayDefaultMaxSize)
}

func BenchmarkArrayContainerXorTailCopySmall(b *testing.B) {
	benchmarkArrayContainerXorTailCopy(b, 9)
}
