package dataentryconfig

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// loadAndValidate parses data-entry YAML and validates it, the two steps a
// config load takes. A parse error is returned as is.
func loadAndValidate(t *testing.T, src string) (*Config, error) {
	t.Helper()
	var cfg Config
	if err := yaml.Unmarshal([]byte(src), &cfg); err != nil {
		return nil, err
	}
	return &cfg, ValidateConfig([]byte(src), &cfg, testMetamodel())
}

const namedDashboardsYAML = `
version: "1.0"
lists:
  all: {entity_type: ticket}
dashboard:
  title: Overview
  cards:
    - {title: All, query: "type:ticket", display: count}
dashboards:
  crm:
    title: Pipeline
    cards:
      - {title: Open, query: "type:ticket", display: count}
pages:
  sales:
    label: Sales
    tabs:
      - {id: pipeline, label: Pipeline, dashboard: crm}
      - {id: home, label: Home, dashboard: true}
commands:
  refresh:
    label: Refresh
    script: refresh.lua
    context: global
    available_on: {dashboard: crm}
spaces:
  - id: crm
    label: CRM
    home: {dashboard: crm}
    navigation:
      - {label: Pipeline, dashboard: crm}
      - {label: Home, dashboard: default}
      - {label: Sales, page: sales}
  - id: main
    label: Main
    home: {dashboard: true}
    navigation:
      - {label: All, list: all}
`

func TestNamedDashboards_LoadAndResolve(t *testing.T) {
	cfg, err := loadAndValidate(t, namedDashboardsYAML)
	require.NoError(t, err)

	require.Equal(t, DashboardRef("crm"), cfg.Spaces[0].Home.Dashboard)
	require.Equal(t, DefaultDashboard, cfg.Spaces[1].Home.Dashboard)
	require.Equal(t, DefaultDashboard, cfg.Spaces[0].Navigation[1].Dashboard,
		"dashboard: default is an alias for true")
	require.Equal(t, DashboardRef("crm"), cfg.Pages["sales"].Tabs[0].Dashboard)
	require.Equal(t, DashboardRef("crm"), cfg.Commands["refresh"].AvailableOn.Dashboard)

	crm, ok := cfg.DashboardByName("crm")
	require.True(t, ok)
	require.Equal(t, "Pipeline", crm.Title)
	def, ok := cfg.DashboardByName("")
	require.True(t, ok)
	require.Equal(t, "Overview", def.Title)
	def, ok = cfg.DashboardByName(DefaultDashboardName)
	require.True(t, ok)
	require.Equal(t, "Overview", def.Title)
	_, ok = cfg.DashboardByName("nope")
	require.False(t, ok)

	all := cfg.AllDashboards()
	require.Len(t, all, 2)
	require.Empty(t, all[0].Name)
	require.Equal(t, "crm", all[1].Name)
}

// The default dashboard resolves even when none is configured, so that
// `dashboard: true` keeps rendering an empty dashboard.
func TestNamedDashboards_DefaultWithoutBlock(t *testing.T) {
	cfg, err := loadAndValidate(t, `
version: "1.0"
navigation:
  - {label: Home, dashboard: true}
`)
	require.NoError(t, err)
	d, ok := cfg.DashboardByName("")
	require.True(t, ok)
	require.Nil(t, d)
	require.Empty(t, cfg.AllDashboards())
}

