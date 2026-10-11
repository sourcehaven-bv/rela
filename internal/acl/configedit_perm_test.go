package acl

import "testing"

func TestPermissionCeiling_NeverPassesConfigEdit(t *testing.T) {
	for name, c := range map[string]permissionCeiling{
		"unrestricted": {},
		"wildcard":     {allow: []string{"*"}},
		"named":        {allow: []string{PermConfigEdit}},
		"scope grant":  {allow: []string{}, except: []string{PermConfigEdit}},
	} {
		if c.permits(PermConfigEdit) {
			t.Errorf("%s: a client ceiling passed %s", name, PermConfigEdit)
		}
	}
	if !(permissionCeiling{}).permits(PermHistoryRead) {
		t.Error("an unrestricted ceiling withheld an ordinary permission")
	}
}
