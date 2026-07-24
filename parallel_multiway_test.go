package roaring

import "testing"

const multiwayBitmapOrTestInputCount = 512

func newSerializedOneKeyBitmaps(t *testing.T, key uint16) [][]byte {
	t.Helper()

	serialized := make([][]byte, 4)
	for phase := uint32(0); phase < uint32(len(serialized)); phase++ {
		bitmap := NewBitmap()
		for low := phase * 2; low <= MaxUint16; low += 8 {
			bitmap.Add(uint32(key)<<16 | low)
		}
		if _, ok := bitmap.highlowcontainer.containers[0].(*bitmapContainer); !ok {
			t.Fatal("test input is not bitmap-backed")
		}
		var err error
		serialized[phase], err = bitmap.ToBytes()
		if err != nil {
			t.Fatalf("serialize bitmap: %v", err)
		}
	}
	return serialized
}

func loadSerializedOneKeyBitmaps(t *testing.T, serialized [][]byte, count int) []*Bitmap {
	t.Helper()

	bitmaps := make([]*Bitmap, count)
	for i := range bitmaps {
		bitmaps[i] = NewBitmap()
		if _, err := bitmaps[i].FromBuffer(serialized[i%len(serialized)]); err != nil {
			t.Fatalf("deserialize bitmap %d: %v", i, err)
		}
		if bitmaps[i].highlowcontainer.size() != 1 {
			t.Fatalf("bitmap %d has %d containers, want 1", i, bitmaps[i].highlowcontainer.size())
		}
		if _, ok := bitmaps[i].highlowcontainer.containers[0].(*bitmapContainer); !ok {
			t.Fatalf("bitmap %d is not bitmap-backed", i)
		}
		if !bitmaps[i].highlowcontainer.needCopyOnWrite[0] {
			t.Fatalf("bitmap %d is not copy-on-write after FromBuffer", i)
		}
	}
	return bitmaps
}

func TestParOrMultiwayBitmapOrFromBuffer(t *testing.T) {
	if multiwayBitmapOrTestInputCount != multiwayBitmapOrMinInputs {
		t.Fatalf("test uses %d inputs, fast path starts at %d", multiwayBitmapOrTestInputCount, multiwayBitmapOrMinInputs)
	}

	const key = uint16(7)
	serialized := newSerializedOneKeyBitmaps(t, key)
	parallelInputs := loadSerializedOneKeyBitmaps(t, serialized, multiwayBitmapOrTestInputCount)
	fastInputs := loadSerializedOneKeyBitmaps(t, serialized, multiwayBitmapOrTestInputCount)
	inputSnapshots := make([]*Bitmap, len(parallelInputs))
	for i, input := range parallelInputs {
		inputSnapshots[i] = input.Clone()
	}

	want := FastOr(fastInputs...)
	got := ParOr(4, parallelInputs...)
	if !got.Equals(want) {
		t.Fatal("ParOr result differs from FastOr")
	}
	if got.GetCardinality() != want.GetCardinality() {
		t.Fatalf("ParOr cardinality %d, want %d", got.GetCardinality(), want.GetCardinality())
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("validate ParOr result: %v", err)
	}
	if err := want.Validate(); err != nil {
		t.Fatalf("validate FastOr result: %v", err)
	}

	for i, input := range parallelInputs {
		if !input.Equals(inputSnapshots[i]) {
			t.Fatalf("ParOr modified input %d", i)
		}
		if err := input.Validate(); err != nil {
			t.Fatalf("validate input %d after ParOr: %v", i, err)
		}
	}

	mutation := uint32(key)<<16 | 1
	if got.Contains(mutation) {
		t.Fatalf("test mutation value %d is already present", mutation)
	}
	got.Add(mutation)
	if !got.Contains(mutation) {
		t.Fatalf("ParOr result did not retain mutation %d", mutation)
	}
	for i, input := range parallelInputs {
		if input.Contains(mutation) {
			t.Fatalf("mutating ParOr result changed imported input %d", i)
		}
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("validate mutated ParOr result: %v", err)
	}
}

func TestParOrMultiwayBitmapOrFullResult(t *testing.T) {
	if multiwayBitmapOrTestInputCount != multiwayBitmapOrMinInputs {
		t.Fatalf("test uses %d inputs, fast path starts at %d", multiwayBitmapOrTestInputCount, multiwayBitmapOrMinInputs)
	}

	const key = uint16(9)
	sources := make([]*Bitmap, 4)
	for phase := uint32(0); phase < uint32(len(sources)); phase++ {
		sources[phase] = NewBitmap()
		for low := phase; low <= MaxUint16; low += uint32(len(sources)) {
			sources[phase].Add(uint32(key)<<16 | low)
		}
		if _, ok := sources[phase].highlowcontainer.containers[0].(*bitmapContainer); !ok {
			t.Fatal("test input is not bitmap-backed")
		}
	}

	inputs := make([]*Bitmap, multiwayBitmapOrTestInputCount)
	inputSnapshots := make([]*Bitmap, len(inputs))
	for i := range inputs {
		inputs[i] = sources[i%len(sources)].Clone()
		inputSnapshots[i] = inputs[i].Clone()
	}

	got := ParOr(4, inputs...)
	want := NewBitmap()
	want.AddRange(uint64(key)<<16, uint64(key+1)<<16)
	if !got.Equals(want) {
		t.Fatal("ParOr full result differs from expected bitmap")
	}
	if got.GetCardinality() != want.GetCardinality() {
		t.Fatalf("ParOr cardinality %d, want %d", got.GetCardinality(), want.GetCardinality())
	}
	if _, ok := got.highlowcontainer.containers[0].(*runContainer16); !ok {
		t.Fatalf("ParOr full result has %T container, want *runContainer16", got.highlowcontainer.containers[0])
	}
	if err := got.Validate(); err != nil {
		t.Fatalf("validate ParOr result: %v", err)
	}

	for i, input := range inputs {
		if !input.Equals(inputSnapshots[i]) {
			t.Fatalf("ParOr modified input %d", i)
		}
		if err := input.Validate(); err != nil {
			t.Fatalf("validate input %d after ParOr: %v", i, err)
		}
	}
}
