package evaluation

import (
	"fmt"
	"math"
)

const AccuracyFormula = "absolute-difference-over-min-v2"

// AccuracyError is symmetric for reciprocal speed ratios. Nonpositive or
// nonfinite observations cannot contribute to an accuracy summary.
func AccuracyError(predicted, reference float64) (float64, error) {
	if predicted <= 0 || reference <= 0 || math.IsNaN(predicted) || math.IsNaN(reference) || math.IsInf(predicted, 0) || math.IsInf(reference, 0) {
		return 0, fmt.Errorf("accuracy requires finite positive times")
	}
	value := 100 * math.Abs(predicted-reference) / math.Min(predicted, reference)
	if math.IsInf(value, 0) {
		return 0, fmt.Errorf("accuracy overflow")
	}
	return value, nil
}
