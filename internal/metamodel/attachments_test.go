package metamodel

import (
	"strings"
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

// TestParse_ScanSocketsRemoved pins that a schema still carrying the removed
// `attachments.scan_sockets` key fails to load with a pointer to its
// replacement. Without the check the loader would ignore the key, and the paths
// it listed would silently stop being bound.
func TestParse_ScanSocketsRemoved(t *testing.T) {
	const base = `
version: "1.0"
entities:
  requirement:
    label: Requirement
    id_prefix: "REQ-"
    id_type: sequential
    properties:
      title:
        type: string
attachments:
  scan_cmd: [clamdscan, "{in}"]
`
	if _, err := Parse([]byte(base)); err != nil {
		t.Fatalf("schema without scan_sockets rejected: %v", err)
	}
	// Any value counts: an emptied list or a bare key still means the operator
	// expects rela to honor it.
	for _, value := range []string{"[/opt/clamav/run/clamd.sock]", "[]", ""} {
		_, err := Parse([]byte(base + "  scan_sockets: " + value + "\n"))
		if err == nil {
			t.Errorf("scan_sockets: %q accepted; want a validation error", value)
			continue
		}
		if !strings.Contains(err.Error(), "RELA_SANDBOX_SCAN_READ_PATHS") {
			t.Errorf("scan_sockets: %q: error does not name the replacement: %v", value, err)
		}
	}

	// A merge key must not hide it either.
	merged := strings.Replace(base, "attachments:\n",
		"attachments:\n  <<: {scan_sockets: [/a]}\n", 1)
	if merged == base {
		t.Fatal("test schema has no attachments block to merge into")
	}
	if _, err := Parse([]byte(merged)); err == nil {
		t.Error("scan_sockets via a merge key accepted; want a validation error")
	} else if !strings.Contains(err.Error(), "RELA_SANDBOX_SCAN_READ_PATHS") {
		t.Errorf("merge key: wrong error: %v", err)
	}
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

func TestHasConfiguredScan(t *testing.T) {
	cmd := []string{"clamdscan", "{in}"}
	cases := []struct {
		name        string
		globalCmd   []string
		propCmd     []string
		propScan    ScanPolicy
		hasFileProp bool
		want        bool
	}{
		{"global command → scanned", cmd, nil, ScanDefault, true, true},
		{"property command → scanned", nil, cmd, ScanDefault, true, true},
		{"no command anywhere → not scanned", nil, nil, ScanDefault, true, false},
		{"global command but scan off → not scanned", cmd, nil, ScanOff, true, false},
		{"no file property → not scanned", cmd, nil, ScanDefault, false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m := fileMetaCmd(tc.globalCmd, tc.propCmd, tc.propScan, tc.hasFileProp)
			if got := NewAttachmentPolicy(m).HasConfiguredScan(); got != tc.want {
				t.Errorf("HasConfiguredScan = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestHasConfiguredScan_NotInverseOfUnconfigured pins the distinction the two
// methods exist for: they can BOTH be true, so neither can be derived from the
// other. Here one property carries its own scan command while a second has none
// and no global fallback — something is scanned, and something is unprotected.
//
// Deriving the startup check as !HasUnconfiguredScan() would conclude "nothing
// is scanned" and silence the broken-sandbox warning in exactly this mixed
// configuration, where uploads to the scanned property still get rejected.
func TestHasConfiguredScan_NotInverseOfUnconfigured(t *testing.T) {
	m := &Metamodel{
		Entities: map[string]EntityDef{
			"thing": {Properties: map[string]PropertyDef{
				"scanned":   {Type: PropertyTypeFile, ScanCmd: []string{"clamdscan", "{in}"}},
				"unscanned": {Type: PropertyTypeFile},
			}},
		},
	}
	p := NewAttachmentPolicy(m)
	if !p.HasConfiguredScan() {
		t.Error("HasConfiguredScan = false, want true (the `scanned` property is scanned)")
	}
	if !p.HasUnconfiguredScan() {
		t.Error("HasUnconfiguredScan = false, want true (the `unscanned` property has no scanner)")
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
