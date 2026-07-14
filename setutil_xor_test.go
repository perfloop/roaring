package roaring

import (
	"slices"
	"testing"
)

func uint16Sequence(start, length int) []uint16 {
	values := make([]uint16, length)
	for i := range values {
		values[i] = uint16(start + i)
	}
	return values
}

func newSingletonXorArrayCases(totalCardinality int) []arrayXorBenchmarkCase {
	cases := []arrayXorBenchmarkCase{
		{
			left:  &arrayContainer{content: uint16Sequence(0, 1)},
			right: &arrayContainer{content: uint16Sequence(1, totalCardinality-1)},
			want:  uint16Sequence(0, totalCardinality),
		},
		{
			left:  &arrayContainer{content: uint16Sequence(0, 1)},
			right: &arrayContainer{content: uint16Sequence(0, totalCardinality-1)},
			want:  uint16Sequence(1, totalCardinality-2),
		},
		{
			left:  &arrayContainer{content: uint16Sequence(0, totalCardinality-1)},
			right: &arrayContainer{content: uint16Sequence(0, 1)},
			want:  uint16Sequence(1, totalCardinality-2),
		},
		{
			left:  &arrayContainer{content: uint16Sequence(1, totalCardinality-1)},
			right: &arrayContainer{content: uint16Sequence(0, 1)},
			want:  uint16Sequence(0, totalCardinality),
		},
	}
	if totalCardinality != arrayDefaultMaxSize {
		return cases
	}

	set := make([]uint16, totalCardinality-1)
	for i := range set {
		set[i] = uint16(2 * i)
	}
	middle := len(set) / 2
	present := set[middle]
	withoutPresent := append([]uint16{}, set[:middle]...)
	withoutPresent = append(withoutPresent, set[middle+1:]...)
	absent := present + 1
	withAbsent := append([]uint16{}, set[:middle+1]...)
	withAbsent = append(withAbsent, absent)
	withAbsent = append(withAbsent, set[middle+1:]...)
	after := set[len(set)-1] + 1
	withAfter := append(append([]uint16{}, set...), after)

	for _, test := range []struct {
		singleton uint16
		want      []uint16
	}{
		{singleton: present, want: withoutPresent},
		{singleton: absent, want: withAbsent},
		{singleton: after, want: withAfter},
	} {
		singleton := &arrayContainer{content: []uint16{test.singleton}}
		array := &arrayContainer{content: set}
		cases = append(cases,
			arrayXorBenchmarkCase{left: singleton, right: array, want: test.want},
			arrayXorBenchmarkCase{left: array, right: singleton, want: test.want},
		)
	}
	return cases
}

func TestExclusiveUnion2by2TailCopy(t *testing.T) {
	for _, tc := range newSingletonXorArrayCases(arrayDefaultMaxSize) {
		buffer := make([]uint16, len(tc.left.content)+len(tc.right.content))
		length := exclusiveUnion2by2(tc.left.content, tc.right.content, buffer)
		if got := buffer[:length]; !slices.Equal(got, tc.want) {
			t.Fatalf("exclusive union = %v, want %v", got, tc.want)
		}

		undersized := make([]uint16, len(tc.want)-1)
		requirePanic(t, func() {
			exclusiveUnion2by2(tc.left.content, tc.right.content, undersized)
		})
	}
}

