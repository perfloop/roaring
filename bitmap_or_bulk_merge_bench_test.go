package roaring

import (
	"bytes"
	"testing"
)

type bitmapOrBulkMergeFixture struct {
	lefts       [2]*Bitmap
	rights      [2]*Bitmap
	wants       [2]*Bitmap
	cardinality uint64
}

func newBitmapOrBulkMergeFixture(leftKeys, rightKeys []uint16, copyOnWrite bool) bitmapOrBulkMergeFixture {
	fixture := bitmapOrBulkMergeFixture{}
	for variant := range fixture.lefts {
		left := New()
		right := New()
		values := make([]uint32, 0, len(leftKeys)+len(rightKeys))
		leftLow := uint16(variant * 2)
		rightLow := leftLow + 1

		for _, key := range leftKeys {
			value := uint32(key)<<16 | uint32(leftLow)
			left.Add(value)
			values = append(values, value)
		}
		for _, key := range rightKeys {
			value := uint32(key)<<16 | uint32(rightLow)
			right.Add(value)
			values = append(values, value)
		}

		if copyOnWrite {
			left.SetCopyOnWrite(true)
			right.SetCopyOnWrite(true)
		}

		fixture.lefts[variant] = left
		fixture.rights[variant] = right
		fixture.wants[variant] = BitmapOf(values...)
	}
	fixture.cardinality = fixture.wants[0].GetCardinality()
	return fixture
}

func bitmapOrBulkMergeKeys(start, count, step int) []uint16 {
	keys := make([]uint16, count)
	for i := range keys {
		keys[i] = uint16(start + i*step)
	}
	return keys
}

func bitmapOrBulkMergeInterleavedFixture(containers int, copyOnWrite bool) bitmapOrBulkMergeFixture {
	return newBitmapOrBulkMergeFixture(
		bitmapOrBulkMergeKeys(0, containers, 2),
		bitmapOrBulkMergeKeys(1, containers, 2),
		copyOnWrite,
	)
}

func bitmapOrBulkMergeAppendFixture(containers int) bitmapOrBulkMergeFixture {
	return newBitmapOrBulkMergeFixture(
		bitmapOrBulkMergeKeys(0, containers, 1),
		bitmapOrBulkMergeKeys(containers, containers, 1),
		false,
	)
}

func bitmapOrBulkMergeOverlapFixture(containers int) bitmapOrBulkMergeFixture {
	keys := bitmapOrBulkMergeKeys(0, containers, 1)
	return newBitmapOrBulkMergeFixture(keys, keys, false)
}

func bitmapOrBulkMergeSingleInteriorFixture(containers int) bitmapOrBulkMergeFixture {
	leftKeys := make([]uint16, 0, containers-1)
	middle := containers / 2
	for key := 0; key < containers; key++ {
		if key != middle {
			leftKeys = append(leftKeys, uint16(key))
		}
	}
	return newBitmapOrBulkMergeFixture(leftKeys, []uint16{uint16(middle)}, false)
}

func bitmapOrBulkMergeTailAdjacentFixture() bitmapOrBulkMergeFixture {
	const containers = 4096

	leftKeys := make([]uint16, 0, containers-1)
	for key := 0; key < containers-2; key++ {
		leftKeys = append(leftKeys, uint16(key))
	}
	leftKeys = append(leftKeys, containers-1)
	return newBitmapOrBulkMergeFixture(leftKeys, []uint16{containers - 2}, false)
}

func bitmapOrBulkMergeFixtureCases() []struct {
	name string
	new  func() bitmapOrBulkMergeFixture
} {
	return []struct {
		name string
		new  func() bitmapOrBulkMergeFixture
	}{
		{"fresh-interleaved-64", func() bitmapOrBulkMergeFixture { return bitmapOrBulkMergeInterleavedFixture(64, false) }},
		{"fresh-interleaved-65", func() bitmapOrBulkMergeFixture { return bitmapOrBulkMergeInterleavedFixture(65, false) }},
		{"fresh-interleaved-1024", func() bitmapOrBulkMergeFixture { return bitmapOrBulkMergeInterleavedFixture(1024, false) }},
		{"fresh-interleaved-4096", func() bitmapOrBulkMergeFixture { return bitmapOrBulkMergeInterleavedFixture(4096, false) }},
		{"fresh-append-only-4096", func() bitmapOrBulkMergeFixture { return bitmapOrBulkMergeAppendFixture(4096) }},
		{"fresh-overlap-4096", func() bitmapOrBulkMergeFixture { return bitmapOrBulkMergeOverlapFixture(4096) }},
		{"fresh-copy-on-write-interleaved-4096", func() bitmapOrBulkMergeFixture { return bitmapOrBulkMergeInterleavedFixture(4096, true) }},
		{"fresh-single-interior-4096", func() bitmapOrBulkMergeFixture { return bitmapOrBulkMergeSingleInteriorFixture(4096) }},
		{"fresh-tail-adjacent-4096", bitmapOrBulkMergeTailAdjacentFixture},
	}
}

