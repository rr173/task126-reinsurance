package domain

import "math"

// Money is an amount expressed in integer minor units (cents). All monetary
// fields in the engine use int64 cents to avoid floating-point drift. Ratios
// and factors stay as float64 during computation and are rounded to the nearest
// cent when persisted.
type Money int64

// RoundCents rounds a float64 amount expressed in minor units to the nearest
// integer cent using round-half-away-from-zero (matches the roundDiv helper
// used for proportional allocations).
func RoundCents(v float64) Money {
	if v >= 0 {
		return Money(math.Floor(v + 0.5))
	}
	return Money(math.Ceil(v - 0.5))
}

// MulFrac returns m*ratio rounded to the nearest cent. Used for proportional
// cessions and commission.
func (m Money) MulFrac(ratio float64) Money {
	return RoundCents(float64(m) * ratio)
}

// Add returns m+other.
func (m Money) Add(other Money) Money { return m + other }

// Sub returns m-other.
func (m Money) Sub(other Money) Money { return m - other }

// Min returns the smaller of m or other.
func (m Money) Min(other Money) Money {
	if m < other {
		return m
	}
	return other
}

// Max returns the larger of m or other.
func (m Money) Max(other Money) Money {
	if m > other {
		return m
	}
	return other
}

// Clamp restricts m to the inclusive [lo, hi] range.
func (m Money) Clamp(lo, hi Money) Money {
	if m < lo {
		return lo
	}
	if m > hi {
		return hi
	}
	return m
}

// Int returns the underlying int64 value.
func (m Money) Int() int64 { return int64(m) }

// roundDiv returns round(a/b) with round-half-away-from-zero for integer
// inputs; b must be non-zero. Used when the divisor is itself an integer amount
// (e.g. spreading a loss across a known limit count).
func roundDiv(a, b int64) int64 {
	if b == 0 {
		return 0
	}
	q := a / b
	r := a % b
	if r == 0 {
		return q
	}
	// Adjust for half-away-from-zero.
	if (a < 0) != (b < 0) {
		// Different signs: result is negative or zero; round magnitude up if |r|*2 >= |b|
		if abs64(r)*2 >= abs64(b) {
			return q - 1
		}
		return q
	}
	if abs64(r)*2 >= abs64(b) {
		return q + 1
	}
	return q
}

func abs64(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
