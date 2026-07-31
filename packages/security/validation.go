package security

import (
	"errors"
	"regexp"
	"unicode"
)

// ValidatePasswordStrength enforces strong corporate cryptographic password criteria:
// - Minimum length of 8 characters
// - At least one uppercase letter
// - At least one lowercase letter
// - At least one numeric digit
// - At least one special symbol character
func ValidatePasswordStrength(password string) error {
	if len(password) < 8 {
		return errors.New("password length must be at least 8 characters")
	}

	var (
		hasUpper   = false
		hasLower   = false
		hasNumber  = false
		hasSpecial = false
	)

	for _, r := range password {
		switch {
		case unicode.IsUpper(r):
			hasUpper = true
		case unicode.IsLower(r):
			hasLower = true
		case unicode.IsNumber(r):
			hasNumber = true
		case unicode.IsPunct(r) || unicode.IsSymbol(r):
			hasSpecial = true
		}
	}

	if !hasUpper {
		return errors.New("password must contain at least one uppercase letter")
	}
	if !hasLower {
		return errors.New("password must contain at least one lowercase letter")
	}
	if !hasNumber {
		return errors.New("password must contain at least one number")
	}
	if !hasSpecial {
		return errors.New("password must contain at least one special character")
	}

	return nil
}

// SanitizeInput filters potentially dangerous HTML tags and cross-site scripting attempts
func SanitizeInput(input string) string {
	re := regexp.MustCompile(`(?i)<script.*?>.*?</script>|<[^>]*>`)
	return re.ReplaceAllString(input, "")
}
