package mailtemplate_test

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mailtemplate"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

type reader struct{ entities []*entity.Entity }

func (r reader) ListEntities(_ context.Context, q store.EntityQuery) iter.Seq2[*entity.Entity, error] {
	return func(yield func(*entity.Entity, error) bool) {
		for _, ent := range r.entities {
			if (q.Type == "" || q.Type == ent.Type) && !yield(ent, nil) {
				return
			}
		}
	}
}

func model() *metamodel.Metamodel {
	return &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"task": {Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: metamodel.PropertyTypeString},
			"status": {Type: metamodel.PropertyTypeString},
		}},
	}}
}

func TestParseStrictlyValidatesSections(t *testing.T) {
	t.Parallel()
	_, err := mailtemplate.Parse([]byte(`mail_templates:
  digest:
    subject: Hello
    address_property: email
    sections:
      - entity_type: missing
        columns: [titel]
`), model())
	require.ErrorContains(t, err, `unknown entity type "missing"`)

	_, err = mailtemplate.Parse([]byte(`mail_templates:
  digest:
    subject: Hello
    address_property: email
    typo: true
`), model())
	require.ErrorContains(t, err, "field typo not found")
}

func TestBuildTableListAndDetail(t *testing.T) {
	t.Parallel()
	cfg, err := mailtemplate.Parse([]byte(`mail_templates:
  digest:
    subject: "Tasks {{today}}"
    address_property: email
    sections:
      - title: Table
        entity_type: task
        where: ["status = open"]
        columns: [title]
        link: true
      - title: List
        entity_type: task
        style: list
      - title: Detail
        entity_type: task
        style: detail
`), model())
	require.NoError(t, err)

	msg, _, err := mailtemplate.Build(t.Context(), model(), reader{entities: []*entity.Entity{
		{ID: "T-1", Type: "task", Properties: map[string]any{"title": "Visible", "status": "open"}, Content: "Agenda"},
	}}, cfg.Templates["digest"], time.Date(2026, 8, 26, 0, 0, 0, 0, time.UTC))
	require.NoError(t, err)
	require.Equal(t, "Tasks 2026-08-26", msg.Subject)
	require.Equal(t, [][]string{{"Visible"}}, msg.Sections[0].Rows)
	require.Equal(t, []string{"/entity/task/T-1"}, msg.Sections[0].Links)
	require.Contains(t, msg.Sections[1].Body, "[Visible](/entity/task/T-1)")
	require.Equal(t, "Agenda", msg.Sections[2].Body)
}

func TestBuildCannotExposeRowsMissingFromReader(t *testing.T) {
	t.Parallel()
	tmpl := mailtemplate.Template{Subject: "Digest", AddressProperty: "email", Sections: []mailtemplate.Section{{EntityType: "task", Columns: []string{"title"}}}}
	msg, _, err := mailtemplate.Build(t.Context(), model(), reader{}, tmpl, time.Now())
	require.NoError(t, err)
	require.Empty(t, msg.Sections[0].Rows)
}

// TestBuildCountsContributionsNotMatches pins the distinction RR-K7RMIC turned
// on: a `detail` entity with blank Content matches the section but renders as
// nothing, so it must not keep RequireVisibleContent from suppressing the send.
func TestBuildCountsContributionsNotMatches(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name        string
		style       string
		content     string
		contributed int
	}{
		{name: "detail with body contributes", style: "detail", content: "Agenda", contributed: 1},
		{name: "detail with blank body does not", style: "detail", content: "", contributed: 0},
		{name: "detail with whitespace body does not", style: "detail", content: "  \n\t ", contributed: 0},
		{name: "list contributes regardless of body", style: "list", content: "", contributed: 1},
		{name: "table contributes regardless of body", style: "table", content: "", contributed: 1},
		{name: "default style contributes", style: "", content: "", contributed: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			tmpl := mailtemplate.Template{
				Subject: "Digest {{count}}", AddressProperty: "email",
				Sections: []mailtemplate.Section{
					{EntityType: "task", Style: tc.style, Columns: []string{"title"}},
				},
			}
			msg, contributed, err := mailtemplate.Build(t.Context(), model(), reader{entities: []*entity.Entity{
				{ID: "T-1", Type: "task", Properties: map[string]any{"title": "Visible"}, Content: tc.content},
			}}, tmpl, time.Now())
			require.NoError(t, err)
			require.Equal(t, tc.contributed, contributed)

			// The match count is a SEPARATE number and must stay at 1 in every
			// case above: {{count}} means "entities matched", and redefining it
			// would silently change every template already using it.
			require.Equal(t, "Digest 1", msg.Subject)
		})
	}
}

