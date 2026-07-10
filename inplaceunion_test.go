package roaring

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRunContainer16InplaceUnionDirect(t *testing.T) {
	testCases := []struct {
		name   string
		iv1    []interval16
		iv2    []interval16
		wantIv []interval16
		want   []uint16
	}{
		{
			name:   "Disjoint",
			iv1:    []interval16{newInterval16Range(10, 20), newInterval16Range(50, 60)},
			iv2:    []interval16{newInterval16Range(30, 40)},
			wantIv: []interval16{newInterval16Range(10, 20), newInterval16Range(30, 40), newInterval16Range(50, 60)},
			want:   append(append(makeRange16(10, 20), makeRange16(30, 40)...), makeRange16(50, 60)...),
		},
		{
			name:   "Overlapping",
			iv1:    []interval16{newInterval16Range(10, 20)},
			iv2:    []interval16{newInterval16Range(15, 30)},
			wantIv: []interval16{newInterval16Range(10, 30)},
			want:   makeRange16(10, 30),
		},
		{
			name:   "Nested",
			iv1:    []interval16{newInterval16Range(10, 50)},
			iv2:    []interval16{newInterval16Range(20, 30)},
			wantIv: []interval16{newInterval16Range(10, 50)},
			want:   makeRange16(10, 50),
		},
		{
			name:   "Adjacent",
			iv1:    []interval16{newInterval16Range(10, 20)},
			iv2:    []interval16{newInterval16Range(21, 30)},
			wantIv: []interval16{newInterval16Range(10, 30)},
			want:   makeRange16(10, 30),
		},
		{
			name:   "EmptyFirst",
			iv1:    []interval16{},
			iv2:    []interval16{newInterval16Range(10, 20)},
			wantIv: []interval16{newInterval16Range(10, 20)},
			want:   makeRange16(10, 20),
		},
		{
			name:   "EmptySecond",
			iv1:    []interval16{newInterval16Range(10, 20)},
			iv2:    []interval16{},
			wantIv: []interval16{newInterval16Range(10, 20)},
			want:   makeRange16(10, 20),
		},
	}

	for _, tc := range testCases {
		for _, useCap := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_cap=%v", tc.name, useCap), func(t *testing.T) {
				var rc1 *runContainer16
				if useCap {
					rc1 = &runContainer16{
						iv: make([]interval16, len(tc.iv1), len(tc.iv1)+len(tc.iv2)),
					}
					copy(rc1.iv, tc.iv1)
				} else {
					rc1 = &runContainer16{
						iv: append([]interval16(nil), tc.iv1...),
					}
				}
				rc2 := &runContainer16{
					iv: append([]interval16(nil), tc.iv2...),
				}

				res := rc1.inplaceUnion(rc2)
				assert.Equal(t, tc.want, containerToSlice(res))

				resRc, ok := res.(*runContainer16)
				if ok {
					assert.Equal(t, tc.wantIv, resRc.iv)
					for i := range resRc.iv {
						assert.True(t, resRc.iv[i].start <= resRc.iv[i].last())
						if i > 0 {
							assert.True(t, int(resRc.iv[i-1].last())+1 < int(resRc.iv[i].start), "Merged intervals must be sorted and non-contiguous")
						}
					}
				}
			})
		}
	}
}

func TestRunContainer16InplaceUnionAdversarial(t *testing.T) {
	rc1 := &runContainer16{
		iv: []interval16{newInterval16Range(5, 5)},
	}
	rc2 := &runContainer16{
		iv: []interval16{newInterval16Range(30, 30), newInterval16Range(10, 10)},
	}

	res := rc1.inplaceUnion(rc2)
	want := []uint16{5, 10, 30}
	assert.Equal(t, want, containerToSlice(res))

	resRc, ok := res.(*runContainer16)
	if ok {
		assert.Equal(t, 3, len(resRc.iv))
		assert.Equal(t, uint16(5), resRc.iv[0].start)
		assert.Equal(t, uint16(10), resRc.iv[1].start)
		assert.Equal(t, uint16(30), resRc.iv[2].start)
	}
}

func TestRunContainer16InplaceUnionAdversarialLarge(t *testing.T) {
	rc1 := &runContainer16{
		iv: []interval16{newInterval16Range(5, 5)},
	}
	rc2 := &runContainer16{
		iv: []interval16{
			newInterval16Range(50, 50),
			newInterval16Range(10, 10),
			newInterval16Range(100, 115),
		},
	}

	res := rc1.inplaceUnion(rc2)
	want := makeRange16(5, 5)
	want = append(want, makeRange16(10, 10)...)
	want = append(want, makeRange16(50, 50)...)
	want = append(want, makeRange16(100, 115)...)

	assert.Equal(t, want, containerToSlice(res))

	resRc, ok := res.(*runContainer16)
	if ok {
		assert.Equal(t, 4, len(resRc.iv))
		assert.Equal(t, uint16(5), resRc.iv[0].start)
		assert.Equal(t, uint16(10), resRc.iv[1].start)
		assert.Equal(t, uint16(50), resRc.iv[2].start)
		assert.Equal(t, uint16(100), resRc.iv[3].start)
	}
}

