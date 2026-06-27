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
		{"required", ScanDefault}, // forgiven, no-op
		{"on", ScanDefault},
		{"", ScanDefault},
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

func TestScanPolicy_InvalidValueErrors(t *testing.T) {
	var s ScanPolicy
	if err := yaml.Unmarshal([]byte("maybe"), &s); err == nil {
		t.Error("expected error for invalid scan value")
	}
}

// fileMetaCmd builds a metamodel with one file property, an optional global
// scan command, and an optional per-property scan command / opt-out.
func fileMetaCmd(globalCmd, propCmd []string, propScan ScanPolicy, hasFileProp bool) *Metamodel {
	m := &Metamodel{Entities: map[string]EntityDef{}}
	if len(globalCmd) > 0 {
		m.Attachments = &AttachmentsConfig{ScanCmd: globalCmd}
	}
	props := map[string]PropertyDef{}
	if hasFileProp {
		props["doc"] = PropertyDef{Type: PropertyTypeFile, ScanCmd: propCmd, Scan: propScan}
	} else {
		props["name"] = PropertyDef{Type: PropertyTypeString}
	}
	m.Entities["thing"] = EntityDef{Properties: props}
	return m
}

func TestScanCommandFor(t *testing.T) {
	global := []string{"clamdscan", "{in}"}
	propLevel := []string{"myscan", "{in}"}
	cases := []struct {
		name      string
		globalCmd []string
		propCmd   []string
		propScan  ScanPolicy
		wantCmd   []string
	}{
		{"global only → inherited", global, nil, ScanDefault, global},
		{"property command wins", global, propLevel, ScanDefault, propLevel},
		{"property opt-out beats global", global, nil, ScanOff, nil},
		{"no command anywhere → none", nil, nil, ScanDefault, nil},
		{"property command only", nil, propLevel, ScanDefault, propLevel},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := fileMetaCmd(tc.globalCmd, tc.propCmd, tc.propScan, true)
			prop := m.Entities["thing"].Properties["doc"]
			got := NewAttachmentPolicy(m).ScanCommandFor(prop)
			if len(got) != len(tc.wantCmd) {
				t.Fatalf("ScanCommandFor = %v, want %v", got, tc.wantCmd)
			}
			for i := range got {
				if got[i] != tc.wantCmd[i] {
					t.Errorf("ScanCommandFor[%d] = %q, want %q", i, got[i], tc.wantCmd[i])
				}
			}
		})
	}
}

func TestHasUnconfiguredScan(t *testing.T) {
	cmd := []string{"clamdscan", "{in}"}
	cases := []struct {
		name        string
		globalCmd   []string
		propCmd     []string
		propScan    ScanPolicy
		hasFileProp bool
		want        bool
	}{
		{"file prop, no command → warn", nil, nil, ScanDefault, true, true},
		{"global command → silent", cmd, nil, ScanDefault, true, false},
		{"property command → silent", nil, cmd, ScanDefault, true, false},
		{"explicit off → silent", nil, nil, ScanOff, true, false},
		{"no file prop → silent", nil, nil, ScanDefault, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := fileMetaCmd(tc.globalCmd, tc.propCmd, tc.propScan, tc.hasFileProp)
			if got := NewAttachmentPolicy(m).HasUnconfiguredScan(); got != tc.want {
				t.Errorf("HasUnconfiguredScan = %v, want %v", got, tc.want)
			}
		})
	}
}
