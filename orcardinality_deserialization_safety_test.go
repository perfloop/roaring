package roaring_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	roaring "github.com/RoaringBitmap/roaring/v2"
	"github.com/RoaringBitmap/roaring/v2/roaring64"
)

const (
	orCardinalitySafetyNoRunCookie = 12346
	orCardinalitySafetyRunCookie   = 12347
	orCardinalitySafetyMaxArray    = 4096
	orCardinalitySafetyMaxCapacity = 1 << 16
)

type orCardinalitySafetyInterval struct {
	start  uint16
	length uint16
}

type orCardinalitySafetyCase struct {
	name                  string
	left, right           []byte
	leftValid, rightValid bool
}

func TestOrCardinalityDeserializationSafety(t *testing.T) {
	for _, test := range orCardinalityDeserializationSafetyCases() {
		test := test
		for _, decoder := range orCardinalitySafetyDecoders32() {
			decoder := decoder
			t.Run(test.name+"/"+decoder.name, func(t *testing.T) {
				left := roaring.NewBitmap()
				if err := decoder.decode(left, test.left); err != nil {
					t.Fatalf("decode left bitmap: %v", err)
				}
				right := roaring.NewBitmap()
				if err := decoder.decode(right, test.right); err != nil {
					t.Fatalf("decode right bitmap: %v", err)
				}
				orCardinalitySafetyCheckValid(t, "left", left.Validate(), test.leftValid)
				orCardinalitySafetyCheckValid(t, "right", right.Validate(), test.rightValid)
				orCardinalitySafetyCheck32(t, left, right)
			})
		}
	}
}

func TestOrCardinalityDeserializationSafety64(t *testing.T) {
	for _, test := range orCardinalityDeserializationSafetyCases() {
		test := test
		for _, decoder := range orCardinalitySafetyDecoders64() {
			decoder := decoder
			t.Run(test.name+"/"+decoder.name, func(t *testing.T) {
				left := roaring64.NewBitmap()
				if err := decoder.decode(left, orCardinalitySafetyBitmap64Bytes(test.left)); err != nil {
					t.Fatalf("decode left bitmap: %v", err)
				}
				right := roaring64.NewBitmap()
				if err := decoder.decode(right, orCardinalitySafetyBitmap64Bytes(test.right)); err != nil {
					t.Fatalf("decode right bitmap: %v", err)
				}
				orCardinalitySafetyCheckValid(t, "left", left.Validate(), test.leftValid)
				orCardinalitySafetyCheckValid(t, "right", right.Validate(), test.rightValid)
				orCardinalitySafetyCheck64(t, left, right)
			})
		}
	}
}

type orCardinalitySafetyDecoder32 struct {
	name   string
	decode func(*roaring.Bitmap, []byte) error
}

func orCardinalitySafetyDecoders32() []orCardinalitySafetyDecoder32 {
	return []orCardinalitySafetyDecoder32{
		{
			name: "UnmarshalBinary",
			decode: func(bitmap *roaring.Bitmap, data []byte) error {
				return bitmap.UnmarshalBinary(data)
			},
		},
		{
			name: "ReadFrom",
			decode: func(bitmap *roaring.Bitmap, data []byte) error {
				_, err := bitmap.ReadFrom(bytes.NewReader(data))
				return err
			},
		},
	}
}

type orCardinalitySafetyDecoder64 struct {
	name   string
	decode func(*roaring64.Bitmap, []byte) error
}

func orCardinalitySafetyDecoders64() []orCardinalitySafetyDecoder64 {
	return []orCardinalitySafetyDecoder64{
		{
			name: "UnmarshalBinary",
			decode: func(bitmap *roaring64.Bitmap, data []byte) error {
				return bitmap.UnmarshalBinary(data)
			},
		},
		{
			name: "ReadFrom",
			decode: func(bitmap *roaring64.Bitmap, data []byte) error {
				_, err := bitmap.ReadFrom(bytes.NewReader(data))
				return err
			},
		},
	}
}

func orCardinalitySafetyCheckValid(t *testing.T, side string, err error, wantValid bool) {
	t.Helper()
	if (err == nil) != wantValid {
		t.Fatalf("%s Validate() error = %v, want valid = %t", side, err, wantValid)
	}
}

func orCardinalitySafetyCheck32(t *testing.T, left, right *roaring.Bitmap) {
	t.Helper()
	if want, got := roaring.Or(left, right).GetCardinality(), left.OrCardinality(right); got != want {
		t.Fatalf("forward OrCardinality = %d, want materialized union cardinality %d", got, want)
	}
	if want, got := roaring.Or(right, left).GetCardinality(), right.OrCardinality(left); got != want {
		t.Fatalf("reverse OrCardinality = %d, want materialized union cardinality %d", got, want)
	}
}

func orCardinalitySafetyCheck64(t *testing.T, left, right *roaring64.Bitmap) {
	t.Helper()
	if want, got := roaring64.Or(left, right).GetCardinality(), left.OrCardinality(right); got != want {
		t.Fatalf("forward OrCardinality = %d, want materialized union cardinality %d", got, want)
	}
	if want, got := roaring64.Or(right, left).GetCardinality(), right.OrCardinality(left); got != want {
		t.Fatalf("reverse OrCardinality = %d, want materialized union cardinality %d", got, want)
	}
}

