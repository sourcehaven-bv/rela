package dataentryconfig

import "testing"

// TestValidateNavPile pins that a pile name with surrounding spaces is a
// config error: pile names are trimmed, so the entry would never find its
// pile.
func TestValidateNavPile(t *testing.T) {
	if errs := validateNavEntry(NavigationEntry{Pile: "Notifications"}, &Config{}); len(errs) != 0 {
		t.Errorf("valid pile: errs = %v", errs)
	}
	if errs := validateNavEntry(NavigationEntry{Pile: " Notifications"}, &Config{}); len(errs) != 1 {
		t.Errorf("padded pile: errs = %v, want one", errs)
	}
}
