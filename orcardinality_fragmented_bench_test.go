package roaring

import "testing"

var orCardinalityFragmentedSink uint64

type fragmentedOrCardinalityFixture struct {
	name  string
	left  *Bitmap
	right *Bitmap
}

func BenchmarkOrCardinalityFragmentedRuns(b *testing.B) {
	for _, fixture := range fragmentedOrCardinalityFixtures() {
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
			orCardinalityFragmentedSink = got
		})
	}
}

func fragmentedOrCardinalityFixtures() []fragmentedOrCardinalityFixture {
	array := fragmentedArrayContainer()
	bitmap := fragmentedBitmapContainer()
	runLeft := fragmentedRunContainer(0)
	runRight := fragmentedRunContainer(16)

	return []fragmentedOrCardinalityFixture{
		{name: "array-run-forward", left: bitmapWithContainer(array), right: bitmapWithContainer(runLeft)},
		{name: "run-array-reverse", left: bitmapWithContainer(runLeft), right: bitmapWithContainer(array)},
		{name: "bitmap-run-forward", left: bitmapWithContainer(bitmap), right: bitmapWithContainer(runLeft)},
		{name: "run-bitmap-reverse", left: bitmapWithContainer(runLeft), right: bitmapWithContainer(bitmap)},
		{name: "run-run", left: bitmapWithContainer(runLeft), right: bitmapWithContainer(runRight)},
	}
}

func bitmapWithContainer(value container) *Bitmap {
	bitmap := NewBitmap()
	bitmap.highlowcontainer.appendContainer(0, value, false)
	return bitmap
}

func fragmentedArrayContainer() *arrayContainer {
	values := make([]uint16, arrayDefaultMaxSize)
	for index := range values {
		values[index] = uint16(index * 16)
	}
	return &arrayContainer{content: values}
}

func fragmentedBitmapContainer() *bitmapContainer {
	bitmap := newBitmapContainer()
	for value := 0; value < maxCapacity; value += 16 {
		bitmap.iadd(uint16(value))
	}
	return bitmap
}

func fragmentedRunContainer(offset int) *runContainer16 {
	intervals := make([]interval16, 1024)
	for index := range intervals {
		start := uint16(index*64 + offset)
		intervals[index] = newInterval16Range(start, start+2)
	}
	return newRunContainer16TakeOwnership(intervals)
}
