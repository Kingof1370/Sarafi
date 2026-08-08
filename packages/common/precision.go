package common

import (
	"math"
	"math/big"
)

// RoundToPrecision rounds a float64 to the specified number of decimal places (e.g. 8 for satoshi precision)
func RoundToPrecision(val float64, precision int) float64 {
	shift := math.Pow(10, float64(precision))
	return math.Round(val*shift) / shift
}

// SafeAdd performs a high-precision addition of two float64s using rational numbers
func SafeAdd(a, b float64) float64 {
	ra := new(big.Rat).SetFloat64(a)
	rb := new(big.Rat).SetFloat64(b)
	if ra == nil || rb == nil {
		return RoundToPrecision(a+b, 8)
	}
	res := new(big.Rat).Add(ra, rb)
	val, _ := res.Float64()
	return RoundToPrecision(val, 8)
}

// SafeSub performs a high-precision subtraction of two float64s using rational numbers
func SafeSub(a, b float64) float64 {
	ra := new(big.Rat).SetFloat64(a)
	rb := new(big.Rat).SetFloat64(b)
	if ra == nil || rb == nil {
		return RoundToPrecision(a-b, 8)
	}
	res := new(big.Rat).Sub(ra, rb)
	val, _ := res.Float64()
	return RoundToPrecision(val, 8)
}

// SafeMul performs a high-precision multiplication of two float64s using rational numbers
func SafeMul(a, b float64) float64 {
	ra := new(big.Rat).SetFloat64(a)
	rb := new(big.Rat).SetFloat64(b)
	if ra == nil || rb == nil {
		return RoundToPrecision(a*b, 8)
	}
	res := new(big.Rat).Mul(ra, rb)
	val, _ := res.Float64()
	return RoundToPrecision(val, 8)
}
