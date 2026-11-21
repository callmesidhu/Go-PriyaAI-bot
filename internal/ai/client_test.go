package ai

import "testing"

func TestValidate(t *testing.T) {
	if got, err := Validate("Awww, miss cheytho? ❤️"); err != nil || got == "" {
		t.Fatalf("expected valid Manglish, got %q, %v", got, err)
	}
	if _, err := Validate("മലയാളം"); err == nil {
		t.Fatal("expected Malayalam Unicode to be rejected")
	}
	if _, err := Validate(""); err == nil {
		t.Fatal("expected empty reply to be rejected")
	}
}
