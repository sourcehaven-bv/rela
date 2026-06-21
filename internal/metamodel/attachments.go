package metamodel

import (
	"fmt"
	"strings"
)

// ScanPolicy is the tri-state virus-scan policy for attachments. It is a
// tri-state — not a bool — precisely so the loader can tell "the operator never
// said anything about scanning" (ScanUnset) from "the operator deliberately
// turned it off" (ScanOff). The first triggers the unset-scan startup warning;
// the second silences it. See [Metamodel.HasUnsetScanPolicy].
type ScanPolicy int

const (
	// ScanUnset means no scan policy was specified (the zero value). Treated as
	// off for enforcement, but surfaces the startup warning when file
	// properties exist.
	ScanUnset ScanPolicy = iota
	// ScanOff disables scanning explicitly (silences the warning).
	ScanOff
	// ScanRequired enforces a clean scan: the upload is rejected on a positive
	// result and, fail-closed, when the scan cannot run.
	ScanRequired
)

// String renders the policy for diagnostics.
func (s ScanPolicy) String() string {
	switch s {
	case ScanOff:
		return "off"
	case ScanRequired:
		return "required"
	default:
		return "unset"
	}
}

// UnmarshalYAML accepts the string forms `off` / `required` (case-insensitive).
// An absent key leaves the field at ScanUnset (the zero value), which is the
// whole point of the tri-state.
func (s *ScanPolicy) UnmarshalYAML(unmarshal func(any) error) error {
	var raw string
	if err := unmarshal(&raw); err != nil {
		return err
	}
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "unset":
		*s = ScanUnset
	case "off", "false", "no":
		*s = ScanOff
	case "required", "on", "true", "yes":
		*s = ScanRequired
	default:
		return fmt.Errorf("invalid scan policy %q (want \"off\" or \"required\")", raw)
	}
	return nil
}

// MarshalYAML renders the policy back to its string form, omitting the unset
// zero value so round-tripping a metamodel does not invent an explicit `off`.
func (s ScanPolicy) MarshalYAML() (any, error) {
	if s == ScanUnset {
		// yaml.v3 treats a nil node as "omit"; the nil error is intentional.
		return nil, nil //nolint:nilnil // (nil,nil) is the documented yaml omit signal
	}
	return s.String(), nil
}

// AttachmentsConfig is the top-level `attachments:` block: the global safety
// floor applied to every `file` property unless a property overrides it.
type AttachmentsConfig struct {
	// Allow names the MIME allowlist preset (e.g. "default-safe") or, when it
	// holds more than a preset name, an explicit list of allowed sniffed MIME
	// types. Empty means the built-in default-safe preset.
	Allow []string `yaml:"allow,omitempty"`

	// Scan is the global virus-scan policy, overridable per property.
	Scan ScanPolicy `yaml:"scan,omitempty"`

	// ScanCmd is the global external scan command (array args), used when a
	// `file` property requires scanning but defines no `scan_cmd` of its own.
	ScanCmd []string `yaml:"scan_cmd,omitempty"`
}

// EffectiveScanPolicy resolves the scan policy for a file property: the
// property's own policy when set (not unset), otherwise the global one.
func (m *Metamodel) EffectiveScanPolicy(prop PropertyDef) ScanPolicy {
	if prop.Scan != ScanUnset {
		return prop.Scan
	}
	if m.Attachments != nil {
		return m.Attachments.Scan
	}
	return ScanUnset
}

// HasUnsetScanPolicy reports whether the metamodel declares at least one
// `file`-type property while leaving the scan policy unset at both the global
// and that-property level — i.e. the operator never made a conscious choice.
// The composition root uses this to emit a single startup warning.
func (m *Metamodel) HasUnsetScanPolicy() bool {
	globalSet := m.Attachments != nil && m.Attachments.Scan != ScanUnset
	for _, def := range m.Entities {
		for _, prop := range def.Properties {
			if prop.Type != PropertyTypeFile {
				continue
			}
			if !globalSet && prop.Scan == ScanUnset {
				return true
			}
		}
	}
	return false
}
