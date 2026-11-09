package flowcfg

import (
	"errors"
	"strings"
	"testing"
)

func TestParseFlow_RoundTrip(t *testing.T) {
	c, err := parseSample(t)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := c.Flow("onboarding")
	got, err := ParseFlow("onboarding", f.Source(), f.Version(), testMeta(t), testPolicy(t))
	if err != nil {
		t.Fatalf("ParseFlow: %v", err)
	}
	if got.Version() != f.Version() || got.Initial() != f.Initial() || len(got.States()) != len(f.States()) {
		t.Errorf("snapshot compiled to version %s initial %s with %d states; want %s %s %d",
			got.Version(), got.Initial(), len(got.States()), f.Version(), f.Initial(), len(f.States()))
	}
}

func TestParseFlow_Refusals(t *testing.T) {
	c, err := parseSample(t)
	if err != nil {
		t.Fatal(err)
	}
	f, _ := c.Flow("onboarding")
	src := string(f.Source())
	tests := []struct {
		name, flow, src, version, want string
	}{
		{"other version", "onboarding", src, "0000000000000000", "hashes to version"},
		{"schema no longer fits", "onboarding", strings.Replace(src, "subject: person", "subject: human", 1),
			f.Version(), "does not declare"},
		{"not yaml", "onboarding", "a: [", f.Version(), "flows.onboarding"},
		{"empty", "onboarding", "", f.Version(), "one flow declaration"},
		{"alias", "onboarding", "a: &x 1\nb: *x\n", f.Version(), "anchor or alias"},
		{"bad name", "Bad Name", src, f.Version(), "valid flow name"},
		{"too big", "onboarding", strings.Repeat(" ", MaxFileBytes+1), f.Version(), "over"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := ParseFlow(tc.flow, []byte(tc.src), tc.version, testMeta(t), testPolicy(t))
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("err = %v, want *Error", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
	if _, err := ParseFlow("onboarding", f.Source(), f.Version(), nil, nil); err == nil {
		t.Error("ParseFlow accepted a nil metamodel")
	}
}

func TestParse_ManagedNames(t *testing.T) {
	tests := []struct {
		name string
		edit [2]string
		key  string
	}{
		{"subject", [2]string{"subject: person", "subject: _flow-run"}, "flows.onboarding.subject"},
		{"create type", [2]string{"type: task", "type: _flow-run"}, "flows.onboarding.states.collect.outputs[1].create.type"},
		{"relate type", [2]string{"relate: { type: about }", "relate: { type: _carries }"},
			"flows.onboarding.states.collect.outputs[1].create.relate.type"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseSample(t, tc.edit)
			var e *Error
			if !errors.As(err, &e) {
				t.Fatalf("err = %v, want *Error", err)
			}
			for _, p := range e.Problems {
				if p.Key == tc.key && strings.Contains(p.Message, "rela manages itself") {
					return
				}
			}
			t.Errorf("problems %v lack a managed-name refusal at %s", e.Problems, tc.key)
		})
	}
}
