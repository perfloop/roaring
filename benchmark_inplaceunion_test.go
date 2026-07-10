package roaring

import "testing"

func BenchmarkRunContainerInplaceUnion(b *testing.B) {
	// We sweep multiple regimes of inplaceUnion:
	// 1. Large dense overlapping intervals (demanding reallocation)
	// 2. Large dense overlapping intervals (utilizing existing capacity in-place)
	// 3. Sparse update (cardinality <= 16) where direct element-by-element addition is preferred
	// 4. Completely disjoint and adjacent intervals
	type testCase struct {
		name   string
		iv1    []interval16
		iv2    []interval16
		preCap bool // If true, pre-allocate cap(iv) >= len(iv1) + len(iv2)
	}

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

	cases := []testCase{
		{
			name:   "DenseRealloc",
			iv1:    iv1Dense,
			iv2:    iv2Dense,
			preCap: false,
		},
		{
			name:   "DenseInPlaceCap",
			iv1:    iv1Dense,
			iv2:    iv2Dense,
			preCap: true,
		},
		{
			name: "SparseAdd",
			iv1:  iv1Dense,
			iv2: []interval16{
				newInterval16Range(50, 50), // Cardinality 1 (<= 16)
			},
			preCap: true,
		},
		{
			name: "DisjointAdjacent",
			iv1: []interval16{
				newInterval16Range(100, 200),
				newInterval16Range(300, 400),
			},
			iv2: []interval16{
				newInterval16Range(201, 299), // Adjacent
				newInterval16Range(500, 600), // Disjoint
			},
			preCap: true,
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, tc := range cases {
			var rc *runContainer16
			if tc.preCap {
				rc = &runContainer16{
					iv: make([]interval16, len(tc.iv1), len(tc.iv1)+len(tc.iv2)),
				}
				copy(rc.iv, tc.iv1)
			} else {
				rc = &runContainer16{
					iv: append([]interval16(nil), tc.iv1...),
				}
			}
			rc2 := &runContainer16{iv: tc.iv2}
			_ = rc.inplaceUnion(rc2)
		}
	}
}
