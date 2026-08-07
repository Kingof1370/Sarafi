package security

import (
	"encoding/base32"
	"testing"
	"time"
)

func TestValidateTOTP(t *testing.T) {
	// Secret key for testing
	secret := "JBSWY3DPEHPK3PXP"

	// Try with an invalid format or empty secret
	if ValidateTOTP("", "123456") {
		t.Error("Expected empty secret validation to fail")
	}

	// Try validating with invalid 6-digit code length
	if ValidateTOTP(secret, "12345") {
		t.Error("Expected 5-digit code validation to fail")
	}

	// Let's generate a valid code dynamically for the current time step
	key, _ := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	currentTime := time.Now().Unix()
	counter := currentTime / 30
	code := generateHOTP(key, counter)

	if !ValidateTOTP(secret, code) {
		t.Errorf("Expected dynamically generated TOTP %s to validate successfully", code)
	}

	if ValidateTOTP(secret, "999999") {
		t.Error("Expected random incorrect code to fail")
	}
}
