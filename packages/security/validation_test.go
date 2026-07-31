package security

import "testing"

func TestValidatePasswordStrength(t *testing.T) {
	tests := []struct {
		password string
		valid    bool
	}{
		{"Weak", false},
		{"NoNumbers!", false},
		{"NoSpecial1", false},
		{"lowercase1!", false},
		{"UPPERCASE1!", false},
		{"StrongPass1!", true},
	}

	for _, tt := range tests {
		err := ValidatePasswordStrength(tt.password)
		if (err == nil) != tt.valid {
			t.Errorf("Password %s expected valid=%t, got err=%v", tt.password, tt.valid, err)
		}
	}
}

func TestSanitizeInput(t *testing.T) {
	dirty := "Hello <script>alert('xss')</script>World!"
	clean := SanitizeInput(dirty)
	expected := "Hello World!"
	if clean != expected {
		t.Errorf("Expected sanitized output to be '%s', got '%s'", expected, clean)
	}
}
