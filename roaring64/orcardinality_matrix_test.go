package roaring64

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/RoaringBitmap/roaring/v2"
)

var orCardinality64MatrixSink uint64

type orCardinality64Fixture struct {
	name  string
	left  *Bitmap
	right *Bitmap
}

type orCardinality64NestedPair struct {
	name        string
	leftKind    string
	rightKind   string
	leftOffset  uint32
	rightOffset uint32
}

func TestOrCardinalityMatrix64(t *testing.T) {
	fixtures := append(orCardinality64Fixtures(), orCardinality64PairFixtures()...)
	for _, fixture := range fixtures {
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

type malformedRunInterval64 struct {
	start  uint16
	length uint16
}

func TestOrCardinalityMalformedRunDeserialization64(t *testing.T) {
	malformedRuns := []struct {
		name      string
		intervals []malformedRunInterval64
	}{
		{name: "overlapping", intervals: []malformedRunInterval64{{start: 1, length: 2}, {start: 2, length: 2}}},
		{name: "unsorted", intervals: []malformedRunInterval64{{start: 4}, {start: 1}}},
		{name: "adjacent", intervals: []malformedRunInterval64{{start: 1, length: 1}, {start: 3, length: 1}}},
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
				if err := decoder.decode(bitmap, malformedRunBitmap64Bytes(malformed.intervals)); err != nil {
					t.Fatalf("decode malformed run bitmap: %v", err)
				}
				if err := bitmap.Validate(); err == nil {
					t.Fatal("malformed run bitmap unexpectedly validated")
				}
				selfWant := Or(bitmap, bitmap).GetCardinality()
				if got := bitmap.OrCardinality(bitmap); got != selfWant {
					t.Fatalf("run/run OrCardinality = %d, want materialized union cardinality %d", got, selfWant)
				}

				for _, peer := range malformedRun64Peers() {
					if err := peer.bitmap.Validate(); err != nil {
						t.Fatalf("invalid %s peer: %v", peer.name, err)
					}
					if peer.name == "bitmap" && (peer.bitmap.GetCardinality() <= 4096 || peer.bitmap.HasRunCompression()) {
						t.Fatal("bitmap peer did not produce a nested bitmap container")
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

type malformedRun64Peer struct {
	name   string
	bitmap *Bitmap
}

func malformedRun64Peers() []malformedRun64Peer {
	arrayOverlap := NewBitmap()
	arrayOverlap.Add(2)

	arrayDisjoint := NewBitmap()
	arrayDisjoint.Add(100)

	bitmap := NewBitmap()
	bitmap.Add(2)
	bitmap.AddRange(1000, 1000+4096)

	return []malformedRun64Peer{
		{name: "array-overlap", bitmap: arrayOverlap},
		{name: "array-disjoint", bitmap: arrayDisjoint},
		{name: "bitmap", bitmap: bitmap},
	}
}

func malformedRunBitmap64Bytes(intervals []malformedRunInterval64) []byte {
	// One outer key wrapping a malformed 32-bit run bitmap.
	data := make([]byte, 23+4*len(intervals))
	binary.LittleEndian.PutUint64(data[0:], 1)
	binary.LittleEndian.PutUint32(data[8:], 0)
	inner := data[12:]
	binary.LittleEndian.PutUint16(inner[0:], uint16(serialCookie))
	binary.LittleEndian.PutUint16(inner[2:], 0) // one container
	inner[4] = 1                                // run-container bitmap
	binary.LittleEndian.PutUint16(inner[5:], 0) // key

	cardinality := 0
	for _, interval := range intervals {
		cardinality += int(interval.length) + 1
	}
	binary.LittleEndian.PutUint16(inner[7:], uint16(cardinality-1))
	binary.LittleEndian.PutUint16(inner[9:], uint16(len(intervals)))
	for index, interval := range intervals {
		offset := 11 + 4*index
		binary.LittleEndian.PutUint16(inner[offset:], interval.start)
		binary.LittleEndian.PutUint16(inner[offset+2:], interval.length)
	}
	return data
}

func BenchmarkOrCardinality64Matrix(b *testing.B) {
	for _, fixture := range orCardinality64Fixtures() {
		fixture := fixture
		want := Or(fixture.left, fixture.right).GetCardinality()
		b.Run(fixture.name, func(b *testing.B) {
			b.ReportAllocs()
			var got uint64
			for b.Loop() {
				got = fixture.left.OrCardinality(fixture.right)
			}
			if got != want {
				b.Fatalf("OrCardinality = %d, want %d", got, want)
			}
			orCardinality64MatrixSink = got
		})
	}
}

func orCardinality64Fixtures() []orCardinality64Fixture {
	return []orCardinality64Fixture{
		newOrCardinality64OuterMixedFixture(),
		newOrCardinality64OuterSkewedFixture(),
	}
}

func newOrCardinality64OuterMixedFixture() orCardinality64Fixture {
	left := NewBitmap()
	right := NewBitmap()
	left.highlowcontainer.appendContainer(0, newOrCardinality64Inner("array", 50000), false)

	for index, pair := range orCardinality64NestedPairs() {
		key := uint32(index*2 + 1)
		left.highlowcontainer.appendContainer(key, newOrCardinality64Inner(pair.leftKind, pair.leftOffset), false)
		right.highlowcontainer.appendContainer(key, newOrCardinality64Inner(pair.rightKind, pair.rightOffset), false)
	}
	right.highlowcontainer.appendContainer(14, newOrCardinality64Inner("array", 50000), false)

	return orCardinality64Fixture{name: "outer-mixed", left: left, right: right}
}

func orCardinality64PairFixtures() []orCardinality64Fixture {
	pairs := orCardinality64NestedPairs()
	fixtures := make([]orCardinality64Fixture, 0, len(pairs))
	for _, pair := range pairs {
		left := NewBitmap()
		right := NewBitmap()
		left.highlowcontainer.appendContainer(7, newOrCardinality64Inner(pair.leftKind, pair.leftOffset), false)
		right.highlowcontainer.appendContainer(7, newOrCardinality64Inner(pair.rightKind, pair.rightOffset), false)
		fixtures = append(fixtures, orCardinality64Fixture{name: "nested-" + pair.name, left: left, right: right})
	}
	return fixtures
}

func orCardinality64NestedPairs() []orCardinality64NestedPair {
	return []orCardinality64NestedPair{
		{name: "array-array", leftKind: "array", rightKind: "array", leftOffset: 0, rightOffset: 256},
		{name: "array-bitmap", leftKind: "array", rightKind: "bitmap", leftOffset: 1000, rightOffset: 0},
		{name: "array-run", leftKind: "array", rightKind: "run", leftOffset: 0, rightOffset: 0},
		{name: "bitmap-bitmap", leftKind: "bitmap", rightKind: "bitmap", leftOffset: 0, rightOffset: 256},
		{name: "bitmap-run", leftKind: "bitmap", rightKind: "run", leftOffset: 0, rightOffset: 0},
		{name: "run-run", leftKind: "run", rightKind: "run", leftOffset: 0, rightOffset: 256},
	}
}

func newOrCardinality64OuterSkewedFixture() orCardinality64Fixture {
	left := NewBitmap()
	right := NewBitmap()
	matches := []struct {
		key         uint32
		leftKind    string
		rightKind   string
		leftOffset  uint32
		rightOffset uint32
	}{
		{key: 2, leftKind: "array", rightKind: "bitmap", leftOffset: 0, rightOffset: 0},
		{key: 14, leftKind: "bitmap", rightKind: "run", leftOffset: 0, rightOffset: 0},
		{key: 26, leftKind: "run", rightKind: "run", leftOffset: 0, rightOffset: 256},
	}

	for key := uint32(0); key < 32; key += 2 {
		kind := "array"
		offset := uint32(50000)
		for _, match := range matches {
			if match.key == key {
				kind = match.rightKind
				offset = match.rightOffset
				break
			}
		}
		right.highlowcontainer.appendContainer(key, newOrCardinality64Inner(kind, offset), false)
	}
	for _, match := range matches {
		left.highlowcontainer.appendContainer(match.key, newOrCardinality64Inner(match.leftKind, match.leftOffset), false)
	}

	return orCardinality64Fixture{name: "outer-skewed", left: left, right: right}
}

func newOrCardinality64Inner(kind string, offset uint32) *roaring.Bitmap {
	bitmap := roaring.NewBitmap()
	switch kind {
	case "array":
		for value := uint32(0); value < 512; value++ {
			bitmap.Add(offset + value*2)
		}
	case "bitmap":
		for value := uint32(0); value < 8192; value++ {
			bitmap.Add(offset + value*4)
		}
	case "run":
		for run := uint32(0); run < 8; run++ {
			start := offset + run*1024
			bitmap.AddRange(uint64(start), uint64(start+512))
		}
		bitmap.RunOptimize()
	default:
		panic("unknown container kind: " + kind)
	}
	return bitmap
}
