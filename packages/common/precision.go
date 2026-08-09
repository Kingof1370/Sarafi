package common

import (
	"math"
	"math/big"
	"strconv"
)

// ToRat converts a float64 to big.Rat exactly by using its shortest decimal string representation
func ToRat(f float64) *big.Rat {
	s := strconv.FormatFloat(f, 'f', -1, 64)
	r := new(big.Rat)
	if _, ok := r.SetString(s); ok {
		return r
	}
	return new(big.Rat).SetFloat64(f) // fallback
}

// SafeAdd precise addition
func SafeAdd(a, b float64) float64 {
	rA := ToRat(a)
	rB := ToRat(b)
	res := new(big.Rat).Add(rA, rB)
	val, _ := res.Float64()
	return val
}

// SafeSub precise subtraction
func SafeSub(a, b float64) float64 {
	rA := ToRat(a)
	rB := ToRat(b)
	res := new(big.Rat).Sub(rA, rB)
	val, _ := res.Float64()
	return val
}

// SafeMul precise multiplication
func SafeMul(a, b float64) float64 {
	rA := ToRat(a)
	rB := ToRat(b)
	res := new(big.Rat).Mul(rA, rB)
	val, _ := res.Float64()
	return val
}

// SafeDiv precise division
func SafeDiv(a, b float64) float64 {
	if b == 0 {
		return 0
	}
	rA := ToRat(a)
	rB := ToRat(b)
	res := new(big.Rat).Quo(rA, rB)
	val, _ := res.Float64()
	return val
}

// RoundToPrecision precise rounding
func RoundToPrecision(val float64, precision int) float64 {
	if precision < 0 {
		precision = 0
	}
	shift := math.Pow(10, float64(precision))
	shifted := SafeMul(val, shift)
	rounded := math.Round(shifted)
	return SafeDiv(rounded, shift)
}

// EnforceTickSize price alignment
func EnforceTickSize(price, tickSize float64) float64 {
	if tickSize <= 0 {
		return price
	}
	ticks := math.Round(price / tickSize)
	return SafeMul(ticks, tickSize)
}

// EnforceLotSize quantity alignment
func EnforceLotSize(quantity, stepSize float64) float64 {
	if stepSize <= 0 {
		return quantity
	}
	lots := math.Round(quantity / stepSize)
	return SafeMul(lots, stepSize)
}
