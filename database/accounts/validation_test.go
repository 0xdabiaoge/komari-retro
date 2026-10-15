package accounts

import (
	"strings"
	"testing"
)

func TestValidateUserUpdate(t *testing.T) {
	for _, tt := range []struct {
		name, password string
		valid          bool
	}{
		{"admin", "SafePassword2026!", true},
		{"中文用户", "Aa123456", true},
		{strings.Repeat("u", 255), "Aa1" + strings.Repeat("x", 4093), true},
		{"ab", "SafePassword2026!", false},
		{strings.Repeat("u", 256), "SafePassword2026!", false},
		{"admin", "Aa1abc", false},
		{"admin", "Aa1中文", false},
		{"admin", "Aa1中文密码好", true},
		{"admin", "alllowercase2026", false},
		{"admin", "ALLUPPERCASE2026", false},
		{"admin", "NoDigitsHere", false},
		{"admin", "Aa1" + strings.Repeat("x", 4094), false},
	} {
		if got := ValidateUserUpdate(&tt.name, &tt.password); (got == nil) != tt.valid {
			t.Errorf("name length %d, password length %d: %v", len(tt.name), len(tt.password), got)
		}
	}
	if err := ValidateUserUpdate(nil, nil); err != nil {
		t.Fatal(err)
	}
	// Renaming an account must not impose a password change on a legacy user.
	name := "legacy-admin"
	if err := ValidateUserUpdate(&name, nil); err != nil {
		t.Fatal(err)
	}
}