func TestNamedDashboards_ValidationErrors(t *testing.T) {
	cases := []struct {
		name string
		yaml string
		want string
	}{
		{
			name: "unknown name in navigation",
			yaml: "navigation:\n  - {label: X, dashboard: nope}\n",
			want: `navigation: references unknown dashboard "nope" (no dashboards: are configured)`,
		},
		{
			name: "unknown name lists the known ones",
			yaml: "dashboards:\n  crm: {title: C, cards: []}\n  ops: {title: O, cards: []}\n" +
				"navigation:\n  - {label: X, dashboard: sales}\n",
			want: `navigation: references unknown dashboard "sales" (known: crm, ops)`,
		},
		{
			name: "unknown name in a space home",
			yaml: "spaces:\n  - id: crm\n    label: CRM\n    home: {dashboard: nope}\n",
			want: `unknown dashboard "nope"`,
		},
		{
			name: "unknown name in a page tab",
			yaml: "pages:\n  p:\n    label: P\n    tabs:\n      - {id: t, label: T, dashboard: nope}\n",
			want: `pages[p].tabs[t]: references unknown dashboard "nope"`,
		},
		{
			name: "unknown name in a command scope",
			yaml: "commands:\n  c:\n    label: C\n    script: c.lua\n    context: global\n" +
				"    available_on: {dashboard: nope}\n",
			want: `command "c": available_on references unknown dashboard "nope"`,
		},
		{
			name: "default is reserved",
			yaml: "dashboards:\n  default: {title: D, cards: []}\n",
			want: `dashboards: the name "default" is reserved`,
		},
		{
			name: "invalid name",
			yaml: "dashboards:\n  CRM: {title: D, cards: []}\n",
			want: `dashboards: name "CRM" is invalid`,
		},
		{
			name: "card errors name the dashboard",
			yaml: "dashboards:\n  crm:\n    title: C\n    cards:\n      - {title: T, query: q, display: pie}\n",
			want: `dashboards[crm]: card[0] "T" has invalid display mode "pie"`,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := loadAndValidate(t, "version: \"1.0\"\n"+tc.yaml)
			require.Error(t, err)
			require.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestDashboardRef_YAML(t *testing.T) {
	cases := []struct {
		src  string
		want DashboardRef
		err  string
	}{
		{src: "dashboard: true", want: DefaultDashboard},
		{src: "dashboard: false", want: ""},
		{src: "dashboard: crm", want: "crm"},
		{src: "dashboard: default", want: DefaultDashboard},
		{src: `dashboard: "yes"`, want: "yes"},
		{src: "dashboard: yes", err: "is not a boolean"},
		{src: "dashboard: |\n  yes\n", err: `dashboard: "yes" is not a boolean`},
		{src: "dashboard: !!str yes", want: "yes"},
		{src: `dashboard: ""`, err: "must be true or the name"},
		{src: "dashboard: 3", err: "must be true or the name"},
		{src: "dashboard: [a]", err: "must be true or the name"},
	}
	for _, tc := range cases {
		t.Run(tc.src, func(t *testing.T) {
			var nav NavigationEntry
			err := yaml.Unmarshal([]byte(tc.src), &nav)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, nav.Dashboard)
		})
	}
}

// The wire form is true for the default dashboard and the name otherwise,
// and both forms read back.
func TestDashboardRef_RoundTrip(t *testing.T) {
	for _, ref := range []DashboardRef{DefaultDashboard, "crm"} {
		t.Run(string(ref), func(t *testing.T) {
			b, err := json.Marshal(NavigationEntry{Label: "X", Dashboard: ref})
			require.NoError(t, err)
			if ref.IsDefault() {
				require.Contains(t, string(b), `"dashboard":true`)
			} else {
				require.Contains(t, string(b), `"dashboard":"crm"`)
			}
			var back NavigationEntry
			require.NoError(t, json.Unmarshal(b, &back))
			require.Equal(t, ref, back.Dashboard)

			y, err := yaml.Marshal(NavigationEntry{Label: "X", Dashboard: ref})
			require.NoError(t, err)
			var yback NavigationEntry
			require.NoError(t, yaml.Unmarshal(y, &yback))
			require.Equal(t, ref, yback.Dashboard)
		})
	}

	b, err := json.Marshal(NavigationEntry{Label: "X"})
	require.NoError(t, err)
	require.NotContains(t, string(b), "dashboard", "an unset ref is omitted")

	var off NavigationEntry
	require.NoError(t, json.Unmarshal([]byte(`{"dashboard":false}`), &off))
	require.False(t, off.Dashboard.IsSet())
	require.Error(t, json.Unmarshal([]byte(`{"dashboard":3}`), &off))
}

func TestDashboardRef_Name(t *testing.T) {
	require.Empty(t, DefaultDashboard.Name())
	require.Empty(t, DashboardRef("").Name())
	require.Equal(t, "crm", DashboardRef("crm").Name())
	require.False(t, DashboardRef("").IsSet())
}
