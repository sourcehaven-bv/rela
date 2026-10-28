package dataentryconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"gopkg.in/yaml.v3"
)

// DefaultDashboardName is the name that refers to the default dashboard, the
// one under the top-level `dashboard:` key. It is reserved: `dashboards:` may
// not declare an entry with this name, and `dashboard: default` on a
// navigation entry, page tab, command scope or space home means the same as
// `dashboard: true`.
const DefaultDashboardName = "default"

// DashboardRef is the value of a `dashboard:` key on a navigation entry, a
// page tab, a command scope or a space home. It is written as `true` for the
// default dashboard or as the name of an entry under `dashboards:`.
//
// The zero value means "no dashboard". [DefaultDashboard] is the default
// dashboard; any other value names an entry under `dashboards:`.
type DashboardRef string

// errDashboardRefShape is the error for a `dashboard:` value that is neither
// a boolean nor a name.
var errDashboardRefShape = errors.New("dashboard must be true or the name of a dashboard under `dashboards`")

// DefaultDashboard refers to the default dashboard.
const DefaultDashboard DashboardRef = DefaultDashboardName

// IsSet reports whether the ref names a dashboard at all.
func (d DashboardRef) IsSet() bool { return d != "" }

// IsDefault reports whether the ref names the default dashboard.
func (d DashboardRef) IsDefault() bool { return d == DefaultDashboard }

// Name returns the name under `dashboards:`, or "" for the default dashboard
// and for an unset ref. It is the value the `?name=` query parameter and the
// `/dashboard/<name>` route carry.
func (d DashboardRef) Name() string {
	if d.IsDefault() {
		return ""
	}
	return string(d)
}

// UnmarshalYAML accepts `dashboard: true|false` and `dashboard: <name>`.
func (d *DashboardRef) UnmarshalYAML(value *yaml.Node) error {
	if value.Kind != yaml.ScalarNode {
		return errDashboardRefShape
	}
	if value.Tag == "!!bool" {
		var b bool
		if err := value.Decode(&b); err != nil {
			return err
		}
		*d = ""
		if b {
			*d = DefaultDashboard
		}
		return nil
	}
	if value.Tag != "!!str" || value.Value == "" {
		return errDashboardRefShape
	}
	// YAML 1.2 reads an unquoted yes/no/on/off as a string, so `dashboard: yes`
	// would silently name a dashboard called "yes". Refuse it, as `confirm:` does.
	if value.Style&(yaml.DoubleQuotedStyle|yaml.SingleQuotedStyle) == 0 && isYAML11Bool(value.Value) {
		return fmt.Errorf("dashboard: %q is not a boolean; use true or false, or quote it to name a dashboard",
			value.Value)
	}
	*d = DashboardRef(value.Value)
	return nil
}

// UnmarshalYAML reads `columns:` by its shape. A scalar names a matrix card's
// column property (MatrixColumns); a sequence is a table card's column list
// (Columns). Both keep the key the operator expects for the display.
func (c *DashboardCard) UnmarshalYAML(value *yaml.Node) error {
	type plain DashboardCard
	node := value
	var matrixColumns string
	if value.Kind == yaml.MappingNode {
		copied := *value
		copied.Content = make([]*yaml.Node, 0, len(value.Content))
		for i := 0; i+1 < len(value.Content); i += 2 {
			key, val := value.Content[i], value.Content[i+1]
			if key.Value == "columns" && val.Kind == yaml.ScalarNode {
				matrixColumns = val.Value
				continue
			}
			copied.Content = append(copied.Content, key, val)
		}
		node = &copied
	}
	var p plain
	if err := node.Decode(&p); err != nil {
		return err
	}
	*c = DashboardCard(p)
	c.MatrixColumns = matrixColumns
	return nil
}

// MarshalYAML writes the same shape UnmarshalYAML reads.
func (d DashboardRef) MarshalYAML() (any, error) {
	if d.IsDefault() {
		return true, nil
	}
	if !d.IsSet() {
		return false, nil
	}
	return string(d), nil
}

// MarshalJSON serves `true` for the default dashboard and the name for a
// named one, so a client that only knows the boolean form keeps working for
// every config written before named dashboards existed.
func (d DashboardRef) MarshalJSON() ([]byte, error) {
	if d.IsDefault() {
		return json.Marshal(true)
	}
	if !d.IsSet() {
		return json.Marshal(false)
	}
	return json.Marshal(string(d))
}

// UnmarshalJSON reads what MarshalJSON writes.
func (d *DashboardRef) UnmarshalJSON(data []byte) error {
	var b bool
	if err := json.Unmarshal(data, &b); err == nil {
		*d = ""
		if b {
			*d = DefaultDashboard
		}
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return errors.New("dashboard must be true or the name of a dashboard")
	}
	*d = DashboardRef(s)
	return nil
}

// DashboardByName returns a dashboard: the default one for "" (or
// [DefaultDashboardName]), else the entry of that name under `dashboards:`.
// ok is false for an unknown name. For the default dashboard ok is true even
// when no `dashboard:` is configured; the returned config is then nil, which
// renders as an empty dashboard, as it always has.
func (c *Config) DashboardByName(name string) (cfg *DashboardConfig, ok bool) {
	if c == nil {
		return nil, name == "" || name == DefaultDashboardName
	}
	if name == "" || name == DefaultDashboardName {
		return c.Dashboard, true
	}
	d, ok := c.Dashboards[name]
	if !ok {
		return nil, false
	}
	return &d, true
}

// NamedDashboard is one configured dashboard, with the name it is reached
// by ("" for the default dashboard).
type NamedDashboard struct {
	Name   string
	Config *DashboardConfig
}

// AllDashboards returns every configured dashboard: the default one first
// (when configured), then the named ones in name order. For code that walks
// every card, such as the ACL audit and the query planner.
func (c *Config) AllDashboards() []NamedDashboard {
	if c == nil {
		return nil
	}
	var out []NamedDashboard
	if c.Dashboard != nil {
		out = append(out, NamedDashboard{Config: c.Dashboard})
	}
	for _, name := range slices.Sorted(maps.Keys(c.Dashboards)) {
		d := c.Dashboards[name]
		out = append(out, NamedDashboard{Name: name, Config: &d})
	}
	return out
}

// dashboardRefError checks that a `dashboard:` ref names a dashboard that
// exists. The default dashboard always resolves, configured or not, so that
// `dashboard: true` without a `dashboard:` block keeps rendering the empty
// dashboard it always rendered. The message carries no prefix.
func dashboardRefError(ref DashboardRef, cfg *Config) string {
	if !ref.IsSet() || ref.IsDefault() {
		return ""
	}
	if _, ok := cfg.Dashboards[string(ref)]; ok {
		return ""
	}
	if len(cfg.Dashboards) == 0 {
		return fmt.Sprintf("references unknown dashboard %q (no dashboards: are configured)", string(ref))
	}
	return fmt.Sprintf("references unknown dashboard %q (known: %s)",
		string(ref), joinSorted(slices.Collect(maps.Keys(cfg.Dashboards))))
}

func joinSorted(names []string) string {
	slices.Sort(names)
	return strings.Join(names, ", ")
}
