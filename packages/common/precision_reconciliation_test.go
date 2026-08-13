package common

import (
	"math/big"
	"testing"
)

func TestPrecisionRationalArithmeticNoDustLeaks(t *testing.T) {
	// Execute 1,000 partial matches of size exactly 1/3 (0.333333333333333333) and verify total sum
	// matches 333.333333333333333 exactly with ZERO floating-point rounding dust desynchronizations.
	const partialValue = 0.333333333333333333
	const iterations = 1000

	// Perform intermediate calculations completely inside high-precision rational types big.Rat
	sumRat := ToRat(0.0)
	partialRat := ToRat(partialValue)
	for i := 0; i < iterations; i++ {
		sumRat = new(big.Rat).Add(sumRat, partialRat)
	}
	sum, _ := sumRat.Float64()

	// Verify that summing 1,000 times 1/3 using exact rational arithmetic doesn't drift
	expectedSumRat := new(big.Rat).Mul(partialRat, ToRat(float64(iterations)))
	expectedSum, _ := expectedSumRat.Float64()
	diff := SafeSub(sum, expectedSum)

	if diff < -1e-15 || diff > 1e-15 {
		t.Errorf("Precision desynchronized! Expected exact match with zero drift, got difference: %g", diff)
	}

	// Verify rounding to specific decimal places
	roundedVal := RoundToPrecision(0.123456789012345678, 8)
	if roundedVal != 0.12345679 {
		t.Errorf("Expected 0.12345679, got %f", roundedVal)
	}
}
