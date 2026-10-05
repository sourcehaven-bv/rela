package acl

import (
	"reflect"
	"strings"
	"testing"
)

// The world axis of the client ceiling is tested at COMPILATION, in-package
// and directly, per the standing rule that a ceiling fails toward less
// access everywhere except the compilation step — which is why that step
// gets unit tests rather than only end-to-end ones (CLAUDE.md).

func worldCeiling(t *testing.T, baseline ClientBaseline, scopes ...string) compiledCeiling {
	t.Helper()
	p := &Policy{ClientBaselines: map[string]ClientBaseline{"app": baseline}}
	if len(scopes) > 0 {
		p.ScopeGrants = map[string]ScopeGrant{
			"extra": {Restriction: Restriction{Worlds: []string{"editorial"}}},
		}
	}
	p.normalizeClientAttenuation()
	return p.ceilingFor("app", scopes)
}

func TestCeilingWorlds_Clamp(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		baseline ClientBaseline
		roleHas  []string
		want     string
	}{
		{
			name:     "no world axis: role keeps its worlds",
			baseline: ClientBaseline{AppliesTo: []string{"app"}, Restriction: Restriction{DenyWrite: []string{"*"}}},
			roleHas:  []string{"published", "editorial"},
			want:     "published,editorial",
		},
		{
			name: "allowlist narrows to the intersection",
			baseline: ClientBaseline{AppliesTo: []string{"app"},
				Restriction: Restriction{Worlds: []string{"published"}}},
			roleHas: []string{"published", "editorial"},
			want:    "published",
		},
		{
			name: "allowlist naming a world the role lacks grants nothing new",
			baseline: ClientBaseline{AppliesTo: []string{"app"},
				Restriction: Restriction{Worlds: []string{"editorial"}}},
			roleHas: []string{"published"},
			want:    "",
		},
		{
			name: "denylist removes the named world",
			baseline: ClientBaseline{AppliesTo: []string{"app"},
				Restriction: Restriction{DenyWorlds: []string{"editorial"}}},
			roleHas: []string{"published", "editorial"},
			want:    "published",
		},
		{
			name: "empty allowlist permits no world",
			baseline: ClientBaseline{AppliesTo: []string{"app"},
				Restriction: Restriction{Worlds: []string{}}},
			roleHas: []string{"published"},
			want:    "",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c := worldCeiling(t, tc.baseline)
			got := c.clamp(RoleDef{Worlds: tc.roleHas}).Worlds
			if strings.Join(got, ",") != tc.want {
				t.Errorf("clamped worlds = %q, want %q", strings.Join(got, ","), tc.want)
			}
		})
	}
}

// TestCeilingWorlds_ScopeReopens pins that a scope grant widens the world
// axis exactly as it widens the verb axes — the union semantics that make
// "more scopes means more capability" hold.
func TestCeilingWorlds_ScopeReopens(t *testing.T) {
	t.Parallel()
	c := worldCeiling(t, ClientBaseline{AppliesTo: []string{"app"},
		Restriction: Restriction{Worlds: []string{"published"}}}, "extra")
	got := c.clamp(RoleDef{Worlds: []string{"published", "editorial"}}).Worlds
	if strings.Join(got, ",") != "published,editorial" {
		t.Errorf("scope must re-open editorial; got %q", strings.Join(got, ","))
	}
}

// TestCeilingWorlds_DefaultWorldAlwaysPermitted pins that an active ceiling
// never denies the default world. A deny_worlds naming it is refused at load
// (TestValidateAgainstMetamodel_DenyDefaultWorld), so permitsWorld need not
// and must not consult it.
func TestCeilingWorlds_DefaultWorldAlwaysPermitted(t *testing.T) {
	t.Parallel()
	c := worldCeiling(t, ClientBaseline{AppliesTo: []string{"app"},
		Restriction: Restriction{Worlds: []string{"published"}}})
	c.defaultWorld = "editorial"
	for _, w := range []string{"", "editorial", "published"} {
		if !c.permitsWorld(w) {
			t.Errorf("permitsWorld(%q) = false, want true", w)
		}
	}
	if c.permitsWorld("archive") {
		t.Error("a world outside the allowlist that is not the default must be denied")
	}
}

func TestCeilingWorlds_InactiveCeilingPermitsEverything(t *testing.T) {
	t.Parallel()
	var c compiledCeiling // no baseline matched
	for _, w := range []string{"", DefaultWorldName, "published"} {
		if !c.permitsWorld(w) {
			t.Errorf("an inactive ceiling must not narrow anything; denied %q", w)
		}
	}
}

// TestCeilingWorlds_Narrows pins that a worlds-only restriction registers
// as narrowing. Without it, `rela acl audit` A11 would report a real
// restriction as inert config and tell the operator to delete it.
func TestCeilingWorlds_Narrows(t *testing.T) {
	t.Parallel()
	for _, r := range []Restriction{
		{Worlds: []string{"published"}},
		{DenyWorlds: []string{"editorial"}},
	} {
		if !r.Narrows() {
			t.Errorf("restriction %+v must count as narrowing", r)
		}
	}
}

