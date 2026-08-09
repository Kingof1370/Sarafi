package common

import (
	"testing"
)

func TestFinancialPrecision(t *testing.T) {
	// Standard float64 precision test (0.1 + 0.2)
	standardFloatAdd := 0.1 + 0.2
	if standardFloatAdd == 0.3 {
		t.Log("Standard float64 addition is unexpectedly exact")
	} else {
		t.Logf("Standard float64 addition drift detected: %f", standardFloatAdd)
	}

	// SafeAdd test
	safeAddVal := SafeAdd(0.1, 0.2)
	if safeAddVal != 0.3 {
		t.Errorf("Expected SafeAdd(0.1, 0.2) to be 0.3, got %f", safeAddVal)
	}

	// SafeSub test
	safeSubVal := SafeSub(0.3, 0.2)
	if safeSubVal != 0.1 {
		t.Errorf("Expected SafeSub(0.3, 0.2) to be 0.1, got %f", safeSubVal)
	}

	// SafeMul test
	safeMulVal := SafeMul(0.1, 0.2)
	if safeMulVal != 0.02 {
		t.Errorf("Expected SafeMul(0.1, 0.2) to be 0.02, got %f", safeMulVal)
	}

	// SafeDiv test
	safeDivVal := SafeDiv(0.3, 3)
	if safeDivVal != 0.1 {
		t.Errorf("Expected SafeDiv(0.3, 3) to be 0.1, got %f", safeDivVal)
	}

	// RoundToPrecision test
	roundVal1 := RoundToPrecision(1.234567, 4)
	if roundVal1 != 1.2346 {
		t.Errorf("Expected RoundToPrecision(1.234567, 4) to be 1.2346, got %f", roundVal1)
	}

	roundVal2 := RoundToPrecision(1.234567, 2)
	if roundVal2 != 1.23 {
		t.Errorf("Expected RoundToPrecision(1.234567, 2) to be 1.23, got %f", roundVal2)
	}

	// EnforceTickSize test
	tickVal := EnforceTickSize(50000.005, 0.01)
	if tickVal != 50000.01 {
		t.Errorf("Expected EnforceTickSize(50000.005, 0.01) to be 50000.01, got %f", tickVal)
	}

	// EnforceLotSize test
	lotVal := EnforceLotSize(1.5555, 0.1)
	if lotVal != 1.6 {
		t.Errorf("Expected EnforceLotSize(1.5555, 0.1) to be 1.6, got %f", lotVal)
	}
}
