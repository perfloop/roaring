package roaring

import (
	"slices"
	"testing"
)

type skewedXorArrayCase struct {
	name  string
	left  *arrayContainer
	right *arrayContainer
	want  []uint16
}

func newSkewedXorArrayCases(totalCardinality int) []skewedXorArrayCase {
	values := func(start, length int) []uint16 {
		result := make([]uint16, length)
		for i := range result {
			result[i] = uint16(start + i)
		}
		return result
	}

	return []skewedXorArrayCase{
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
			for _, tc := range newSkewedXorArrayCases(test.totalCardinality) {
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

	t.Run("singleton-middle", func(t *testing.T) {
		tests := []struct {
			name      string
			singleton uint16
			set       []uint16
			want      []uint16
		}{
			{name: "present", singleton: 3, set: []uint16{1, 3, 5}, want: []uint16{1, 5}},
			{name: "absent", singleton: 4, set: []uint16{1, 3, 5}, want: []uint16{1, 3, 4, 5}},
			{name: "after", singleton: 7, set: []uint16{1, 3, 5}, want: []uint16{1, 3, 5, 7}},
		}
		for _, test := range tests {
			t.Run(test.name, func(t *testing.T) {
				singleton := &arrayContainer{content: []uint16{test.singleton}}
				set := &arrayContainer{content: test.set}
				for _, operands := range [][2]*arrayContainer{{singleton, set}, {set, singleton}} {
					result, ok := operands[0].xorArray(operands[1]).(*arrayContainer)
					if !ok {
						t.Fatalf("xor result type = %T, want *arrayContainer", result)
					}
					if !slices.Equal(result.content, test.want) {
						t.Fatalf("xor result = %v, want %v", result.content, test.want)
					}
				}
			})
		}
	})
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

var skewedXorArraySink uint64

func benchmarkSkewedXorArray(b *testing.B, totalCardinality int) {
	cases := newSkewedXorArrayCases(totalCardinality)
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
		skewedXorArraySink += uint64(result.content[len(result.content)-1])
	}
}

func BenchmarkArrayContainerXorTailCopyLarge(b *testing.B) {
	benchmarkSkewedXorArray(b, arrayDefaultMaxSize)
}

func BenchmarkArrayContainerXorTailCopySmall(b *testing.B) {
	benchmarkSkewedXorArray(b, 9)
}

type balancedXorArrayCase struct {
	left  *arrayContainer
	right *arrayContainer
	want  []uint16
}

func newBalancedXorArrayCases() []balancedXorArrayCase {
	values := func(start, length int) []uint16 {
		result := make([]uint16, length)
		for i := range result {
			result[i] = uint16(start + i)
		}
		return result
	}

	return []balancedXorArrayCase{
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

var balancedXorArraySink uint64

func BenchmarkArrayContainerXorTailCopyShort(b *testing.B) {
	cases := newBalancedXorArrayCases()
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
		balancedXorArraySink += uint64(len(result.content))
		if len(result.content) > 0 {
			balancedXorArraySink += uint64(result.content[0])
		}
	}
}

type balancedXorTailCase struct {
	left  *arrayContainer
	right *arrayContainer
	want  []uint16
}

func newBalancedXorTailCases(tailLength int) []balancedXorTailCase {
	values := func(start, length int) []uint16 {
		result := make([]uint16, length)
		for i := range result {
			result[i] = uint16(start + i)
		}
		return result
	}

	commonLength := (arrayDefaultMaxSize - tailLength) / 2
	return []balancedXorTailCase{
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

var balancedXorTailSink uint64

func benchmarkBalancedXorTail(b *testing.B, tailLength int) {
	cases := newBalancedXorTailCases(tailLength)
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
		balancedXorTailSink += uint64(cap(result.content) + len(result.content))
	}
}

func BenchmarkArrayContainerXorTailCopyCrossover64(b *testing.B) {
	benchmarkBalancedXorTail(b, 64)
}

func BenchmarkArrayContainerXorTailCopyCrossover512(b *testing.B) {
	benchmarkBalancedXorTail(b, 512)
}

func BenchmarkArrayContainerXorTailCopyCrossover1024(b *testing.B) {
	benchmarkBalancedXorTail(b, 1024)
}

func BenchmarkArrayContainerXorTailCopyCrossover1025(b *testing.B) {
	benchmarkBalancedXorTail(b, 1025)
}

func BenchmarkArrayContainerXorTailCopyCrossover2048(b *testing.B) {
	benchmarkBalancedXorTail(b, 2048)
}
