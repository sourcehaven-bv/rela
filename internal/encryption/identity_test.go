package encryption

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

// newTestIdentity returns a fresh identity for use in tests. Hides
// the Generate error boilerplate.
func newTestIdentity(t *testing.T) Identity {
	t.Helper()
	id, err := GenerateIdentity()
	if err != nil {
		t.Fatalf("GenerateIdentity: %v", err)
	}
	return id
}

func TestIdentity_String_Redacts(t *testing.T) {
	id := newTestIdentity(t)
	if s := id.(interface{ String() string }).String(); strings.Contains(s, "AGE-SECRET-KEY-1") {
		t.Errorf("Identity.String() must not contain the secret (got %q)", s)
	}
}

func TestIdentity_MarshalJSON_Redacts(t *testing.T) {
	id := newTestIdentity(t)
	b, err := json.Marshal(id)
	if err != nil {
		t.Fatalf("json.Marshal: %v", err)
	}
	if strings.Contains(string(b), "AGE-SECRET-KEY-1") {
		t.Errorf("json.Marshal must not contain the secret (got %s)", b)
	}
}

// TestSecretTypes_NoStringMethods is the reflective guard that
// ensures no secret-bearing type grows a String()/MarshalText that
// leaks. The exported Identity interface already defines String as
// redacted; this test fails loudly if someone later embeds a type
// whose String() is the age default (which would print the secret).
func TestSecretTypes_NoStringMethods(t *testing.T) {
	id := newTestIdentity(t)
	// x25519Identity is the concrete type; inspect its struct fields
	// to make sure the embedded *age.X25519Identity isn't publicly
	// reachable via a method that would print the secret.
	rv := reflect.ValueOf(id).Elem()
	for i := range rv.NumField() {
		f := rv.Type().Field(i)
		if f.IsExported() {
			t.Errorf("x25519Identity.%s is exported; secret fields MUST be unexported", f.Name)
		}
	}
}

func TestParseRecipient_BadInput(t *testing.T) {
	cases := []string{"", "   ", "not-a-recipient", "age1garbage"}
	for _, c := range cases {
		if _, err := ParseRecipient(c); err == nil {
			t.Errorf("ParseRecipient(%q) should error", c)
		}
	}
}

func TestParseIdentity_BadInput(t *testing.T) {
	cases := []string{"", "   ", "not-an-identity", "AGE-SECRET-KEY-1GARBAGE"}
	for _, c := range cases {
		if _, err := ParseIdentity(c); err == nil {
			t.Errorf("ParseIdentity(%q) should error", c)
		}
	}
}

func TestReadIdentity_EmptyInput(t *testing.T) {
	_, err := ReadIdentity(strings.NewReader(""))
	if err == nil {
		t.Fatal("ReadIdentity(empty) should error")
	}
}

func TestReadIdentity_Multiple(t *testing.T) {
	a := newTestIdentity(t)
	b := newTestIdentity(t)
	input := a.(*x25519Identity).i.String() + "\n" + b.(*x25519Identity).i.String() + "\n"
	if _, err := ReadIdentity(strings.NewReader(input)); err == nil {
		t.Fatal("ReadIdentity(two identities) should error (expected one)")
	}
}

func TestReadIdentity_WithComments(t *testing.T) {
	a := newTestIdentity(t)
	input := "# this is a comment\n\n" + a.(*x25519Identity).i.String() + "\n"
	got, err := ReadIdentity(strings.NewReader(input))
	if err != nil {
		t.Fatalf("ReadIdentity: %v", err)
	}
	if got.PublicRecipient().String() != a.PublicRecipient().String() {
		t.Errorf("ReadIdentity returned different identity")
	}
}
