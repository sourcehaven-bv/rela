package dataentryconfig

import (
	"strings"
	"testing"
	"time"
)

func TestParseNextActionDuration(t *testing.T) {
	tests := []struct {
		in      string
		want    time.Duration
		wantErr bool
	}{
		{in: "3d", want: 72 * time.Hour},
		{in: "1d", want: 24 * time.Hour},
		{in: "0.5d", want: 12 * time.Hour},
		{in: "12h", want: 12 * time.Hour},
		{in: "90m", want: 90 * time.Minute},
		{in: "1h30m", want: 90 * time.Minute},
		{in: "", wantErr: true},
		{in: "d", wantErr: true},
		{in: "3x", wantErr: true},
		{in: "banana", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got, err := ParseNextActionDuration(tc.in)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ParseNextActionDuration(%q) = %v, want error", tc.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("ParseNextActionDuration(%q): %v", tc.in, err)
			}
			if got != tc.want {
				t.Errorf("ParseNextActionDuration(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestNextActionOffer_Kind(t *testing.T) {
	tests := []struct {
		name  string
		offer NextActionOffer
		want  string
	}{
		{"action", NextActionOffer{Action: "mark-done"}, "action"},
		{"set", NextActionOffer{Set: map[string]string{"status": "done"}}, "set"},
		{"navigate", NextActionOffer{Navigate: "/entity/task/{id}"}, "navigate"},
		{"snooze", NextActionOffer{Snooze: []string{"1d"}}, "snooze"},
		{"dismiss", NextActionOffer{Dismiss: true}, "dismiss"},
		{"acknowledge", NextActionOffer{Acknowledge: true}, "acknowledge"},
		{"empty", NextActionOffer{}, ""},
		{"label alone is not a kind", NextActionOffer{Label: "x"}, ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.offer.Kind(); got != tc.want {
				t.Errorf("Kind() = %q, want %q", got, tc.want)
			}
		})
	}
}

// validNextActionConfig is the baseline each validation test perturbs, so a
// test asserts one failure rather than a pile of unrelated ones.
func validNextActionConfig() *Config {
	return &Config{
		Actions: map[string]Action{
			"mark-done": {Label: "Mark done"},
		},
		NextActionBands: []NextActionBand{
			{ID: "blocking"},
			{ID: "ambient"},
		},
		NextActions: map[string]NextActionSource{
			"quip": {
				Band:    "ambient",
				Query:   "type:quip",
				Suggest: "{text}",
				Actions: []NextActionOffer{{Acknowledge: true}},
			},
		},
	}
}

func TestValidateNextActions_Valid(t *testing.T) {
	if errs := validateNextActions(validNextActionConfig(), nil); len(errs) > 0 {
		t.Fatalf("valid config reported errors: %v", errs)
	}
}

func TestValidateNextActions_Errors(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*Config)
		wantSub string
	}{
		{
			name:    "unknown band",
			mutate:  func(c *Config) { s := c.NextActions["quip"]; s.Band = "nope"; c.NextActions["quip"] = s },
			wantSub: `unknown band "nope"`,
		},
		{
			// Near-miss casing is the likeliest real typo, so it earns a
			// "did you mean" rather than a bare rejection.
			name:    "unknown band suggests near miss",
			mutate:  func(c *Config) { s := c.NextActions["quip"]; s.Band = "Ambient"; c.NextActions["quip"] = s },
			wantSub: `did you mean "ambient"`,
		},
		{
			name:    "missing band",
			mutate:  func(c *Config) { s := c.NextActions["quip"]; s.Band = ""; c.NextActions["quip"] = s },
			wantSub: "band is required",
		},
		{
			name:    "missing suggest",
			mutate:  func(c *Config) { s := c.NextActions["quip"]; s.Suggest = ""; c.NextActions["quip"] = s },
			wantSub: "suggest is required",
		},
		{
			name:    "no candidate source",
			mutate:  func(c *Config) { s := c.NextActions["quip"]; s.Query = ""; c.NextActions["quip"] = s },
			wantSub: "needs one of query",
		},
		{
			name:    "query and context are exclusive",
			mutate:  func(c *Config) { s := c.NextActions["quip"]; s.Context = "task"; c.NextActions["quip"] = s },
			wantSub: "mutually exclusive",
		},
		{
			name:    "duplicate band id",
			mutate:  func(c *Config) { c.NextActionBands = append(c.NextActionBands, NextActionBand{ID: "ambient"}) },
			wantSub: `duplicate band id "ambient"`,
		},
		{
			name:    "empty band id",
			mutate:  func(c *Config) { c.NextActionBands = append(c.NextActionBands, NextActionBand{}) },
			wantSub: "id is required",
		},
		{
			name:    "sources without bands",
			mutate:  func(c *Config) { c.NextActionBands = nil },
			wantSub: "no next_action_bands declared",
		},
		{
			name:    "unknown pick",
			mutate:  func(c *Config) { s := c.NextActions["quip"]; s.Pick = "vibes"; c.NextActions["quip"] = s },
			wantSub: `unknown pick "vibes"`,
		},
		{
			name:    "bad cooldown",
			mutate:  func(c *Config) { s := c.NextActions["quip"]; s.Cooldown = "soon"; c.NextActions["quip"] = s },
			wantSub: `invalid cooldown "soon"`,
		},
		{
			name: "unknown action reference",
			mutate: func(c *Config) {
				s := c.NextActions["quip"]
				s.Actions = []NextActionOffer{{Action: "ghost"}}
				c.NextActions["quip"] = s
			},
			wantSub: `unknown action "ghost"`,
		},
		{
			name: "empty offer",
			mutate: func(c *Config) {
				s := c.NextActions["quip"]
				s.Actions = []NextActionOffer{{Label: "just a label"}}
				c.NextActions["quip"] = s
			},
			wantSub: "empty (set one of",
		},
		{
			// The union must be exactly one member: an offer that both acts
			// and navigates has no defined behavior.
			name: "ambiguous offer",
			mutate: func(c *Config) {
				s := c.NextActions["quip"]
				s.Actions = []NextActionOffer{{Action: "mark-done", Navigate: "/x"}}
				c.NextActions["quip"] = s
			},
			wantSub: "an affordance is exactly one",
		},
		{
			name: "bad snooze duration",
			mutate: func(c *Config) {
				s := c.NextActions["quip"]
				s.Actions = []NextActionOffer{{Snooze: []string{"later"}}}
				c.NextActions["quip"] = s
			},
			wantSub: `invalid snooze duration "later"`,
		},
		{
			// Silently ignoring confirm on a non-mutating offer would teach
			// an operator that it works.
			name: "confirm on non-mutating offer",
			mutate: func(c *Config) {
				s := c.NextActions["quip"]
				s.Actions = []NextActionOffer{{Navigate: "/x", Confirm: true}}
				c.NextActions["quip"] = s
			},
			wantSub: "confirm only applies to",
		},
		{
			name: "malformed count",
			mutate: func(c *Config) {
				s := c.NextActions["quip"]
				s.Query = ""
				s.Count = "client > 0"
				c.NextActions["quip"] = s
			},
			wantSub: "count must be of the form",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cfg := validNextActionConfig()
			tc.mutate(cfg)
			errs := validateNextActions(cfg, nil)
			if len(errs) == 0 {
				t.Fatalf("expected an error containing %q, got none", tc.wantSub)
			}
			for _, e := range errs {
				if strings.Contains(e, tc.wantSub) {
					return
				}
			}
			t.Errorf("no error contained %q; got %v", tc.wantSub, errs)
		})
	}
}

