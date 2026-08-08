package common

import (
	"testing"
)

func TestRoundToPrecision(t *testing.T) {
	val1 := 0.123456789
	rounded1 := RoundToPrecision(val1, 8)
	if rounded1 != 0.12345679 {
		t.Errorf("Expected 0.12345679, got %f", rounded1)
	}

	val2 := 10.000000001
	rounded2 := RoundToPrecision(val2, 8)
	if rounded2 != 10.0 {
		t.Errorf("Expected 10.00000000, got %f", rounded2)
	}
}

func TestSafeArithmetic(t *testing.T) {
	a := 0.1
	b := 0.2
	sum := SafeAdd(a, b)
	if sum != 0.3 {
		t.Errorf("Expected exactly 0.3, got %f", sum)
	}

	sub := SafeSub(b, a)
	if sub != 0.1 {
		t.Errorf("Expected exactly 0.1, got %f", sub)
	}

	mul := SafeMul(a, b)
	if mul != 0.02 {
		t.Errorf("Expected exactly 0.02, got %f", mul)
	}
}
