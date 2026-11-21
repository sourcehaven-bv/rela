package flowcfg

import (
	"strings"
	"testing"
	"time"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

// TestRunTitle pins BUG-53GP5F5: a run stores the flow's title:
// template, or the flow's label, which copies nothing of the subject.
func TestRunTitle(t *testing.T) {
	now := time.Date(2026, 11, 21, 9, 0, 0, 0, time.UTC)
	ada := &entity.Entity{ID: "PER-1", Type: "person", Properties: map[string]any{"name": "Ada"}}
	tests := []struct {
		name  string
		title string // the flow's title: line; "" for none
		subj  *entity.Entity
		want  string
	}{
		{name: "label", subj: ada, want: "Onboarding"},
		{name: "template", title: `title: "Welcome {{subject.name}} on {{today}}"`, subj: ada,
			want: "Welcome Ada on 2026-11-21"},
		{name: "literal template", title: "title: Signup", subj: ada, want: "Signup"},
		{name: "no subject", title: `title: "{{subject.name}}"`, want: "Onboarding"},
		{name: "template without the value", title: `title: "{{subject.stage}}"`, subj: ada, want: "Onboarding"},
		{name: "long", title: `title: "{{subject.name}}"`,
			subj: &entity.Entity{ID: "PER-3", Type: "person", Properties: map[string]any{"name": strings.Repeat("x", 300)}},
			want: strings.Repeat("x", MaxTitleLen-1) + "…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var edits [][2]string
			if tc.title != "" {
				edits = append(edits, [2]string{"    label: Onboarding\n", "    label: Onboarding\n    " + tc.title + "\n"})
			}
			c, err := parseSample(t, edits...)
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			f, _ := c.Flow("onboarding")
			if got := f.RunTitle(tc.subj, now); got != tc.want {
				t.Errorf("RunTitle = %q, want %q", got, tc.want)
			}
			if f.HasTitle() != (tc.title != "") {
				t.Errorf("HasTitle = %v", f.HasTitle())
			}
		})
	}
}

// TestParse_TitleAndFailedMessage pins the load checks of title: and
// failed_message:.
func TestParse_TitleAndFailedMessage(t *testing.T) {
	const at = "    label: Onboarding\n"
	line := func(l string) [2]string { return [2]string{at, at + "    " + l + "\n"} }
	tests := []struct {
		name string
		edit [2]string
		want string // "" for no problem
	}{
		{name: "title", edit: line(`title: "Onboarding {{subject.name}}"`)},
		{name: "restricted property", edit: line(`title: "{{subject.salary}}"`), want: "visible:"},
		{name: "unknown property", edit: line(`title: "{{subject.nope}}"`), want: "no property"},
		{name: "run id", edit: line(`title: "{{run.id}}"`), want: "run.id"},
		{name: "too long", edit: line("title: " + strings.Repeat("x", MaxTitleLen+1)), want: "at most"},
		{name: "failed message", edit: line("failed_message: Mail support@example.com.")},
		{name: "long failed message", edit: line("failed_message: " + strings.Repeat("x", MaxFailedMessageLen+1)),
			want: "at most"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := parseSample(t, tc.edit)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("Parse: %v", err)
				}
				f, _ := c.Flow("onboarding")
				if tc.name == "failed message" && f.FailedMessage() != "Mail support@example.com." {
					t.Errorf("FailedMessage = %q", f.FailedMessage())
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %v, want %q", err, tc.want)
			}
		})
	}
}
