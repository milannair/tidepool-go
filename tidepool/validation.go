package tidepool

import (
	"fmt"
	"math"
)

// ValidateVector validates vector contents and optional expected dimensions.
func ValidateVector(v Vector, expectedDims int) error {
	if len(v) == 0 {
		return fmt.Errorf("%w: vector cannot be empty", ErrValidation)
	}
	if expectedDims > 0 && len(v) != expectedDims {
		return fmt.Errorf("%w: expected %d dimensions, got %d", ErrValidation, expectedDims, len(v))
	}
	for i, val := range v {
		if math.IsNaN(float64(val)) || math.IsInf(float64(val), 0) {
			return fmt.Errorf("%w: invalid value at index %d", ErrValidation, i)
		}
	}
	return nil
}
