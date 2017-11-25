package main

import (
	"math"
	"testing"
)

const minNormal = 2.2250738585072014E-308 // Smallest positive normal value of type float64

func TestIntegrator(t *testing.T) {
	fn := func(t, _ float64) float64 {
		return math.Exp(-(t * t * t * t))
	}
	res := numIntegrate(fn, -2, 2, 1000)
	if !nearlyEqual(res, 1.81280494737, 0.00000001) {
		t.Errorf("Expected %v got %v", 1.81280494737, res)
	}
}

// Returns true if two floating point numbers are within epsilon of each other
func nearlyEqual(a, b, epsilon float64) bool {
	diff := math.Abs(a - b)

	if a == b {
		return true
	} else if a == 0 || b == 0 || diff < minNormal {
		return diff < (epsilon * minNormal)
	} else {
		absA, absB := math.Abs(a), math.Abs(b)
		return diff/math.Min((absA+absB), math.MaxFloat64) < epsilon
	}
}
