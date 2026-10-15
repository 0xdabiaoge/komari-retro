package accounts

import (
	"fmt"
	"unicode/utf8"
)

// ValidateUserUpdate applies to new profile changes only. Existing credentials
// remain usable, including accounts created before the password policy existed.
func ValidateUserUpdate(name, password *string) error {
	if name != nil && (len(*name) < 3 || len(*name) > 255) {
		return fmt.Errorf("用户名长度必须为 3 到 255 字节")
	}
	if password == nil {
		return nil
	}
	if utf8.RuneCountInString(*password) < 8 || len(*password) > 4096 {
		return fmt.Errorf("密码至少需要 8 个字符，且不能超过 4096 字节")
	}
	var lower, upper, digit bool
	for _, char := range *password {
		lower = lower || char >= 'a' && char <= 'z'
		upper = upper || char >= 'A' && char <= 'Z'
		digit = digit || char >= '0' && char <= '9'
	}
	if !lower || !upper || !digit {
		return fmt.Errorf("密码必须包含大写字母、小写字母和数字")
	}
	return nil
}