func TestCeilingWorlds_OneSpellingPerAxis(t *testing.T) {
	t.Parallel()
	r := Restriction{Worlds: []string{"a"}, DenyWorlds: []string{"b"}}
	if err := r.validate("client_baselines.app"); err == nil {
		t.Fatal("declaring both worlds and deny_worlds must be rejected")
	}
	// deny_write is write-side; it must NOT collide with the read-side
	// world axis.
	ok := Restriction{DenyWrite: []string{"*"}, Worlds: []string{"published"}}
	if err := ok.validate("client_baselines.app"); err != nil {
		t.Errorf("deny_write and worlds are orthogonal axes: %v", err)
	}
}

func TestCeilingWorlds_BlankRejected(t *testing.T) {
	t.Parallel()
	for _, r := range []Restriction{
		{Worlds: []string{"published", "  "}},
		{DenyWorlds: []string{""}},
	} {
		if err := r.validate("client_baselines.app"); err == nil {
			t.Errorf("a blank world entry is silently inert and must be rejected: %+v", r)
		}
	}
}

// TestCeilingWorlds_AllowlistDoesNotRevokeDefaultWorld pins the answer to
// the trap a design review surfaced: does `worlds: [published]` also take
// away the default world?
//
// It does NOT. A ceiling narrows what it NAMES, and the default world is
// the one world a grant never names — it is the absence of a grant. An
// operator who wants it gone says `deny_worlds: [default]`.
//
// The failure this avoids is the world axis's own hazard pointed backwards:
// the default world is the DRAFT face under the design doc's layout, so a
// client scoped to `published` would find its ordinary reads vanishing for
// a reason nothing in its config states.
func TestCeilingWorlds_AllowlistDoesNotRevokeDefaultWorld(t *testing.T) {
	t.Parallel()
	c := worldCeiling(t, ClientBaseline{AppliesTo: []string{"app"},
		Restriction: Restriction{Worlds: []string{"published"}}})

	if !c.permitsWorld(DefaultWorldName) {
		t.Error("an allowlist naming only `published` must not revoke the default world")
	}
	if !c.permitsWorld("") {
		t.Error("the empty world name means the default world; same rule")
	}
	if !c.permitsWorld("published") {
		t.Error("the allowlisted world must be permitted")
	}
	if c.permitsWorld("editorial") {
		t.Error("a world outside the allowlist must be denied")
	}
}

// TestClampCoversEveryGrantAxis is a field-coverage guard: every slice
// field on RoleDef that carries GRANTS must either be narrowed by
// [compiledCeiling.clamp] or be explicitly exempted here.
//
// The failure it catches has now happened once per axis added. A new grant
// axis that clamp forgets produces NO compile error and NO test failure —
// ceilingguard_test.go scans for direct `policy.Roles[...]` access, not for
// field coverage — so the axis silently escapes client attenuation. That is
// the fail-open direction, and CLAUDE.md's ceiling rule ("a ceiling only
// ever NARROWS") is the thing it breaks.
//
// The exemption list is the point: adding a field forces a decision here
// rather than allowing an omission to pass unnoticed.
func TestClampCoversEveryGrantAxis(t *testing.T) {
	t.Parallel()

	// Fields that are deliberately NOT clamped by this mechanism, each with
	// the reason it is safe.
	exempt := map[string]string{
		"Description": "documentation only; never consulted by any decision path",
		"Fields":      "affordance map — clamped at evaluation time by internal/affordances (applyClientCeiling)",
		"Visible":     "affordance map — same, and the field axis is resolved against declared properties",
		"Options":     "affordance map — same",
		"Relations":   "affordance map — same",
	}

	// A role holding every axis, so a clamped field can be told from an
	// untouched one by comparing before/after.
	full := RoleDef{
		Create:      []string{"a"},
		Update:      []string{"a"},
		Delete:      []string{"a"},
		Read:        []string{"a"},
		Permissions: []string{"p"},
		Worlds:      []string{"w"},
	}
	// A ceiling that permits NOTHING on every axis, so every clamped field
	// must come back empty.
	p := &Policy{ClientBaselines: map[string]ClientBaseline{"app": {
		AppliesTo: []string{"app"},
		Restriction: Restriction{
			Create: []string{}, Update: []string{}, Delete: []string{},
			Read: []string{}, Permissions: []string{}, Worlds: []string{},
		},
	}}}
	p.normalizeClientAttenuation()
	got := p.ceilingFor("app", nil).clamp(full)

	rt := reflect.TypeFor[RoleDef]()
	rv := reflect.ValueOf(got)
	for i := range rt.NumField() {
		f := rt.Field(i)
		if reason, ok := exempt[f.Name]; ok {
			if reason == "" {
				t.Errorf("field %q is exempt with no reason", f.Name)
			}
			continue
		}
		if f.Type.Kind() != reflect.Slice {
			continue
		}
		if l := rv.Field(i).Len(); l != 0 {
			t.Errorf("RoleDef.%s survived a permit-nothing ceiling (len=%d) — "+
				"clamp does not narrow it. Add it to clamp, or exempt it here "+
				"with the reason it cannot be used to escalate.", f.Name, l)
		}
	}
}
