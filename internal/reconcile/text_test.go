package reconcile_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/canonical"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/reconcile"
)

func TestFingerprintText(t *testing.T) {
	f := reconcile.Fingerprint{Hi: 0x0123456789abcdef, Lo: 0xfedcba9876543210}
	if got, want := f.String(), "0123456789abcdeffedcba9876543210"; got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
	if got := (reconcile.Fingerprint{}).String(); got != strings.Repeat("0", 32) {
		t.Errorf("zero String() = %q", got)
	}
	got, err := reconcile.ParseFingerprint(f.String())
	if err != nil || got != f {
		t.Fatalf("ParseFingerprint(String()) = %v, %v; want %v", got, err, f)
	}
	for name, s := range map[string]string{
		"empty":      "",
		"short":      f.String()[:31],
		"long":       f.String() + "0",
		"upper case": strings.ToUpper(f.String()),
		"not hex":    "g" + f.String()[1:],
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := reconcile.ParseFingerprint(s); err == nil {
				t.Errorf("ParseFingerprint(%q) accepted", s)
			}
		})
	}
}

func TestFingerprintAndDigestJSON(t *testing.T) {
	el := reconcile.EntityElement(*entity.New("T-1", "ticket"))
	in := struct {
		F reconcile.Fingerprint `json:"f"`
		D canonical.Digest      `json:"d"`
	}{el.Fingerprint(), el.Hash}
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"f":"` + in.F.String() + `","d":"` + in.D.String() + `"}`
	if string(b) != want {
		t.Fatalf("Marshal = %s, want %s", b, want)
	}
	out := in
	out.F, out.D = reconcile.Fingerprint{}, canonical.Digest{}
	if err := json.Unmarshal(b, &out); err != nil || out != in {
		t.Fatalf("Unmarshal = %+v, %v; want %+v", out, err, in)
	}
	for _, bad := range []string{`{"f":"zz"}`, `{"d":"ABC"}`, `{"f":12}`} {
		if err := json.Unmarshal([]byte(bad), &out); err == nil {
			t.Errorf("Unmarshal(%s) accepted", bad)
		}
	}
}
