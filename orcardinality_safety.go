package roaring

func (ac *arrayContainer) isCardinalitySafe() bool {
	if len(ac.content) == 0 || len(ac.content) > arrayDefaultMaxSize {
		return false
	}
	for index := 1; index < len(ac.content); index++ {
		if ac.content[index-1] >= ac.content[index] {
			return false
		}
	}
	return true
}

func (bc *bitmapContainer) isCardinalitySafe() bool {
	return bc.cardinality >= arrayDefaultMaxSize &&
		bc.cardinality <= maxCapacity &&
		len(bc.bitmap) == maxCapacity/64 &&
		bc.cardinality == int(popcntSlice(bc.bitmap))
}