func TestRunContainer16InplaceUnionAdversarialWrapped(t *testing.T) {
	rc1 := &runContainer16{
		iv: []interval16{newInterval16Range(5, 5)},
	}
	// Construct a wrapped interval16 directly:
	// start = 65000, length = 1000 => last() = 464, which is < start.
	wrappedIv := interval16{start: 65000, length: 1000}
	rc2 := &runContainer16{
		iv: []interval16{
			wrappedIv,
			newInterval16Range(500, 510),
		},
	}

	res := rc1.inplaceUnion(rc2)
	resRc, ok := res.(*runContainer16)
	if ok {
		for i := range resRc.iv {
			assert.True(t, resRc.iv[i].start <= resRc.iv[i].last())
			if i > 0 {
				assert.True(t, int(resRc.iv[i-1].last())+1 < int(resRc.iv[i].start), "Merged intervals must be sorted and non-contiguous")
			}
		}
	}
}

func containerToSlice(c container) []uint16 {
	it := c.getShortIterator()
	var res []uint16
	for it.hasNext() {
		res = append(res, it.next())
	}
	return res
}

func makeRange16(start, end uint16) []uint16 {
	res := make([]uint16, 0, end-start+1)
	for i := start; i <= end; i++ {
		res = append(res, i)
	}
	return res
}

func BenchmarkRunContainerInplaceUnion(b *testing.B) {
	iv1Dense := []interval16{
		newInterval16Range(100, 5000),
		newInterval16Range(10000, 15000),
		newInterval16Range(20000, 25000),
		newInterval16Range(30000, 35000),
		newInterval16Range(40000, 45000),
	}
	iv2Dense := []interval16{
		newInterval16Range(50, 4000),
		newInterval16Range(12000, 16000),
		newInterval16Range(22000, 26000),
		newInterval16Range(32000, 36000),
		newInterval16Range(42000, 46000),
	}

	b.Run("DenseRealloc", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rc := &runContainer16{
				iv: append([]interval16(nil), iv1Dense...),
			}
			rc2 := &runContainer16{iv: iv2Dense}
			_ = rc.inplaceUnion(rc2)
		}
	})

	b.Run("DenseInPlaceCap", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rc := &runContainer16{
				iv: make([]interval16, len(iv1Dense), len(iv1Dense)+len(iv2Dense)),
			}
			copy(rc.iv, iv1Dense)
			rc2 := &runContainer16{iv: iv2Dense}
			_ = rc.inplaceUnion(rc2)
		}
	})

	b.Run("SparseAdd", func(b *testing.B) {
		iv2Sparse := []interval16{
			newInterval16Range(50, 50),
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rc := &runContainer16{
				iv: make([]interval16, len(iv1Dense), len(iv1Dense)+len(iv2Sparse)),
			}
			copy(rc.iv, iv1Dense)
			rc2 := &runContainer16{iv: iv2Sparse}
			_ = rc.inplaceUnion(rc2)
		}
	})

	b.Run("SparseAdd_Card15", func(b *testing.B) {
		iv2Sparse := []interval16{
			newInterval16Range(50, 64),
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rc := &runContainer16{
				iv: make([]interval16, len(iv1Dense), len(iv1Dense)+len(iv2Sparse)),
			}
			copy(rc.iv, iv1Dense)
			rc2 := &runContainer16{iv: iv2Sparse}
			_ = rc.inplaceUnion(rc2)
		}
	})

	b.Run("SparseAdd_Card17", func(b *testing.B) {
		iv2Sparse := []interval16{
			newInterval16Range(50, 66),
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rc := &runContainer16{
				iv: make([]interval16, len(iv1Dense), len(iv1Dense)+len(iv2Sparse)),
			}
			copy(rc.iv, iv1Dense)
			rc2 := &runContainer16{iv: iv2Sparse}
			_ = rc.inplaceUnion(rc2)
		}
	})

	b.Run("DisjointAdjacent", func(b *testing.B) {
		iv1 := []interval16{
			newInterval16Range(100, 200),
			newInterval16Range(300, 400),
		}
		iv2 := []interval16{
			newInterval16Range(201, 299),
			newInterval16Range(500, 600),
		}
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			rc := &runContainer16{
				iv: make([]interval16, len(iv1), len(iv1)+len(iv2)),
			}
			copy(rc.iv, iv1)
			rc2 := &runContainer16{iv: iv2}
			_ = rc.inplaceUnion(rc2)
		}
	})
}