func TestBitmapOrBulkMergeFixtures(t *testing.T) {
	for _, test := range bitmapOrBulkMergeFixtureCases() {
		t.Run(test.name, func(t *testing.T) {
			fixture := test.new()
			receiver := fixture.lefts[0].Clone()
			receiver.Or(fixture.rights[0])

			if !receiver.Equals(fixture.wants[0]) {
				t.Fatalf("unexpected union: got %v, want %v", receiver, fixture.wants[0])
			}
			if receiver.GetCardinality() != fixture.cardinality {
				t.Fatalf("unexpected cardinality: got %d, want %d", receiver.GetCardinality(), fixture.cardinality)
			}
			if err := receiver.Validate(); err != nil {
				t.Fatalf("union produced an invalid bitmap: %v", err)
			}
		})
	}
}

func TestBitmapOrBulkMergeCopyOnWrite(t *testing.T) {
	fixture := bitmapOrBulkMergeInterleavedFixture(64, true)
	left := fixture.lefts[0]
	right := fixture.rights[0]
	receiver := left.Clone()
	receiver.Or(right)

	const (
		leftKey       = uint32(0) << 16
		tailKey       = uint32(127) << 16
		receiverValue = uint32(10)
		sourceValue   = uint32(11)
	)

	receiver.Add(tailKey | receiverValue)
	if right.Contains(tailKey | receiverValue) {
		t.Fatal("receiver mutation changed a source-only tail container")
	}
	right.Add(tailKey | sourceValue)
	if receiver.Contains(tailKey | sourceValue) {
		t.Fatal("source mutation changed a receiver tail container")
	}

	receiver.Add(leftKey | receiverValue)
	if left.Contains(leftKey | receiverValue) {
		t.Fatal("receiver mutation changed a receiver-only container")
	}
	left.Add(leftKey | sourceValue)
	if receiver.Contains(leftKey | sourceValue) {
		t.Fatal("left mutation changed a receiver container")
	}

	for name, bitmap := range map[string]*Bitmap{"receiver": receiver, "left": left, "right": right} {
		if err := bitmap.Validate(); err != nil {
			t.Fatalf("%s became invalid after copy-on-write mutations: %v", name, err)
		}
	}
}

type bitmapLoadBoundaryFixture struct {
	source      *Bitmap
	portable    []byte
	frozen      []byte
	cardinality uint64
}

func newBitmapLoadBoundaryFixture(containers int) bitmapLoadBoundaryFixture {
	source := New()
	for key := 0; key < containers; key++ {
		source.Add(uint32(key)<<16 | uint32(key&1))
	}

	portable, err := source.ToBytes()
	if err != nil {
		panic(err)
	}
	frozen, err := source.Freeze()
	if err != nil {
		panic(err)
	}
	return bitmapLoadBoundaryFixture{
		source:      source,
		portable:    portable,
		frozen:      frozen,
		cardinality: source.GetCardinality(),
	}
}

func TestBitmapLoadBoundaryFixtures(t *testing.T) {
	fixture := newBitmapLoadBoundaryFixture(4096)

	portable := New()
	if _, err := portable.ReadFrom(bytes.NewReader(fixture.portable)); err != nil {
		t.Fatalf("read portable bitmap: %v", err)
	}
	if !portable.Equals(fixture.source) {
		t.Fatal("portable load changed the bitmap")
	}

	frozen := New()
	if err := frozen.FrozenView(fixture.frozen); err != nil {
		t.Fatalf("read frozen bitmap: %v", err)
	}
	if !frozen.Equals(fixture.source) {
		t.Fatal("frozen load changed the bitmap")
	}
}

func benchmarkBitmapOrBulkMerge(b *testing.B, fixture bitmapOrBulkMergeFixture) {
	b.ReportAllocs()
	fixtureIndex := 0
	var cardinality uint64
	b.ResetTimer()
	for b.Loop() {
		receiver := fixture.lefts[fixtureIndex].Clone()
		receiver.Or(fixture.rights[fixtureIndex])
		cardinality += receiver.GetCardinality()
		fixtureIndex ^= 1
	}
	b.StopTimer()
	if cardinality != fixture.cardinality*uint64(b.N) {
		b.Fatalf("unexpected total cardinality: got %d, want %d", cardinality, fixture.cardinality*uint64(b.N))
	}
}

type bitmapOrBulkMergeSerializedPair struct {
	left  []byte
	right []byte
}

