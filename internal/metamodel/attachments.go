package metamodel

import (
	"fmt"
	"strings"
)

// ScanPolicy is the per-scope virus-scan switch for attachments. Scanning is
// driven by the *presence of a scan command*: if a `scan_cmd` is configured
// (globally or on the property) the upload is scanned, fail-closed. There is no
// separate "required" level — configuring a scanner is the intent to use it.
//
// ScanPolicy exists only as an OPT-OUT: a property may set `scan: off` to skip
// scanning despite a global `scan_cmd`. The zero value (ScanDefault) means
// "scan when a command is configured."
type ScanPolicy int

const (
	// ScanDefault (the zero value) means scan iff a scan command is configured
	// for this scope. No explicit policy was set.
	ScanDefault ScanPolicy = iota
	// ScanOff disables scanning for this property, even when a global scan
	// command exists.
	ScanOff
)

// String renders the policy for diagnostics.
func (s ScanPolicy) String() string {
	if s == ScanOff {
		return "off"
	}
	return "default"
}

// UnmarshalYAML accepts the string `off` (case-insensitive). `required`/`on`
// are accepted as no-ops for forgiveness (scanning is already implied by
// configuring a command), mapping to ScanDefault. An absent key leaves
// ScanDefault.
func (s *ScanPolicy) UnmarshalYAML(unmarshal func(any) error) error {
	var raw string
	if err := unmarshal(&raw); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "default", "on", "required", "true", "yes":
		*s = ScanDefault
	case "off", "false", "no":
		*s = ScanOff
	default:
		return fmt.Errorf("invalid scan value %q (want \"off\")", raw)
	}
	return nil
}

// MarshalYAML renders only the explicit `off`; the default is omitted so
// round-tripping a metamodel does not invent a key.
func (s ScanPolicy) MarshalYAML() (any, error) {
	if s == ScanOff {
		return "off", nil
	}
	return nil, nil //nolint:nilnil // (nil,nil) is the documented yaml omit signal
}

// AttachmentsConfig is the top-level `attachments:` block: the global safety
// floor applied to every `file` property unless a property overrides it.
type AttachmentsConfig struct {
	// Allow names the MIME allowlist preset (e.g. "default-safe") or, when it
	// holds more than a preset name, an explicit list of allowed sniffed MIME
	// types. Empty means the built-in default-safe preset.
	Allow []string `yaml:"allow,omitempty"`

	// ScanCmd is the global external scan command (array args). Its presence
	// enables scanning for every `file` property that does not opt out with
	// `scan: off`.
	ScanCmd []string `yaml:"scan_cmd,omitempty"`
}

// ScanCommandFor resolves the scan command that should run for a file property,
// or nil when the property is not scanned. Scanning runs when a command is
// configured (property-level wins over global) and the property has not opted
// out with `scan: off`.
func (m *Metamodel) ScanCommandFor(prop PropertyDef) []string {
	if prop.Scan == ScanOff {
		return nil
	}
	if len(prop.ScanCmd) > 0 {
		return prop.ScanCmd
	}
	if m.Attachments != nil && len(m.Attachments.ScanCmd) > 0 {
		return m.Attachments.ScanCmd
	}
	return nil
}

// HasUnconfiguredScan reports whether the metamodel declares at least one
// `file`-type property while no scan command is configured for it (no global
// command, no property command) and it has not explicitly opted out with
// `scan: off`. The composition root uses this to emit a single startup warning
// nudging the operator to wire a scanner or explicitly disable scanning.
func (m *Metamodel) HasUnconfiguredScan() bool {
	globalCmd := m.Attachments != nil && len(m.Attachments.ScanCmd) > 0
	for _, def := range m.Entities {
		for _, prop := range def.Properties {
			if prop.Type != PropertyTypeFile {
				continue
			}
			if prop.Scan == ScanOff {
				continue // explicitly opted out — a conscious choice
			}
			if len(prop.ScanCmd) == 0 && !globalCmd {
				return true
			}
		}
	}
	return false
}
