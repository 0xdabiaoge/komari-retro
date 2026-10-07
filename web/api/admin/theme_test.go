package admin

import "testing"

func TestIsValidThemeShortProtectsBuiltins(t *testing.T) {
	tests := []struct {
		short string
		valid bool
	}{
		{short: "default", valid: false},
		{short: "retro", valid: false},
		{short: "next", valid: false},
		{short: "nasdaq", valid: false},
		{short: "custom-theme", valid: true},
		{short: "../custom", valid: false},
	}

	for _, test := range tests {
		t.Run(test.short, func(t *testing.T) {
			if got := isValidThemeShort(test.short); got != test.valid {
				t.Fatalf("isValidThemeShort(%q) = %v, want %v", test.short, got, test.valid)
			}
		})
	}
}