func TestExclusiveUnionWithSingleton(t *testing.T) {
	tests := []struct {
		name      string
		singleton uint16
		set       []uint16
		want      []uint16
	}{
		{name: "empty", singleton: 1, want: []uint16{1}},
		{name: "before", singleton: 0, set: []uint16{1, 3, 5}, want: []uint16{0, 1, 3, 5}},
		{name: "equal", singleton: 1, set: []uint16{1, 3, 5}, want: []uint16{3, 5}},
		{name: "present-middle", singleton: 3, set: []uint16{1, 3, 5}, want: []uint16{1, 5}},
		{name: "absent-middle", singleton: 4, set: []uint16{1, 3, 5}, want: []uint16{1, 3, 4, 5}},
		{name: "after", singleton: 7, set: []uint16{1, 3, 5}, want: []uint16{1, 3, 5, 7}},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			buffer := make([]uint16, len(test.want))
			length := exclusiveUnionWithSingleton(test.singleton, test.set, buffer)
			if got := buffer[:length]; !slices.Equal(got, test.want) {
				t.Fatalf("exclusive union = %v, want %v", got, test.want)
			}

			undersized := make([]uint16, len(test.want)-1)
			requirePanic(t, func() {
				exclusiveUnionWithSingleton(test.singleton, test.set, undersized)
			})

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

func BenchmarkArrayContainerXorTailCopyLarge(b *testing.B) {
	benchmarkArrayXorCases(b, newSingletonXorArrayCases(arrayDefaultMaxSize))
}

func BenchmarkArrayContainerXorTailCopySmall(b *testing.B) {
	benchmarkArrayXorCases(b, newSingletonXorArrayCases(9))
}

func BenchmarkArrayContainerXorTailCopyShort(b *testing.B) {
	cases := []arrayXorBenchmarkCase{
		{
			left:  &arrayContainer{content: uint16Sequence(0, 2048)},
			right: &arrayContainer{content: uint16Sequence(0, 2048)},
		},
		{
			left:  &arrayContainer{content: uint16Sequence(0, 2047)},
			right: &arrayContainer{content: uint16Sequence(0, 2048)},
			want:  uint16Sequence(2047, 1),
		},
		{
			left:  &arrayContainer{content: uint16Sequence(0, 2048)},
			right: &arrayContainer{content: uint16Sequence(0, 2047)},
			want:  uint16Sequence(2047, 1),
		},
		{
			left:  &arrayContainer{content: uint16Sequence(0, 2046)},
			right: &arrayContainer{content: uint16Sequence(0, 2050)},
			want:  uint16Sequence(2046, 4),
		},
		{
			left:  &arrayContainer{content: uint16Sequence(0, 2050)},
			right: &arrayContainer{content: uint16Sequence(0, 2046)},
			want:  uint16Sequence(2046, 4),
		},
	}
	benchmarkArrayXorCases(b, cases)
}

func newNearLimitXorCases(tailLength int) []arrayXorBenchmarkCase {
	commonLength := (arrayDefaultMaxSize - tailLength) / 2
	return []arrayXorBenchmarkCase{
		{
			left:  &arrayContainer{content: uint16Sequence(0, commonLength)},
			right: &arrayContainer{content: uint16Sequence(0, commonLength+tailLength)},
			want:  uint16Sequence(commonLength, tailLength),
		},
		{
			left:  &arrayContainer{content: uint16Sequence(0, commonLength+tailLength)},
			right: &arrayContainer{content: uint16Sequence(0, commonLength)},
			want:  uint16Sequence(commonLength, tailLength),
		},
	}
}

func BenchmarkArrayContainerXorTailCopyCrossover64(b *testing.B) {
	benchmarkArrayXorCases(b, newNearLimitXorCases(64))
}

func BenchmarkArrayContainerXorTailCopyCrossover512(b *testing.B) {
	benchmarkArrayXorCases(b, newNearLimitXorCases(512))
}

func BenchmarkArrayContainerXorTailCopyCrossover1024(b *testing.B) {
	benchmarkArrayXorCases(b, newNearLimitXorCases(1024))
}

func BenchmarkArrayContainerXorTailCopyCrossover1025(b *testing.B) {
	benchmarkArrayXorCases(b, newNearLimitXorCases(1025))
}

func BenchmarkArrayContainerXorTailCopyCrossover2048(b *testing.B) {
	benchmarkArrayXorCases(b, newNearLimitXorCases(2048))
}
