package roaring

import "github.com/RoaringBitmap/roaring/v2"

func (b *BSI) parallelBatchEqualScan(parallelism int, vals []uint64) *roaring.Bitmap {
	return roaring.NewBitmap()
}
