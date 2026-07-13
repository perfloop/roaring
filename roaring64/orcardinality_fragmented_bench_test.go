package roaring64

import (
	"testing"

	"github.com/RoaringBitmap/roaring/v2"
)

var orCardinality64FragmentedSink uint64

type fragmentedOrCardinality64Fixture struct {
	name  string
	left  *Bitmap
	right *Bitmap
}

func BenchmarkOrCardinality64FragmentedRuns(b *testing.B) {
	for _, fixture := range fragmentedOrCardinality64Fixtures() {
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
			orCardinality64FragmentedSink = got
		})
	}
}

func fragmentedOrCardinality64Fixtures() []fragmentedOrCardinality64Fixture {
	array := fragmentedRoaringArray()
	bitmap := fragmentedRoaringBitmap()
	runLeft := fragmentedRoaringRun(0)
	runRight := fragmentedRoaringRun(16)
	if array.GetCardinality() != 4096 || array.HasRunCompression() || bitmap.GetCardinality() != 4097 || bitmap.HasRunCompression() {
		panic("fragmented fixture did not preserve array and bitmap containers")
	}

	return []fragmentedOrCardinality64Fixture{
		{name: "array-run-forward", left: bitmapWithNestedContainer(array), right: bitmapWithNestedContainer(runLeft)},
		{name: "run-array-reverse", left: bitmapWithNestedContainer(runLeft), right: bitmapWithNestedContainer(array)},
		{name: "bitmap-run-forward", left: bitmapWithNestedContainer(bitmap), right: bitmapWithNestedContainer(runLeft)},
		{name: "run-bitmap-reverse", left: bitmapWithNestedContainer(runLeft), right: bitmapWithNestedContainer(bitmap)},
		{name: "run-run", left: bitmapWithNestedContainer(runLeft), right: bitmapWithNestedContainer(runRight)},
	}
}

func bitmapWithNestedContainer(value *roaring.Bitmap) *Bitmap {
	bitmap := NewBitmap()
	bitmap.highlowcontainer.appendContainer(0, value, false)
	return bitmap
}

func fragmentedRoaringArray() *roaring.Bitmap {
	bitmap := roaring.NewBitmap()
	for index := 0; index < 4096; index++ {
		bitmap.Add(uint32(index * 16))
	}
	return bitmap
}

func fragmentedRoaringBitmap() *roaring.Bitmap {
	bitmap := roaring.NewBitmap()
	for index := 0; index <= 4096; index++ {
		bitmap.Add(uint32(index * 15))
	}
	return bitmap
}

func fragmentedRoaringRun(offset int) *roaring.Bitmap {
	bitmap := roaring.NewBitmap()
	for index := 0; index < 1024; index++ {
		start := uint64(index*64 + offset)
		bitmap.AddRange(start, start+3)
	}
	bitmap.RunOptimize()
	if !bitmap.HasRunCompression() {
		panic("fragmented run fixture did not produce a run container")
	}
	return bitmap
}
