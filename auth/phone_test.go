package auth

import (
	"errors"
	"testing"
)

func TestAuthNormalizePhoneUnit(t *testing.T) {
	testCases := []struct {
		name          string
		inputPhone    string
		expectedPhone string
		expectedErr   error
	}{
		{
			name:          "standard e164 us phone",
			inputPhone:    "+15550192834",
			expectedPhone: "+15550192834",
			expectedErr:   nil,
		},
		{
			name:          "formatted us phone with parentheses, spaces, and hyphen",
			inputPhone:    "+1 (555) 123-4567",
			expectedPhone: "+15551234567",
			expectedErr:   nil,
		},
		{
			name:          "phone with dots and leading/trailing whitespace",
			inputPhone:    "  +1.555.123.4567  ",
			expectedPhone: "+15551234567",
			expectedErr:   nil,
		},
		{
			name:          "minimum valid length e164 phone with 8 digits",
			inputPhone:    "+12345678",
			expectedPhone: "+12345678",
			expectedErr:   nil,
		},
		{
			name:          "maximum valid length e164 phone with 15 digits",
			inputPhone:    "+123456789012345",
			expectedPhone: "+123456789012345",
			expectedErr:   nil,
		},
		{
			name:          "international uk phone with spaces and hyphens",
			inputPhone:    "+44-20-7946-0958",
			expectedPhone: "+442079460958",
			expectedErr:   nil,
		},
		{
			name:          "empty phone number string",
			inputPhone:    "",
			expectedPhone: "",
			expectedErr:   ErrInvalidPhone,
		},
		{
			name:          "whitespace-only phone number string",
			inputPhone:    "   \t\n  ",
			expectedPhone: "",
			expectedErr:   ErrInvalidPhone,
		},
		{
			name:          "missing plus prefix",
			inputPhone:    "15551234567",
			expectedPhone: "",
			expectedErr:   ErrInvalidPhone,
		},
		{
			name:          "phone with letters",
			inputPhone:    "+1 (555) CALL-ME",
			expectedPhone: "",
			expectedErr:   ErrInvalidPhone,
		},
		{
			name:          "phone with special disallowed characters",
			inputPhone:    "+1#555*12345",
			expectedPhone: "",
			expectedErr:   ErrInvalidPhone,
		},
		{
			name:          "phone too short with 7 digits",
			inputPhone:    "+1234567",
			expectedPhone: "",
			expectedErr:   ErrInvalidPhone,
		},
		{
			name:          "phone too long with 16 digits",
			inputPhone:    "+1234567890123456",
			expectedPhone: "",
			expectedErr:   ErrInvalidPhone,
		},
		{
			name:          "plus only with no digits",
			inputPhone:    "+",
			expectedPhone: "",
			expectedErr:   ErrInvalidPhone,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			normalizedPhone, err := NormalizePhone(testCase.inputPhone)
			if testCase.expectedErr != nil {
				if err == nil {
					t.Fatalf("expected error %v, got nil", testCase.expectedErr)
				}
				if !errors.Is(err, testCase.expectedErr) {
					t.Fatalf("expected error %v, got %v", testCase.expectedErr, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if normalizedPhone != testCase.expectedPhone {
				t.Fatalf("expected %q, got %q", testCase.expectedPhone, normalizedPhone)
			}
		})
	}
}
