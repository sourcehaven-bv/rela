package metamodel

import (
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"
)

// RecordTypeDef is one entry of the top-level `records:` block: an
// append-only stream of typed rows attached to an owner entity and stored
// outside the graph (TKT-J7Q9C4K). Policy only: the rows live in the record
// store, see internal/records.
type RecordTypeDef struct {
	// Label is the display name. Empty means the key.
	Label string `yaml:"label,omitempty"`
	// Description is documentation for the record type.
	Description string `yaml:"description,omitempty"`
	// Owner is the entity type every row attaches to.
	Owner string `yaml:"owner"`
	// Fields are the typed columns of a row. A field is a [PropertyDef]
	// restricted to the scalar types [RecordFieldTypeAllowed] accepts.
	Fields map[string]PropertyDef `yaml:"fields"`
	// Retention drops rows whose timestamp is older than this, such as
	// "90d", "2w" or "5y". Empty keeps rows for ever.
	Retention string `yaml:"retention,omitempty"`
	// Ledger makes the type a ledger: rows are appended in balanced
	// transactions and corrected only by reversal.
	Ledger *LedgerDef `yaml:"ledger,omitempty"`
}

// LedgerDef is the `ledger:` block of a record type.
type LedgerDef struct {
	// Amount names the decimal or integer field that must balance.
	Amount string `json:"amount" yaml:"amount"`
	// GroupBy names the string field that groups rows into one
	// transaction. The rows of a transaction sum to zero.
	GroupBy string `json:"group_by" yaml:"group_by"`
	// Commodity optionally names a field the zero sum is taken per value
	// of, such as a currency.
	Commodity string `json:"commodity,omitempty" yaml:"commodity,omitempty"`
	// ClosedBefore optionally names a date property of the owner type. A
	// row dated on or before the owner's value of it is refused.
	ClosedBefore string `json:"closed_before,omitempty" yaml:"closed_before,omitempty"`
}

// recordReservedFields are the names a record row carries itself, so no
// declared field may take them.
var recordReservedFields = []string{"id", "type", "owner", "ts", "recorded_at", "recorded_by", "group", "reverses"}

