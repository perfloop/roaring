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

func TestOrCardinalityMalformedRunDeserialization64(t *testing.T) {
	data := malformedOverlappingRunBitmap64Bytes()
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

	for _, decoder := range decoders {
		decoder := decoder
		t.Run(decoder.name, func(t *testing.T) {
			bitmap := NewBitmap()
			if err := decoder.decode(bitmap, data); err != nil {
				t.Fatalf("decode malformed run bitmap: %v", err)
			}
			if err := bitmap.Validate(); err == nil {
				t.Fatal("overlapping run bitmap unexpectedly validated")
			}

			want := Or(bitmap, bitmap).GetCardinality()
			if want != 4 {
				t.Fatalf("materialized union cardinality = %d, want 4", want)
			}
			if got := bitmap.OrCardinality(bitmap); got != want {
				t.Fatalf("OrCardinality = %d, want materialized union cardinality %d", got, want)
			}
		})
	}
}

func malformedOverlappingRunBitmap64Bytes() []byte {
	// One outer key wrapping the same malformed 32-bit run bitmap used above.
	data := make([]byte, 31)
	binary.LittleEndian.PutUint64(data[0:], 1)
	binary.LittleEndian.PutUint32(data[8:], 0)
	inner := data[12:]
	binary.LittleEndian.PutUint16(inner[0:], uint16(serialCookie))
	binary.LittleEndian.PutUint16(inner[2:], 0) // one container
	inner[4] = 1                                // run-container bitmap
	binary.LittleEndian.PutUint16(inner[5:], 0) // key
	binary.LittleEndian.PutUint16(inner[7:], 3) // cardinality minus one
	binary.LittleEndian.PutUint16(inner[9:], 2) // interval count
	binary.LittleEndian.PutUint16(inner[11:], 1)
	binary.LittleEndian.PutUint16(inner[13:], 2) // [1,3]
	binary.LittleEndian.PutUint16(inner[15:], 2)
	binary.LittleEndian.PutUint16(inner[17:], 2) // [2,4]
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