// TestValidateNextActions_CountIsValid pins the first-run shape, which is the
// one source kind that has no entity to be about.
func TestValidateNextActions_CountIsValid(t *testing.T) {
	cfg := validNextActionConfig()
	cfg.NextActions["first-run"] = NextActionSource{
		Band:    "blocking",
		Count:   "client == 0",
		Suggest: "Nothing here yet. Start with a client?",
		Actions: []NextActionOffer{{Navigate: "/form/client"}},
	}
	if errs := validateNextActions(cfg, nil); len(errs) > 0 {
		t.Fatalf("count source reported errors: %v", errs)
	}
}

func TestNextActionBand_ResolvedProminence(t *testing.T) {
	tests := []struct {
		name string
		band NextActionBand
		want NextActionProminence
	}{
		// The default is the QUIETEST tier: an operator who has not thought
		// about prominence has not earned the top of the page.
		{"unset defaults to statusbar", NextActionBand{ID: "b"}, ProminenceStatusBar},
		{"banner", NextActionBand{ID: "b", Prominence: ProminenceBanner}, ProminenceBanner},
		{"notice", NextActionBand{ID: "b", Prominence: ProminenceNotice}, ProminenceNotice},
		{"statusbar", NextActionBand{ID: "b", Prominence: ProminenceStatusBar}, ProminenceStatusBar},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.band.Resolved(); got != tc.want {
				t.Errorf("Resolved() = %q, want %q", got, tc.want)
			}
		})
	}
}

// An unknown prominence must be rejected, not silently defaulted: a band the
// operator asked to be quiet would otherwise render at full volume with no
// explanation.
func TestValidateNextActions_RejectsUnknownProminence(t *testing.T) {
	cfg := validNextActionConfig()
	cfg.NextActionBands[0].Prominence = "shouty"

	errs := validateNextActions(cfg, nil)
	for _, e := range errs {
		if strings.Contains(e, `unknown prominence "shouty"`) {
			return
		}
	}
	t.Errorf("expected an unknown-prominence error, got %v", errs)
}

func TestValidateNextActions_AcceptsEveryProminence(t *testing.T) {
	for _, p := range []NextActionProminence{
		"", ProminenceBanner, ProminenceNotice, ProminenceStatusBar,
	} {
		t.Run(string(p), func(t *testing.T) {
			cfg := validNextActionConfig()
			cfg.NextActionBands[0].Prominence = p
			if errs := validateNextActions(cfg, nil); len(errs) > 0 {
				t.Errorf("prominence %q rejected: %v", p, errs)
			}
		})
	}
}
