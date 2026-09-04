package utils_test

import (
	"rebate/pkg/utils"
	"testing"
)

func TestClamp(t *testing.T) {
	tests := []struct {
		name  string
		value float64
		min   float64
		max   float64
		want  float64
	}{
		{name: "below min", value: -1, min: 0, max: 10, want: 0},
		{name: "above max", value: 11, min: 0, max: 10, want: 10},
		{name: "inside range", value: 5, min: 0, max: 10, want: 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := utils.Clamp(tt.value, tt.min, tt.max); got != tt.want {
				t.Fatalf("Clamp(%v, %v, %v) = %v, want %v", tt.value, tt.min, tt.max, got, tt.want)
			}
		})
	}
}
