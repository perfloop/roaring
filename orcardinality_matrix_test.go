package roaring

import (
	"bytes"
	"encoding/binary"
	"testing"
)

var orCardinalityMatrixSink uint64

type orCardinalityPair struct {
	name      string
	leftKind  string
	rightKind string
}

type orCardinalityShape struct {
	name             string
	leftCardinality  int
	rightCardinality int
	relation         string
	skewed           bool
}

type orCardinalityFixture struct {
	name  string
	left  *Bitmap
	right *Bitmap
}

type orCardinalityFixtureGroup struct {
	name     string
	fixtures []orCardinalityFixture
	want     uint64
}

func TestOrCardinalityMatrix(t *testing.T) {
	for _, group := range orCardinalityFixtureGroups() {
		for _, fixture := range group.fixtures {
			fixture := fixture
			t.Run(fixture.name, func(t *testing.T) {
				if err := fixture.left.Validate(); err != nil {
					t.Fatalf("invalid left bitmap: %v", err)
				}
				if err := fixture.right.Validate(); err != nil {
					t.Fatalf("invalid right bitmap: %v", err)
				}

				leftBefore := fixture.left.Clone()
				rightBefore := fixture.right.Clone()
				want := Or(fixture.left, fixture.right).GetCardinality()

				if got := fixture.left.OrCardinality(fixture.right); got != want {
					t.Fatalf("forward OrCardinality = %d, want %d", got, want)
				}
				if got := fixture.right.OrCardinality(fixture.left); got != want {
					t.Fatalf("reverse OrCardinality = %d, want %d", got, want)
				}
				if !fixture.left.Equals(leftBefore) {
					t.Fatal("OrCardinality modified the left bitmap")
				}
				if !fixture.right.Equals(rightBefore) {
					t.Fatal("OrCardinality modified the right bitmap")
				}
			})
		}
	}
}

type malformedRunInterval struct {
	start  uint16
	length uint16
}

func TestOrCardinalityMalformedRunDeserialization(t *testing.T) {
	malformedRuns := []struct {
		name      string
		intervals []malformedRunInterval
	}{
		{name: "overlapping", intervals: []malformedRunInterval{{start: 1, length: 2}, {start: 2, length: 2}}},
		{name: "unsorted", intervals: []malformedRunInterval{{start: 4}, {start: 1}}},
		{name: "adjacent", intervals: []malformedRunInterval{{start: 1, length: 1}, {start: 3, length: 1}}},
	}
	decoders := []struct {
		name   string
		decode func(*Bitmap, []byte) error
	}{
		{
			name: "UnmarshalBinary",
			decode: func(bitmap *Bitmap, data []byte) error {
				return bitmap.UnmarshalBinary(data)
			},
		},
		{
			name: "ReadFrom",
			decode: func(bitmap *Bitmap, data []byte) error {
				_, err := bitmap.ReadFrom(bytes.NewReader(data))
				return err
			},
		},
	}

	for _, malformed := range malformedRuns {
		malformed := malformed
		for _, decoder := range decoders {
			decoder := decoder
			t.Run(malformed.name+"/"+decoder.name, func(t *testing.T) {
				bitmap := NewBitmap()
				if err := decoder.decode(bitmap, malformedRunBitmapBytes(malformed.intervals)); err != nil {
					t.Fatalf("decode malformed run bitmap: %v", err)
				}
				if err := bitmap.Validate(); err == nil {
					t.Fatal("malformed run bitmap unexpectedly validated")
				}
				selfWant := Or(bitmap, bitmap).GetCardinality()
				if got := bitmap.OrCardinality(bitmap); got != selfWant {
					t.Fatalf("run/run OrCardinality = %d, want materialized union cardinality %d", got, selfWant)
				}

				for _, peer := range malformedRunPeers() {
					if err := peer.bitmap.Validate(); err != nil {
						t.Fatalf("invalid %s peer: %v", peer.name, err)
					}
					if peer.name == "bitmap" {
						if _, ok := peer.bitmap.highlowcontainer.getContainerAtIndex(0).(*bitmapContainer); !ok {
							t.Fatal("bitmap peer did not produce a bitmap container")
						}
					}
					forwardWant := Or(bitmap, peer.bitmap).GetCardinality()
					if got := bitmap.OrCardinality(peer.bitmap); got != forwardWant {
						t.Fatalf("run/%s OrCardinality = %d, want materialized union cardinality %d", peer.name, got, forwardWant)
					}
					reverseWant := Or(peer.bitmap, bitmap).GetCardinality()
					if got := peer.bitmap.OrCardinality(bitmap); got != reverseWant {
						t.Fatalf("%s/run OrCardinality = %d, want materialized union cardinality %d", peer.name, got, reverseWant)
					}
				}
			})
		}
	}
}

