package dataentryconfig

import (
	"encoding/json"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestChromeToggle_UnmarshalYAML(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		want    ChromeToggle
		wantErr string // substring; "" means expect success
	}{
		{name: "true", yaml: "v: true", want: ChromeToggle{}},
		{name: "false", yaml: "v: false", want: ChromeToggle{Off: true}},
		{name: "permission", yaml: "v: {permission: staff.manage}", want: ChromeToggle{Permission: "staff.manage"}},
		{name: "absent", yaml: "other: 1", want: ChromeToggle{}},

		{name: "yes is not a boolean", yaml: "v: yes", wantErr: `line 1: "yes" is not a boolean`},
		{name: "string", yaml: "v: hidden", wantErr: "must be true, false or {permission: <name>}"},
		{name: "list", yaml: "v: [a]", wantErr: "must be true, false or {permission: <name>}"},
		{name: "unknown key", yaml: "v: {role: staff}", wantErr: `unknown key "role"`},
		{name: "empty mapping", yaml: "v: {}", wantErr: "permission is required"},
		{name: "empty permission", yaml: `v: {permission: ""}`, wantErr: "permission must be a permission name"},
		{name: "permission with spaces", yaml: `v: {permission: " staff.manage "}`,
			wantErr: "without surrounding spaces"},
		{name: "permission not a string", yaml: "v: {permission: [a]}", wantErr: "permission must be a permission name"},
		{name: "error names the line", yaml: "a: 1\nv: hidden", wantErr: "line 2: must be true"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var got struct {
				V ChromeToggle `yaml:"v"`
			}
			err := yaml.Unmarshal([]byte(tc.yaml), &got)
			if tc.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.V != tc.want {
				t.Errorf("got %+v, want %+v", got.V, tc.want)
			}
		})
	}
}

func TestChromeToggle_RoundTrip(t *testing.T) {
	cases := []struct {
		name     string
		toggle   ChromeToggle
		wantJSON string
	}{
		{"shown", ChromeToggle{}, "true"},
		{"off", ChromeToggle{Off: true}, "false"},
		{"permission", ChromeToggle{Permission: "staff.manage"}, `{"permission":"staff.manage"}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			b, err := json.Marshal(tc.toggle)
			if err != nil {
				t.Fatalf("marshal json: %v", err)
			}
			if string(b) != tc.wantJSON {
				t.Errorf("json = %s, want %s", b, tc.wantJSON)
			}
			var fromJSON ChromeToggle
			if jerr := json.Unmarshal(b, &fromJSON); jerr != nil || fromJSON != tc.toggle {
				t.Errorf("json round trip = %+v (%v), want %+v", fromJSON, jerr, tc.toggle)
			}
			y, err := yaml.Marshal(tc.toggle)
			if err != nil {
				t.Fatalf("marshal yaml: %v", err)
			}
			var fromYAML ChromeToggle
			if err := yaml.Unmarshal(y, &fromYAML); err != nil || fromYAML != tc.toggle {
				t.Errorf("yaml round trip of %q = %+v (%v), want %+v", y, fromYAML, err, tc.toggle)
			}
		})
	}
}

// A JSON null is the default, as an absent YAML value is.
func TestChromeToggle_JSONNullIsDefault(t *testing.T) {
	toggle := ChromeToggle{Permission: "kept"}
	if err := json.Unmarshal([]byte("null"), &toggle); err != nil {
		t.Fatalf("unmarshal null: %v", err)
	}
	if toggle != (ChromeToggle{Permission: "kept"}) {
		t.Errorf("null changed the toggle to %+v", toggle)
	}
	var cfg DashboardConfig
	if err := json.Unmarshal([]byte(`{"validation": null}`), &cfg); err != nil || !cfg.Validation.IsZero() {
		t.Errorf("validation null = %+v (%v), want the default", cfg.Validation, err)
	}
}

// An unset toggle is left out of the served config, so a config that does
// not use the key serializes exactly as before.
func TestChromeToggle_OmittedWhenUnset(t *testing.T) {
	b, err := json.Marshal(DashboardConfig{Title: "Home"})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(b), "validation") {
		t.Errorf("unset validation serialized: %s", b)
	}
	y, err := yaml.Marshal(struct {
		HeaderButtons HeaderButtonsConfig `yaml:"header_buttons,omitempty"`
	}{})
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}
	if strings.Contains(string(y), "header_buttons") {
		t.Errorf("unset header_buttons serialized: %s", y)
	}
	y, err = yaml.Marshal(struct {
		ListButtons ListButtonsConfig `yaml:"list_buttons,omitempty"`
	}{})
	if err != nil {
		t.Fatalf("marshal yaml: %v", err)
	}
	if strings.Contains(string(y), "list_buttons") {
		t.Errorf("unset list_buttons serialized: %s", y)
	}
}

func TestChromeToggle_Shown(t *testing.T) {
	holds := func(perm string) bool { return perm == "staff.manage" }
	cases := []struct {
		name   string
		toggle ChromeToggle
		want   bool
	}{
		{"default", ChromeToggle{}, true},
		{"off", ChromeToggle{Off: true}, false},
		{"held permission", ChromeToggle{Permission: "staff.manage"}, true},
		{"missing permission", ChromeToggle{Permission: "admin"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.toggle.Shown(holds); got != tc.want {
				t.Errorf("Shown = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestValidateConfig_PageChrome(t *testing.T) {
	cases := []struct {
		name    string
		yaml    string
		wantErr string // substring; "" means expect success
	}{
		{"absent", ``, ""},
		{"dashboard validation off", "dashboard: {title: Home, validation: false}", ""},
		{"dashboard validation gated", "dashboard: {title: Home, validation: {permission: staff.manage}}", ""},
		{"named dashboard validation off", "dashboards: {portal: {title: Portal, validation: false}}", ""},
		{"entity page header", "header_buttons: {history: false, add_to_pile: {permission: staff.manage}}", ""},
		{"empty entity page header", "header_buttons: {}", ""},
		{"entity page header unknown key", "header_buttons: {histroy: false}", `header_buttons: unknown key "histroy"`},
		{"list buttons", "list_buttons: {add_to_pile: {permission: staff.manage}}", ""},
		{"list buttons unknown key", "list_buttons: {add_to_piles: false}", `list_buttons: unknown key "add_to_piles"`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := []byte("version: \"1.0\"\n" + tc.yaml)
			var cfg Config
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			err := ValidateConfig(data, &cfg, testMetamodel())
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error = %v, want it to contain %q", err, tc.wantErr)
			}
		})
	}
}
