package main

import (
	"testing"
)

func TestCompareVersions(t *testing.T) {
	tests := []struct {
		current  string
		latest   string
		expected bool
		hasError bool
	}{
		{"dev", "1.0.0", true, false},
		{"unknown", "1.0.0", true, false},
		{"1.0.0", "1.0.1", true, false},
		{"1.0.0", "1.1.0", true, false},
		{"1.0.0", "2.0.0", true, false},
		{"1.0.1", "1.0.0", false, false},
		{"1.1.0", "1.0.0", false, false},
		{"2.0.0", "1.0.0", false, false},
		{"1.0.0", "1.0.0", false, false},
		{"v1.0.0", "v1.0.1", true, false},
		{"v1.0.1", "v1.0.0", false, false},
		{"1.2.3", "1.2.4", true, false},
		{"0.3.6", "0.3.7", true, false},
		{"0.3.7", "0.3.6", false, false},
	}

	for _, test := range tests {
		result, err := compareVersions(test.current, test.latest)

		if test.hasError && err == nil {
			t.Errorf("compareVersions(%q, %q) expected error but got none", test.current, test.latest)
		}

		if !test.hasError && err != nil {
			t.Errorf("compareVersions(%q, %q) unexpected error: %v", test.current, test.latest, err)
		}

		if result != test.expected {
			t.Errorf("compareVersions(%q, %q) = %v, want %v", test.current, test.latest, result, test.expected)
		}
	}
}