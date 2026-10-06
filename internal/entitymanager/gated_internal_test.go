package entitymanager

import "testing"

// gated strips every form of ACL bypass, so a cascade writer cannot hand its
// trigger-authorized authority to a nested cascade or a script (BUG-J3PBFN).
func TestGated_StripsEveryBypass(t *testing.T) {
	for _, tc := range []struct {
		name string
		m    *Manager
	}{
		{"elevated", &Manager{bypassACL: true}},
		{"cascade writer", &Manager{cascadeWrite: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			g := tc.m.gated()
			if g == tc.m || g.bypassACL || g.cascadeWrite {
				t.Errorf("gated() = %+v, want a fresh handle with no bypass", g)
			}
		})
	}
	plain := &Manager{}
	if plain.gated() != plain {
		t.Error("gated() on a plain manager should return the receiver")
	}
}
