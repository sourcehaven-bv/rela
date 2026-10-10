package entitymanager

import "testing"

// TestGated_DropsFieldGate: an automation cascade dispatches through
// gated(), so a script action triggered by a field-gated write is not
// field-gated (RR-00ERM9), while the row ACL still applies.
func TestGated_DropsFieldGate(t *testing.T) {
	m := FieldGated(&Manager{})
	if !fieldGated(m) {
		t.Fatal("FieldGated handle is not field-gated")
	}
	g := m.gated()
	if fieldGated(g) || g.bypassACL || g.cascadeWrite {
		t.Errorf("gated() = %+v, want a plain handle", g)
	}
	if fieldGated(m.elevated()) {
		t.Error("an elevated handle must skip the field gate")
	}
}