type malformedRunPeer struct {
	name   string
	bitmap *Bitmap
}

func malformedRunPeers() []malformedRunPeer {
	arrayOverlap := NewBitmap()
	arrayOverlap.Add(2)

	arrayDisjoint := NewBitmap()
	arrayDisjoint.Add(100)

	bitmap := NewBitmap()
	bitmap.Add(2)
	bitmap.AddRange(1000, 1000+uint64(arrayDefaultMaxSize))

	return []malformedRunPeer{
		{name: "array-overlap", bitmap: arrayOverlap},
		{name: "array-disjoint", bitmap: arrayDisjoint},
		{name: "bitmap", bitmap: bitmap},
	}
}

func malformedRunBitmapBytes(intervals []malformedRunInterval) []byte {
	// ReadFrom accepts run intervals without validation, so this exercises the
	// public deserialization boundary with non-canonical but syntactically valid runs.
	data := make([]byte, 11+4*len(intervals))
	binary.LittleEndian.PutUint16(data[0:], uint16(serialCookie))
	binary.LittleEndian.PutUint16(data[2:], 0) // one container
	data[4] = 1                                // run-container bitmap
	binary.LittleEndian.PutUint16(data[5:], 0) // key

	cardinality := 0
	for _, interval := range intervals {
		cardinality += int(interval.length) + 1
	}
	binary.LittleEndian.PutUint16(data[7:], uint16(cardinality-1))
	binary.LittleEndian.PutUint16(data[9:], uint16(len(intervals)))
	for index, interval := range intervals {
		offset := 11 + 4*index
		binary.LittleEndian.PutUint16(data[offset:], interval.start)
		binary.LittleEndian.PutUint16(data[offset+2:], interval.length)
	}
	return data
}

func BenchmarkOrCardinalityMatrix(b *testing.B) {
	for _, group := range orCardinalityFixtureGroups() {
		group := group
		b.Run(group.name, func(b *testing.B) {
			b.ReportAllocs()
			var got uint64
			for b.Loop() {
				got = 0
				for _, fixture := range group.fixtures {
					got += fixture.left.OrCardinality(fixture.right)
				}
			}
			if got != group.want {
				b.Fatalf("OrCardinality total = %d, want %d", got, group.want)
			}
			orCardinalityMatrixSink = got
		})
	}
}

func orCardinalityFixtureGroups() []orCardinalityFixtureGroup {
	groups := make([]orCardinalityFixtureGroup, 0, len(orCardinalityPairs()))
	for _, pair := range orCardinalityPairs() {
		fixtures := make([]orCardinalityFixture, 0, len(orCardinalityShapes())+1)
		for _, shape := range orCardinalityShapes() {
			fixtures = append(fixtures, newOrCardinalityFixture(pair, shape))
		}
		fixtures = append(fixtures, newOrCardinalityMultiFixture(pair))

		group := orCardinalityFixtureGroup{name: pair.name, fixtures: fixtures}
		for _, fixture := range fixtures {
			group.want += Or(fixture.left, fixture.right).GetCardinality()
		}
		groups = append(groups, group)
	}
	return groups
}

func newOrCardinalityFixture(pair orCardinalityPair, shape orCardinalityShape) orCardinalityFixture {
	leftCardinality, rightCardinality := orCardinalityShapeCardinalities(pair, shape)
	leftValues, rightValues := orCardinalityValues(leftCardinality, rightCardinality, shape.relation)
	return orCardinalityFixture{
		name:  pair.name + "-" + shape.name,
		left:  newOrCardinalityBitmap(0, newOrCardinalityContainer(pair.leftKind, leftValues)),
		right: newOrCardinalityBitmap(0, newOrCardinalityContainer(pair.rightKind, rightValues)),
	}
}

func newOrCardinalityMultiFixture(pair orCardinalityPair) orCardinalityFixture {
	left := NewBitmap()
	right := NewBitmap()
	left.highlowcontainer.appendContainer(0, newOrCardinalityContainer(pair.leftKind, matrixValues(orCardinalityUnmatchedCardinality(pair.leftKind), 1024)), false)

	for index, shape := range orCardinalityShapes() {
		leftCardinality, rightCardinality := orCardinalityShapeCardinalities(pair, shape)
		leftValues, rightValues := orCardinalityValues(leftCardinality, rightCardinality, shape.relation)
		key := uint16(index*2 + 1)
		left.highlowcontainer.appendContainer(key, newOrCardinalityContainer(pair.leftKind, leftValues), false)
		right.highlowcontainer.appendContainer(key, newOrCardinalityContainer(pair.rightKind, rightValues), false)
	}
	right.highlowcontainer.appendContainer(12, newOrCardinalityContainer(pair.rightKind, matrixValues(orCardinalityUnmatchedCardinality(pair.rightKind), 43000)), false)

	return orCardinalityFixture{
		name:  pair.name + "-multi-container",
		left:  left,
		right: right,
	}
}