func TestBuildReportsZeroContributionsWhenNothingMatches(t *testing.T) {
	t.Parallel()
	tmpl := mailtemplate.Template{
		Subject: "Digest", AddressProperty: "email",
		Sections: []mailtemplate.Section{{EntityType: "task", Columns: []string{"title"}}},
	}
	_, contributed, err := mailtemplate.Build(t.Context(), model(), reader{}, tmpl, time.Now())
	require.NoError(t, err)
	require.Zero(t, contributed)
}

// TestBuildCountsContributionsAcrossSections guards the "at least one section
// has content" reading: one non-empty section is enough to send.
func TestBuildCountsContributionsAcrossSections(t *testing.T) {
	t.Parallel()
	tmpl := mailtemplate.Template{
		Subject: "Digest", AddressProperty: "email",
		Sections: []mailtemplate.Section{
			{EntityType: "task", Where: []string{"status = done"}, Columns: []string{"title"}},
			{EntityType: "task", Where: []string{"status = open"}, Columns: []string{"title"}},
		},
	}
	_, contributed, err := mailtemplate.Build(t.Context(), model(), reader{entities: []*entity.Entity{
		{ID: "T-1", Type: "task", Properties: map[string]any{"title": "Open one", "status": "open"}},
	}}, tmpl, time.Now())
	require.NoError(t, err)
	require.Equal(t, 1, contributed, "one empty section must not mask the other's content")
}

// TestParseRequireVisibleContentYAMLForms pins yaml.v3's actual boolean
// handling (RR-NV7O2V). The asymmetry is genuinely surprising: quoted "true"
// is an error while quoted "yes" is accepted, so both directions are pinned
// against a future yaml bump.
func TestParseRequireVisibleContentYAMLForms(t *testing.T) {
	t.Parallel()
	// A section is required only because require_visible_content now demands
	// one (RR-RV093C); it is otherwise irrelevant to what this test pins.
	const tail = `    sections:
      - entity_type: task
        columns: [title]
`
	const head = `mail_templates:
  digest:
    subject: Digest
    address_property: email
`
	for _, tc := range []struct {
		name    string
		line    string
		wantErr bool
		want    bool
	}{
		{name: "bare true", line: "    require_visible_content: true\n", want: true},
		{name: "bare false", line: "    require_visible_content: false\n", want: false},
		{name: "absent defaults off", line: "", want: false},
		{name: "bare yes is YAML 1.1 true", line: "    require_visible_content: yes\n", want: true},
		{name: "quoted yes is also true", line: "    require_visible_content: \"yes\"\n", want: true},
		{name: "quoted true is rejected", line: "    require_visible_content: \"true\"\n", wantErr: true},
		{name: "non-boolean is rejected", line: "    require_visible_content: sometimes\n", wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			cfg, err := mailtemplate.Parse([]byte(head+tc.line+tail), model())
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.Templates["digest"].RequireVisibleContent)
		})
	}
}

// TestParseRejectsUnknownKeyBesideRequireVisibleContent guards that adding the
// field did not loosen the strict decoder.
func TestParseRejectsUnknownKeyBesideRequireVisibleContent(t *testing.T) {
	t.Parallel()
	_, err := mailtemplate.Parse([]byte(`mail_templates:
  digest:
    subject: Digest
    address_property: email
    require_visible_content: true
    bogus_key: 1
`), model())
	require.Error(t, err)
}

// A sections-less template is valid on its own (it may be pure intro), but
// combined with require_visible_content it can never send — the flag asks for
// content that has nowhere to come from. Caught at load, not at send time
// (RR-RV093C).
func TestParseRejectsRequireVisibleContentWithoutSections(t *testing.T) {
	t.Parallel()
	const body = `mail_templates:
  announce:
    subject: Weekly notice
    intro: The office is closed Monday.
    address_property: email
`
	// The same template WITHOUT the flag stays valid: this is the control that
	// proves the error is about the combination, not about sections alone.
	cfg, err := mailtemplate.Parse([]byte(body), model())
	require.NoError(t, err)
	require.Empty(t, cfg.Templates["announce"].Sections)

	_, err = mailtemplate.Parse([]byte(body+"    require_visible_content: true\n"), model())
	require.ErrorContains(t, err, "at least one section")
}
