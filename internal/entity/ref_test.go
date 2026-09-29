package entity

import (
	"encoding/json"
	"testing"
)

// TestParseRef pins that a Ref parses through the one boundary grammar
// (ParseStateRef): a bare id is the implicit face, "ID@face" names a face,
// and everything the grammar rejects is an error, not a guessed Ref.
func TestParseRef(t *testing.T) {
	t.Parallel()

	valid := []struct {
		in   string
		want Ref
	}{
		{"PAGE-1", Ref{ID: "PAGE-1"}},
		{"PAGE-1@draft", Ref{ID: "PAGE-1", Face: "draft"}},
		{"POL-1@nl-be-draft", Ref{ID: "POL-1", Face: "nl-be-draft"}},
		{"a", Ref{ID: "a"}},
	}
	for _, tc := range valid {
		t.Run("valid "+tc.in, func(t *testing.T) {
			t.Parallel()
			got, err := ParseRef(tc.in)
			if err != nil || got != tc.want {
				t.Fatalf("ParseRef(%q) = (%#v, %v), want (%#v, nil)", tc.in, got, err, tc.want)
			}
			if s := got.String(); s != tc.in {
				t.Errorf("String() = %q, want %q", s, tc.in)
			}
		})
	}

	invalid := []struct{ name, in string }{
		{"empty", ""},
		{"empty id", "@draft"},
		{"empty face", "PAGE-1@"},
		{"invalid face", "PAGE-1@Draft"},
		{"face with relation-key separator", "PAGE-1@a--b"},
		{"multi-axis face reserved", "PAGE-1@nl+draft"},
		{"@ in face", "PAGE-1@draft@x"},
		{"@ in id", "PA@GE@draft"},
		{"path separator in id", "PAGE/1@draft"},
		{"consecutive dashes in id", "PAGE--1"},
		{"control char in face", "PAGE-1@draft\x00"},
	}
	for _, tc := range invalid {
		t.Run("invalid "+tc.name, func(t *testing.T) {
			t.Parallel()
			if got, err := ParseRef(tc.in); err == nil {
				t.Errorf("ParseRef(%q) = %#v, want error", tc.in, got)
			}
		})
	}
}

// TestRef_Text pins MarshalText as the inverse of UnmarshalText, and that
// it refuses a Ref whose text would parse back to a different value.
func TestRef_Text(t *testing.T) {
	t.Parallel()

	roundTrip := []Ref{
		{ID: "PAGE-1"},
		{ID: "PAGE-1", Face: "draft"},
		{ID: "x9", Face: "review-2"},
	}
	for _, r := range roundTrip {
		t.Run("round trip "+r.String(), func(t *testing.T) {
			t.Parallel()
			b, err := r.MarshalText()
			if err != nil {
				t.Fatalf("MarshalText(%#v): %v", r, err)
			}
			var back Ref
			if err := back.UnmarshalText(b); err != nil || back != r {
				t.Fatalf("UnmarshalText(%q) = (%#v, %v), want %#v", b, back, err, r)
			}
		})
	}

	refused := []struct {
		name string
		ref  Ref
	}{
		{"zero Ref", Ref{}},
		{"@ in id", Ref{ID: "PAGE@1"}},
		{"@ in id with face", Ref{ID: "PAGE@x", Face: "draft"}},
		{"invalid face", Ref{ID: "PAGE-1", Face: "Draft"}},
		{"face containing @", Ref{ID: "PAGE-1", Face: "a@b"}},
		{"empty id, named face", Ref{Face: "draft"}},
		{"multi-axis face", Ref{ID: "PAGE-1", Face: "nl+draft"}},
		{"relation-key separator in face", Ref{ID: "PAGE-1", Face: "a--b"}},
	}
	for _, tc := range refused {
		t.Run("refused "+tc.name, func(t *testing.T) {
			t.Parallel()
			if b, err := tc.ref.MarshalText(); err == nil {
				t.Errorf("MarshalText(%#v) = %q, want error", tc.ref, b)
			}
		})
	}

	for _, in := range []string{"bad@@x", "KEEP-2@Draft", "KEEP-2@"} {
		t.Run("unmarshal failure leaves the Ref unchanged: "+in, func(t *testing.T) {
			t.Parallel()
			r := Ref{ID: "KEEP-1", Face: "draft"}
			if err := r.UnmarshalText([]byte(in)); err == nil {
				t.Fatalf("UnmarshalText(%q) accepted an invalid ref", in)
			}
			if r != (Ref{ID: "KEEP-1", Face: "draft"}) {
				t.Errorf("Ref changed to %#v on a failed unmarshal", r)
			}
		})
	}
}

// TestRef_JSON pins the unchanged wire form: a Ref is a JSON string, and a
// map keyed by Ref is a JSON object keyed by "ID" or "ID@face".
func TestRef_JSON(t *testing.T) {
	t.Parallel()

	in := map[Ref]Ref{
		{ID: "A-1"}:                {ID: "B-1", Face: "draft"},
		{ID: "A-1", Face: "draft"}: {ID: "B-2"},
	}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	const want = `{"A-1":"B-1@draft","A-1@draft":"B-2"}`
	if string(b) != want {
		t.Fatalf("json = %s, want %s", b, want)
	}
	var out map[Ref]Ref
	if err := json.Unmarshal(b, &out); err != nil {
		t.Fatal(err)
	}
	if len(out) != len(in) {
		t.Fatalf("round trip = %#v, want %#v", out, in)
	}
	for k, v := range in {
		if out[k] != v {
			t.Errorf("out[%v] = %v, want %v", k, out[k], v)
		}
	}

	if _, err := json.Marshal(Ref{}); err == nil {
		t.Error("json.Marshal(Ref{}) succeeded, want error")
	}
	var r Ref
	if err := json.Unmarshal([]byte(`"PAGE-1@"`), &r); err == nil {
		t.Errorf("json.Unmarshal of an empty face = %#v, want error", r)
	}
}

// TestRef_Accessors covers IsZero and Entity.Ref, including the implicit
// face of a faceless entity.
func TestRef_Accessors(t *testing.T) {
	t.Parallel()

	zero := []struct {
		name string
		ref  Ref
		want bool
	}{
		{"zero Ref", Ref{}, true},
		{"face without id addresses nothing", Ref{Face: "draft"}, true},
		{"bare id", Ref{ID: "A-1"}, false},
		{"id and face", Ref{ID: "A-1", Face: "draft"}, false},
	}
	for _, tc := range zero {
		t.Run("IsZero "+tc.name, func(t *testing.T) {
			t.Parallel()
			if got := tc.ref.IsZero(); got != tc.want {
				t.Errorf("%#v.IsZero() = %v, want %v", tc.ref, got, tc.want)
			}
		})
	}

	entities := []struct {
		name       string
		e          *Entity
		wantRef    Ref
		wantString string
	}{
		{"faceless", &Entity{ID: "A-1", Type: "note"}, Ref{ID: "A-1"}, "A-1"},
		{
			"faced", &Entity{ID: "POL-1", Type: "policy", Face: "published"},
			Ref{ID: "POL-1", Face: "published"}, "POL-1@published",
		},
	}
	for _, tc := range entities {
		t.Run("Entity.Ref "+tc.name, func(t *testing.T) {
			t.Parallel()
			got := tc.e.Ref()
			if got != tc.wantRef || got.String() != tc.wantString {
				t.Errorf("Ref() = %#v (%q), want %#v (%q)", got, got.String(), tc.wantRef, tc.wantString)
			}
		})
	}
}
