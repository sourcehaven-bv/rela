package entity

import "testing"

func TestParseAddress(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name      string
		in        string
		wantErr   bool
		wantID    string
		wantNamed bool
		wantRef   Ref
	}{
		{name: "bare id", in: "POL-1", wantID: "POL-1"},
		{name: "named face", in: "POL-1@draft", wantID: "POL-1", wantNamed: true,
			wantRef: Ref{ID: "POL-1", Face: "draft"}},
		{name: "empty", in: "", wantErr: true},
		{name: "separator only", in: "@", wantErr: true},
		{name: "empty face", in: "POL-1@", wantErr: true},
		{name: "empty id", in: "@draft", wantErr: true},
		{name: "two separators", in: "POL-1@draft@x", wantErr: true},
		{name: "bad face", in: "POL-1@Draft", wantErr: true},
		{name: "NUL in id", in: "POL\x00-1", wantErr: true},
		{name: "NUL in face", in: "POL-1@dr\x00aft", wantErr: true},
		{name: "multi-axis face", in: "POL-1@nl+draft", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			a, err := ParseAddress(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseAddress(%q) = %#v, want an error", tc.in, a)
				}
				if a != (Address{}) {
					t.Errorf("ParseAddress(%q) returned %#v with its error, want the zero Address", tc.in, a)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseAddress(%q): %v", tc.in, err)
			}
			if a.ID() != tc.wantID {
				t.Errorf("ID() = %q, want %q", a.ID(), tc.wantID)
			}
			ref, named := a.Named()
			if named != tc.wantNamed || ref != tc.wantRef {
				t.Errorf("Named() = (%#v, %v), want (%#v, %v)", ref, named, tc.wantRef, tc.wantNamed)
			}
			if a.String() != tc.in {
				t.Errorf("String() = %q, want %q", a.String(), tc.in)
			}
		})
	}
}

func TestAddressConstructors(t *testing.T) {
	t.Parallel()

	bare := BareAddress("TKT-1")
	if ref, ok := bare.Named(); ok || ref != (Ref{}) {
		t.Errorf("BareAddress.Named() = (%#v, %v), want the zero Ref and false", ref, ok)
	}
	if bare.ID() != "TKT-1" || bare.String() != "TKT-1" {
		t.Errorf("BareAddress = %q / %q, want TKT-1", bare.ID(), bare.String())
	}

	// An implicit-face Ref, named: Named reports it, but the wire form is the
	// bare id, which parses back as unnamed.
	implicit := AddressOf(Ref{ID: "TKT-1", Face: ImplicitFace})
	if ref, ok := implicit.Named(); !ok || ref != (Ref{ID: "TKT-1"}) {
		t.Errorf("AddressOf(implicit).Named() = (%#v, %v), want (TKT-1, true)", ref, ok)
	}
	if implicit.String() != "TKT-1" {
		t.Errorf("AddressOf(implicit).String() = %q, want TKT-1", implicit.String())
	}
	back, err := ParseAddress(implicit.String())
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := back.Named(); ok {
		t.Error("the bare id of an implicit-face Ref parsed back as named; a bare id must name no face")
	}
	if implicit == bare {
		t.Error("AddressOf(implicit) == BareAddress: the named bit was lost")
	}

	faced := AddressOf(Ref{ID: "POL-1", Face: "draft"})
	if ref, ok := faced.Named(); !ok || ref != (Ref{ID: "POL-1", Face: "draft"}) {
		t.Errorf("AddressOf(faced).Named() = (%#v, %v)", ref, ok)
	}
	if faced.String() != "POL-1@draft" {
		t.Errorf("AddressOf(faced).String() = %q, want POL-1@draft", faced.String())
	}

	var zero Address
	if zero.ID() != "" || zero.String() != "" {
		t.Errorf("zero Address = %q / %q, want empty", zero.ID(), zero.String())
	}
	if _, ok := zero.Named(); ok {
		t.Error("zero Address is named")
	}
}