type bitmapOrBulkMergeLoadedFixture struct {
	portable    [2]bitmapOrBulkMergeSerializedPair
	frozen      [2]bitmapOrBulkMergeSerializedPair
	wants       [2]*Bitmap
	cardinality uint64
}

func newBitmapOrBulkMergeLoadedFixture() bitmapOrBulkMergeLoadedFixture {
	source := bitmapOrBulkMergeInterleavedFixture(4096, false)
	fixture := bitmapOrBulkMergeLoadedFixture{
		wants:       source.wants,
		cardinality: source.cardinality,
	}
	for variant := range source.lefts {
		portableLeft, err := source.lefts[variant].ToBytes()
		if err != nil {
			panic(err)
		}
		portableRight, err := source.rights[variant].ToBytes()
		if err != nil {
			panic(err)
		}
		frozenLeft, err := source.lefts[variant].Freeze()
		if err != nil {
			panic(err)
		}
		frozenRight, err := source.rights[variant].Freeze()
		if err != nil {
			panic(err)
		}
		fixture.portable[variant] = bitmapOrBulkMergeSerializedPair{left: portableLeft, right: portableRight}
		fixture.frozen[variant] = bitmapOrBulkMergeSerializedPair{left: frozenLeft, right: frozenRight}
	}
	return fixture
}

func bitmapOrBulkMergeReadFrom(receiver *Bitmap, data []byte) error {
	_, err := receiver.ReadFrom(bytes.NewReader(data))
	return err
}

func bitmapOrBulkMergeFrozenView(receiver *Bitmap, data []byte) error {
	return receiver.FrozenView(data)
}

func TestBitmapOrBulkMergeLoadedFixtures(t *testing.T) {
	fixture := newBitmapOrBulkMergeLoadedFixture()
	for _, test := range []struct {
		name  string
		pairs [2]bitmapOrBulkMergeSerializedPair
		load  func(*Bitmap, []byte) error
	}{
		{name: "read-from", pairs: fixture.portable, load: bitmapOrBulkMergeReadFrom},
		{name: "frozen-view", pairs: fixture.frozen, load: bitmapOrBulkMergeFrozenView},
	} {
		t.Run(test.name, func(t *testing.T) {
			for variant, pair := range test.pairs {
				receiver := New()
				if err := test.load(receiver, pair.left); err != nil {
					t.Fatalf("load receiver: %v", err)
				}
				source := New()
				if err := test.load(source, pair.right); err != nil {
					t.Fatalf("load source: %v", err)
				}
				receiver.Or(source)
				if !receiver.Equals(fixture.wants[variant]) {
					t.Fatalf("unexpected union for fixture %d", variant)
				}
				if err := receiver.Validate(); err != nil {
					t.Fatalf("union produced an invalid bitmap: %v", err)
				}
			}
		})
	}
}

func benchmarkBitmapOrBulkMergeLoaded(b *testing.B, fixture bitmapOrBulkMergeLoadedFixture, pairs [2]bitmapOrBulkMergeSerializedPair, load func(*Bitmap, []byte) error) {
	b.ReportAllocs()
	fixtureIndex := 0
	var cardinality uint64
	b.ResetTimer()
	for b.Loop() {
		pair := pairs[fixtureIndex]
		receiver := New()
		if err := load(receiver, pair.left); err != nil {
			b.Fatal(err)
		}
		source := New()
		if err := load(source, pair.right); err != nil {
			b.Fatal(err)
		}
		receiver.Or(source)
		cardinality += receiver.GetCardinality()
		fixtureIndex ^= 1
	}
	b.StopTimer()
	if cardinality != fixture.cardinality*uint64(b.N) {
		b.Fatalf("unexpected total cardinality: got %d, want %d", cardinality, fixture.cardinality*uint64(b.N))
	}
}

func BenchmarkBitmapOrBulkMerge(b *testing.B) {
	for _, benchmark := range bitmapOrBulkMergeFixtureCases() {
		benchmark := benchmark
		b.Run(benchmark.name, func(b *testing.B) {
			benchmarkBitmapOrBulkMerge(b, benchmark.new())
		})
	}
	b.Run("loaded-read-from-interleaved-4096", func(b *testing.B) {
		fixture := newBitmapOrBulkMergeLoadedFixture()
		benchmarkBitmapOrBulkMergeLoaded(b, fixture, fixture.portable, bitmapOrBulkMergeReadFrom)
	})
	b.Run("loaded-frozen-view-interleaved-4096", func(b *testing.B) {
		fixture := newBitmapOrBulkMergeLoadedFixture()
		benchmarkBitmapOrBulkMergeLoaded(b, fixture, fixture.frozen, bitmapOrBulkMergeFrozenView)
	})
}
