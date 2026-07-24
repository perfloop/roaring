package roaring

import "testing"

const (
	parOrHighFanInInputCount  = 32
	parOrHighFanInKeyCount    = 2
	parOrHighFanInParallelism = 4
)

// newParOrHighFanInBitmapInputs makes 32 bitmap-backed inputs sharing two
// adjacent high keys. Each input contributes one of four disjoint bit phases,
// so the known union remains dense without becoming a full run container.
func newParOrHighFanInBitmapInputs() ([]*Bitmap, *Bitmap) {
	inputs := make([]*Bitmap, parOrHighFanInInputCount)
	expected := NewBitmap()

	for key := uint32(0); key < parOrHighFanInKeyCount; key++ {
		base := key << 16
		for low := uint32(0); low < 1<<16; low += 2 {
			expected.Add(base + low)
		}
	}

	for input := range inputs {
		bitmap := NewBitmap()
		phase := uint32(input%4) * 2
		for key := uint32(0); key < parOrHighFanInKeyCount; key++ {
			base := key << 16
			for low := phase; low < 1<<16; low += 8 {
				bitmap.Add(base + low)
			}
		}
		inputs[input] = bitmap
	}

	return inputs, expected
}

func newParOrMixedFallbackInputs() ([]*Bitmap, *Bitmap) {
	const inputCount = 12

	inputs := make([]*Bitmap, inputCount)
	for input := range inputs {
		bitmap := NewBitmap()
		for key := uint32(0); key < parOrHighFanInKeyCount; key++ {
			base := key << 16
			switch input % 3 {
			case 0:
				for low := uint32(input); low < 512; low += 32 {
					bitmap.Add(base + low)
				}
			case 1:
				start := uint64(base) + 4096 + uint64(input)
				bitmap.AddRange(start, start+8192)
			case 2:
				phase := uint32(input % 8)
				for low := phase; low < 1<<16; low += 8 {
					bitmap.Add(base + low)
				}
			}
		}
		if input%3 == 1 {
			bitmap.RunOptimize()
		}
		inputs[input] = bitmap
	}

	expected := NewBitmap()
	for _, input := range inputs {
		iterator := input.Iterator()
		for iterator.HasNext() {
			expected.Add(iterator.Next())
		}
	}

	return inputs, expected
}

func assertBitmapOnlyParOrInputs(t testing.TB, inputs []*Bitmap) {
	t.Helper()

	if len(inputs) != parOrHighFanInInputCount {
		t.Fatalf("got %d high-fan-in inputs, want %d", len(inputs), parOrHighFanInInputCount)
	}
	for inputIndex, input := range inputs {
		if input.highlowcontainer.size() != parOrHighFanInKeyCount {
			t.Fatalf("input %d has %d containers, want %d", inputIndex, input.highlowcontainer.size(), parOrHighFanInKeyCount)
		}
		for containerIndex, container := range input.highlowcontainer.containers {
			if _, ok := container.(*bitmapContainer); !ok {
				t.Fatalf("input %d container %d is %T, want *bitmapContainer", inputIndex, containerIndex, container)
			}
		}
	}
}

func assertMixedParOrInputs(t testing.TB, inputs []*Bitmap) {
	t.Helper()

	var haveArray, haveBitmap, haveRun bool
	for _, input := range inputs {
		for _, container := range input.highlowcontainer.containers {
			switch container.(type) {
			case *arrayContainer:
				haveArray = true
			case *bitmapContainer:
				haveBitmap = true
			case *runContainer16:
				haveRun = true
			}
		}
	}
	if !haveArray || !haveBitmap || !haveRun {
		t.Fatalf("mixed fallback inputs missing container types: array=%t bitmap=%t run=%t", haveArray, haveBitmap, haveRun)
	}
}

func cloneBitmaps(inputs []*Bitmap) []*Bitmap {
	clones := make([]*Bitmap, len(inputs))
	for i, input := range inputs {
		clones[i] = input.Clone()
	}
	return clones
}

func assertParOrResult(t testing.TB, got, expected *Bitmap) {
	t.Helper()

	if got.GetCardinality() != expected.GetCardinality() {
		t.Fatalf("got cardinality %d, want %d", got.GetCardinality(), expected.GetCardinality())
	}
	if !got.Equals(expected) {
		t.Fatal("ParOr result differs from independently constructed union")
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("invalid ParOr result: %v", err)
	}
}

func assertInputsUnchanged(t testing.TB, inputs, snapshots []*Bitmap) {
	t.Helper()

	for i, input := range inputs {
		if !input.Equals(snapshots[i]) {
			t.Fatalf("ParOr mutated input %d", i)
		}
	}
}

func TestParOrHighFanInBitmap(t *testing.T) {
	inputs, expected := newParOrHighFanInBitmapInputs()
	assertBitmapOnlyParOrInputs(t, inputs)
	snapshots := cloneBitmaps(inputs)

	for _, parallelism := range []int{1, 2, parOrHighFanInParallelism} {
		t.Run("parallelism", func(t *testing.T) {
			got := ParOr(parallelism, inputs...)
			assertParOrResult(t, got, expected)
			assertInputsUnchanged(t, inputs, snapshots)
		})
	}
}

func TestParOrMixedFallback(t *testing.T) {
	inputs, expected := newParOrMixedFallbackInputs()
	assertMixedParOrInputs(t, inputs)
	snapshots := cloneBitmaps(inputs)

	got := ParOr(parOrHighFanInParallelism, inputs...)
	assertParOrResult(t, got, expected)
	assertInputsUnchanged(t, inputs, snapshots)
}

func BenchmarkParOrHighFanInBitmap(b *testing.B) {
	inputs, expected := newParOrHighFanInBitmapInputs()
	assertBitmapOnlyParOrInputs(b, inputs)
	assertParOrResult(b, ParOr(parOrHighFanInParallelism, inputs...), expected)

	b.ReportAllocs()
	var total uint64
	for b.Loop() {
		result := ParOr(parOrHighFanInParallelism, inputs...)
		total += result.GetCardinality()
	}
	if total != expected.GetCardinality()*uint64(b.N) {
		b.Fatalf("got total cardinality %d, want %d", total, expected.GetCardinality()*uint64(b.N))
	}
}

func BenchmarkParOrMixedFallback(b *testing.B) {
	inputs, expected := newParOrMixedFallbackInputs()
	assertMixedParOrInputs(b, inputs)
	assertParOrResult(b, ParOr(parOrHighFanInParallelism, inputs...), expected)

	b.ReportAllocs()
	var total uint64
	for b.Loop() {
		result := ParOr(parOrHighFanInParallelism, inputs...)
		total += result.GetCardinality()
	}
	if total != expected.GetCardinality()*uint64(b.N) {
		b.Fatalf("got total cardinality %d, want %d", total, expected.GetCardinality()*uint64(b.N))
	}
}
