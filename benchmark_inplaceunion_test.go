package roaring

import "testing"

func BenchmarkRunContainerInplaceUnion(b *testing.B) {
	BenchmarkRunContainerInplaceUnion_FewLargeRuns(b)
}

func BenchmarkRunContainerInplaceUnion_FewLargeRuns(b *testing.B) {
	iv1 := []interval16{
		newInterval16Range(100, 5000),
		newInterval16Range(10000, 15000),
		newInterval16Range(20000, 25000),
		newInterval16Range(30000, 35000),
		newInterval16Range(40000, 45000),
	}
	iv2 := []interval16{
		newInterval16Range(50, 4000),
		newInterval16Range(12000, 16000),
		newInterval16Range(22000, 26000),
		newInterval16Range(32000, 36000),
		newInterval16Range(42000, 46000),
	}
	rc1 := &runContainer16{iv: iv1}
	rc2 := &runContainer16{iv: iv2}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc := rc1.Clone()
		_ = rc.inplaceUnion(rc2)
	}
}

func BenchmarkRunContainerInplaceUnion_Sparse(b *testing.B) {
	iv1 := []interval16{
		newInterval16Range(100, 500),
		newInterval16Range(1000, 1500),
		newInterval16Range(2000, 2500),
		newInterval16Range(3000, 3500),
		newInterval16Range(4000, 4500),
		newInterval16Range(5000, 5500),
		newInterval16Range(6000, 6500),
		newInterval16Range(7000, 7500),
		newInterval16Range(8000, 8500),
		newInterval16Range(9000, 9500),
	}
	iv2 := []interval16{
		newInterval16Range(4250, 4250),
	}
	rc1 := &runContainer16{iv: iv1}
	rc2 := &runContainer16{iv: iv2}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rc := rc1.Clone()
		_ = rc.inplaceUnion(rc2)
	}
}