// RecordKeys returns the declared record type keys, sorted.
func (m *Metamodel) RecordKeys() []string {
	keys := make([]string, 0, len(m.Records))
	for k := range m.Records {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// FieldNames returns the declared field names, sorted.
func (d RecordTypeDef) FieldNames() []string {
	names := make([]string, 0, len(d.Fields))
	for k := range d.Fields {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}

// RetentionDuration returns the parsed retention, zero for none. An invalid
// value is zero: validateRecords rejects it at load.
func (d RecordTypeDef) RetentionDuration() time.Duration {
	r, _ := ParseRetention(d.Retention)
	return r
}

// recordDurationUnits are the units [ParseRetention] accepts.
var recordDurationUnits = map[string]time.Duration{
	"h": time.Hour, "d": 24 * time.Hour, "w": 7 * 24 * time.Hour, "y": 365 * 24 * time.Hour,
}

// ParseRetention parses a record retention: a whole number of hours, days,
// weeks or years, such as "48h", "90d", "2w" or "5y". Empty is zero.
func ParseRetention(s string) (time.Duration, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, nil
	}
	for unit, size := range recordDurationUnits {
		if num, ok := strings.CutSuffix(s, unit); ok {
			n, err := strconv.Atoi(num)
			if err != nil || n <= 0 {
				break
			}
			return time.Duration(n) * size, nil
		}
	}
	return 0, fmt.Errorf("%q is not a retention such as 48h, 90d, 2w or 5y", s)
}

// RecordFieldTypeAllowed reports whether a record field may have type t:
// the scalar built-ins, and custom enum types.
func (m *Metamodel) RecordFieldTypeAllowed(p PropertyDef) bool {
	switch p.Type {
	case PropertyTypeString, PropertyTypeInteger, PropertyTypeDecimal, PropertyTypeDate,
		PropertyTypeDatetime, PropertyTypeBoolean:
		return true
	case PropertyTypeEnum:
		return len(p.Values) > 0
	}
	ct, ok := m.Types[p.Type]
	return ok && len(ct.Values) > 0
}

// validateRecords checks the top-level `records:` block.
func validateRecords(m *Metamodel) []string {
	var errs []string
	for _, key := range m.RecordKeys() {
		d := m.Records[key]
		at := "records." + key
		if !pileKeyPattern.MatchString(key) {
			errs = append(errs, at+": a record type key is lowercase letters, digits, - and _, starting with a letter")
		}
		if _, clash := m.Entities[key]; clash {
			errs = append(errs, at+": the key is also an entity type")
		}
		if d.Owner == "" {
			errs = append(errs, at+": owner is required")
		} else if _, ok := m.Entities[d.Owner]; !ok {
			errs = append(errs, fmt.Sprintf("%s: owner %q is not an entity type", at, d.Owner))
		}
		if len(d.Fields) == 0 {
			errs = append(errs, at+": at least one field is required")
		}
		for _, name := range d.FieldNames() {
			errs = append(errs, validateRecordField(m, at+".fields."+name, name, d.Fields[name])...)
		}
		if _, err := ParseRetention(d.Retention); err != nil {
			errs = append(errs, fmt.Sprintf("%s: retention: %v", at, err))
		}
		if d.Ledger != nil {
			errs = append(errs, validateLedger(m, at+".ledger", d)...)
		}
	}
	return errs
}

func validateRecordField(m *Metamodel, at, name string, p PropertyDef) []string {
	var errs []string
	if !pileKeyPattern.MatchString(name) {
		errs = append(errs, at+": a field name is lowercase letters, digits, - and _, starting with a letter")
	}
	if slices.Contains(recordReservedFields, name) {
		errs = append(errs, fmt.Sprintf("%s: %q is reserved", at, name))
	}
	if !m.RecordFieldTypeAllowed(p) {
		errs = append(errs, fmt.Sprintf("%s: type %q is not allowed; use string, integer, decimal, date, "+
			"datetime, boolean or an enum", at, p.Type))
	}
	if p.List || p.Computed != "" || p.Rollup != nil || p.Unique {
		errs = append(errs, at+": list, computed, rollup and unique are not supported on record fields")
	}
	return errs
}

func validateLedger(m *Metamodel, at string, d RecordTypeDef) []string {
	var errs []string
	field := func(key, name string, check func(PropertyDef) bool, want string) {
		if name == "" {
			errs = append(errs, fmt.Sprintf("%s: %s is required", at, key))
			return
		}
		p, ok := d.Fields[name]
		switch {
		case !ok:
			errs = append(errs, fmt.Sprintf("%s: %s names %q, which is not a field", at, key, name))
		case !check(p):
			errs = append(errs, fmt.Sprintf("%s: %s field %q must be %s", at, key, name, want))
		}
	}
	field("amount", d.Ledger.Amount, func(p PropertyDef) bool {
		return p.Required && (p.Type == PropertyTypeDecimal || p.Type == PropertyTypeInteger)
	}, "a required decimal or integer")
	field("group_by", d.Ledger.GroupBy, func(p PropertyDef) bool {
		return p.Required && p.Type == PropertyTypeString
	}, "a required string")
	if d.Retention != "" {
		// Retention would drop old legs and leave balances that no longer
		// add up.
		errs = append(errs, at+": a ledger cannot have a retention")
	}
	if d.Ledger.Commodity != "" {
		field("commodity", d.Ledger.Commodity, func(p PropertyDef) bool { return p.Required }, "required")
	}
	if name := d.Ledger.ClosedBefore; name != "" {
		owner, ok := m.Entities[d.Owner]
		if ok {
			p, has := owner.Properties[name]
			if !has || p.Type != PropertyTypeDate {
				errs = append(errs, fmt.Sprintf("%s: closed_before %q must be a date property of %s", at, name, d.Owner))
			}
		}
	}
	return errs
}