func orCardinalityUnmatchedCardinality(kind string) int {
	if kind == "bitmap" {
		return 4096
	}
	return 32
}

func newOrCardinalityBitmap(key uint16, value container) *Bitmap {
	bitmap := NewBitmap()
	bitmap.highlowcontainer.appendContainer(key, value, false)
	return bitmap
}

func newOrCardinalityContainer(kind string, values []uint16) container {
	switch kind {
	case "array":
		return &arrayContainer{content: append([]uint16(nil), values...)}
	case "bitmap":
		bitmap := newBitmapContainer()
		for _, value := range values {
			bitmap.iadd(value)
		}
		return bitmap
	case "run":
		return newRunContainer16FromVals(true, values...)
	default:
		panic("unknown container kind: " + kind)
	}
}

func orCardinalityPairs() []orCardinalityPair {
	return []orCardinalityPair{
		{name: "array-array", leftKind: "array", rightKind: "array"},
		{name: "array-bitmap", leftKind: "array", rightKind: "bitmap"},
		{name: "array-run", leftKind: "array", rightKind: "run"},
		{name: "bitmap-bitmap", leftKind: "bitmap", rightKind: "bitmap"},
		{name: "bitmap-run", leftKind: "bitmap", rightKind: "run"},
		{name: "run-run", leftKind: "run", rightKind: "run"},
	}
}

func orCardinalityShapes() []orCardinalityShape {
	return []orCardinalityShape{
		{name: "balanced-disjoint", leftCardinality: 4096, rightCardinality: 4096, relation: "disjoint"},
		{name: "balanced-overlap", leftCardinality: 4096, rightCardinality: 4096, relation: "overlap"},
		{name: "balanced-identical", leftCardinality: 4096, rightCardinality: 4096, relation: "identical"},
		{name: "skewed-disjoint", leftCardinality: 32, rightCardinality: 4096, relation: "disjoint", skewed: true},
		{name: "skewed-overlap", leftCardinality: 32, rightCardinality: 4096, relation: "overlap", skewed: true},
	}
}

func orCardinalityShapeCardinalities(pair orCardinalityPair, shape orCardinalityShape) (int, int) {
	if !shape.skewed {
		return shape.leftCardinality, shape.rightCardinality
	}

	switch pair.name {
	case "bitmap-bitmap":
		// A bitmap container is valid only at cardinalities of at least 4096.
		return 4096, 32768
	case "bitmap-run":
		// Keep the canonical pair order while putting the legal 32-cardinality
		// side in the run container.
		return 4096, 32
	default:
		return shape.leftCardinality, shape.rightCardinality
	}
}

func orCardinalityValues(leftCardinality, rightCardinality int, relation string) ([]uint16, []uint16) {
	switch relation {
	case "disjoint":
		return matrixValues(leftCardinality, 1024), matrixValues(rightCardinality, 43000)
	case "overlap":
		shared := min(leftCardinality, rightCardinality) / 2
		left := append(matrixValues(leftCardinality-shared, 1024), matrixValues(shared, 22000)...)
		right := append(matrixValues(shared, 22000), matrixValues(rightCardinality-shared, 43000)...)
		return left, right
	case "identical":
		if leftCardinality != rightCardinality {
			panic("identical inputs require equal cardinalities")
		}
		left := matrixValues(leftCardinality, 22000)
		return left, append([]uint16(nil), left...)
	default:
		panic("unknown relation: " + relation)
	}
}

func matrixValues(cardinality, base int) []uint16 {
	if cardinality == 0 {
		return nil
	}

	groups := cardinality / 64
	if groups < 1 {
		groups = 1
	}
	groups = min(8, groups)
	values := make([]uint16, 0, cardinality)
	remaining := cardinality
	for group := 0; group < groups; group++ {
		groupCardinality := remaining / (groups - group)
		start := base + group*2048
		for value := 0; value < groupCardinality; value++ {
			values = append(values, uint16(start+value))
		}
		remaining -= groupCardinality
	}
	return values
}
