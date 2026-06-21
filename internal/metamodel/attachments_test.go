package metamodel

import (
	"testing"

	"gopkg.in/yaml.v3"
)

func TestScanPolicy_UnmarshalYAML(t *testing.T) {
	cases := []struct {
		in   string
		want ScanPolicy
	}{
		{"off", ScanOff},
		{"OFF", ScanOff},
		{"required", ScanRequired},
		{"Required", ScanRequired},
	}
	for _, tc := range cases {
		t.Run(tc.in, func(t *testing.T) {
			var s ScanPolicy
			if err := yaml.Unmarshal([]byte(tc.in), &s); err != nil {
				t.Fatalf("unmarshal %q: %v", tc.in, err)
			}
			if s != tc.want {
				t.Errorf("got %v, want %v", s, tc.want)
			}
		})
	}
}

func TestScanPolicy_AbsentIsUnset(t *testing.T) {
	// A struct whose scan field is omitted must leave the zero value (unset),
	// which is what distinguishes it from an explicit off.
	var cfg AttachmentsConfig
	if err := yaml.Unmarshal([]byte("allow: [image/png]\n"), &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Scan != ScanUnset {
		t.Errorf("absent scan = %v, want unset", cfg.Scan)
	}
}

func TestScanPolicy_InvalidValueErrors(t *testing.T) {
	var s ScanPolicy
	if err := yaml.Unmarshal([]byte("maybe"), &s); err == nil {
		t.Error("expected error for invalid scan policy")
	}
}

func fileMeta(global, propScan ScanPolicy, hasFileProp bool) *Metamodel {
	m := &Metamodel{Entities: map[string]EntityDef{}}
	if global != ScanUnset {
		m.Attachments = &AttachmentsConfig{Scan: global}
	}
	props := map[string]PropertyDef{}
	if hasFileProp {
		props["doc"] = PropertyDef{Type: PropertyTypeFile, Scan: propScan}
	} else {
		props["name"] = PropertyDef{Type: PropertyTypeString}
	}
	m.Entities["thing"] = EntityDef{Properties: props}
	return m
}

func TestHasUnsetScanPolicy(t *testing.T) {
	cases := []struct {
		name        string
		global      ScanPolicy
		propScan    ScanPolicy
		hasFileProp bool
		want        bool
	}{
		{"file prop, nothing set → warn", ScanUnset, ScanUnset, true, true},
		{"global required → silent", ScanRequired, ScanUnset, true, false},
		{"global off → silent", ScanOff, ScanUnset, true, false},
		{"per-prop off → silent", ScanUnset, ScanOff, true, false},
		{"per-prop required → silent", ScanUnset, ScanRequired, true, false},
		{"no file prop → silent", ScanUnset, ScanUnset, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := fileMeta(tc.global, tc.propScan, tc.hasFileProp)
			if got := m.HasUnsetScanPolicy(); got != tc.want {
				t.Errorf("HasUnsetScanPolicy = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestEffectiveScanPolicy(t *testing.T) {
	m := &Metamodel{Attachments: &AttachmentsConfig{Scan: ScanRequired}}
	// Property unset → inherit global.
	if got := m.EffectiveScanPolicy(PropertyDef{Type: PropertyTypeFile}); got != ScanRequired {
		t.Errorf("inherit global = %v, want required", got)
	}
	// Property override wins.
	if got := m.EffectiveScanPolicy(PropertyDef{Type: PropertyTypeFile, Scan: ScanOff}); got != ScanOff {
		t.Errorf("override = %v, want off", got)
	}
}