func orCardinalityDeserializationSafetyCases() []orCardinalitySafetyCase {
	canonicalRun := orCardinalitySafetyRunBytes([]orCardinalitySafetyInterval{{start: 0, length: 4095}})
	bitmapPeer := orCardinalitySafetyBitmapBytes(4097, orCardinalitySafetyBitmapPeerValues())

	cases := []orCardinalitySafetyCase{
		{
			name:       "run-unsorted-duplicate-array",
			left:       canonicalRun,
			right:      orCardinalitySafetyArrayBytes([]uint16{5000, 4097, 4097}),
			leftValid:  true,
			rightValid: false,
		},
		{
			name:       "run-stale-bitmap-cardinality",
			left:       canonicalRun,
			right:      orCardinalitySafetyBitmapBytes(5000, []uint16{2}),
			leftValid:  true,
			rightValid: false,
		},
		{
			name:       "descending-array-array",
			left:       orCardinalitySafetyArrayBytes(orCardinalitySafetyDescendingValues(orCardinalitySafetyMaxArray)),
			right:      orCardinalitySafetyArrayBytes([]uint16{0}),
			leftValid:  false,
			rightValid: true,
		},
		{
			name:       "duplicate-array-array",
			left:       orCardinalitySafetyArrayBytes(orCardinalitySafetyRepeatedValues(1, orCardinalitySafetyMaxArray)),
			right:      orCardinalitySafetyArrayBytes(orCardinalitySafetyRepeatedValues(2, orCardinalitySafetyMaxArray)),
			leftValid:  true,
			rightValid: true,
		},
	}

	for _, malformed := range []struct {
		name      string
		intervals []orCardinalitySafetyInterval
	}{
		{name: "overlapping-run", intervals: []orCardinalitySafetyInterval{{start: 1, length: 2}, {start: 2, length: 2}}},
		{name: "unsorted-run", intervals: []orCardinalitySafetyInterval{{start: 4}, {start: 1}}},
		{name: "adjacent-run", intervals: []orCardinalitySafetyInterval{{start: 1, length: 1}, {start: 3, length: 1}}},
		{name: "empty-run", intervals: nil},
		{name: "wrapping-run", intervals: []orCardinalitySafetyInterval{{start: 65535, length: 1}}},
	} {
		malformedRun := orCardinalitySafetyRunBytes(malformed.intervals)
		cases = append(cases,
			orCardinalitySafetyCase{
				name:       malformed.name + "-array",
				left:       malformedRun,
				right:      orCardinalitySafetyArrayBytes([]uint16{2}),
				leftValid:  false,
				rightValid: true,
			},
			orCardinalitySafetyCase{
				name:       malformed.name + "-bitmap",
				left:       malformedRun,
				right:      bitmapPeer,
				leftValid:  false,
				rightValid: true,
			},
		)
	}

	return cases
}

func orCardinalitySafetyArrayBytes(values []uint16) []byte {
	if len(values) == 0 || len(values) > orCardinalitySafetyMaxArray {
		panic("invalid array fixture cardinality")
	}

	data := make([]byte, 16+2*len(values))
	binary.LittleEndian.PutUint32(data[0:], orCardinalitySafetyNoRunCookie)
	binary.LittleEndian.PutUint32(data[4:], 1)
	binary.LittleEndian.PutUint16(data[8:], 0)
	binary.LittleEndian.PutUint16(data[10:], uint16(len(values)-1))
	binary.LittleEndian.PutUint32(data[12:], 16)
	for index, value := range values {
		binary.LittleEndian.PutUint16(data[16+2*index:], value)
	}
	return data
}

func orCardinalitySafetyBitmapBytes(cardinality int, values []uint16) []byte {
	if cardinality <= orCardinalitySafetyMaxArray || cardinality > orCardinalitySafetyMaxCapacity {
		panic("invalid bitmap fixture cardinality")
	}

	data := make([]byte, 16+orCardinalitySafetyMaxCapacity/8)
	binary.LittleEndian.PutUint32(data[0:], orCardinalitySafetyNoRunCookie)
	binary.LittleEndian.PutUint32(data[4:], 1)
	binary.LittleEndian.PutUint16(data[8:], 0)
	binary.LittleEndian.PutUint16(data[10:], uint16(cardinality-1))
	binary.LittleEndian.PutUint32(data[12:], 16)
	for _, value := range values {
		offset := 16 + int(value/64)*8
		word := binary.LittleEndian.Uint64(data[offset:])
		binary.LittleEndian.PutUint64(data[offset:], word|(uint64(1)<<uint(value%64)))
	}
	return data
}

func orCardinalitySafetyRunBytes(intervals []orCardinalitySafetyInterval) []byte {
	data := make([]byte, 11+4*len(intervals))
	binary.LittleEndian.PutUint16(data[0:], orCardinalitySafetyRunCookie)
	binary.LittleEndian.PutUint16(data[2:], 0)
	data[4] = 1
	binary.LittleEndian.PutUint16(data[5:], 0)

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

func orCardinalitySafetyBitmap64Bytes(inner []byte) []byte {
	data := make([]byte, 12+len(inner))
	binary.LittleEndian.PutUint64(data[0:], 1)
	binary.LittleEndian.PutUint32(data[8:], 0)
	copy(data[12:], inner)
	return data
}

func orCardinalitySafetyBitmapPeerValues() []uint16 {
	values := make([]uint16, 0, 4097)
	values = append(values, 2)
	for value := 1000; len(values) < 4097; value++ {
		values = append(values, uint16(value))
	}
	return values
}

func orCardinalitySafetyDescendingValues(length int) []uint16 {
	values := make([]uint16, length)
	for index := range values {
		values[index] = uint16(length - index - 1)
	}
	return values
}

func orCardinalitySafetyRepeatedValues(value uint16, length int) []uint16 {
	values := make([]uint16, length)
	for index := range values {
		values[index] = value
	}
	return values
}
