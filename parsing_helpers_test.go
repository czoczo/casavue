package main

import "testing"

func TestGetSizeFromString(t *testing.T) {
	// the size is the number before "x", not every digit of the attribute
	for value, want := range map[string]int{
		"32x32":          32,
		"180x180":        180,
		"icon-64x64.png": 64,
		"any":            0,
	} {
		if got := getSizeFromString(value); got != want {
			t.Errorf("getSizeFromString(%q) = %d, expected %d", value, got, want)
		}
	}
}
