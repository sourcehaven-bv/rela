package mailtemplate_test

import (
	"context"
	"iter"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mailrender"
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

	msg, err := mailtemplate.Build(t.Context(), model(), reader{entities: []*entity.Entity{
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
	msg, err := mailtemplate.Build(t.Context(), model(), reader{}, tmpl, time.Now())
	require.NoError(t, err)
	require.Empty(t, msg.Sections[0].Rows)
}

// TestTemplateLangReachesMessage covers the third of the three call sites that
// can set a language (config, Lua, Go), and pins that it is PER TEMPLATE.
//
// One deployment sends a Dutch digest and an English one from the same
// renderer, so a language that lived on the renderer would mislabel one of
// them. Building two templates and comparing is what makes that regression
// visible here.
func TestTemplateLangReachesMessage(t *testing.T) {
	t.Parallel()

	cfg, err := mailtemplate.Parse([]byte(`mail_templates:
  dutch:
    subject: Agenda
    address_property: email
    lang: nl
  english:
    subject: Digest
    address_property: email
  unset:
    subject: Plain
    address_property: email
`), model())
	require.NoError(t, err)

	build := func(name string) *mailrender.Message {
		msg, buildErr := mailtemplate.Build(
			context.Background(), model(), reader{}, cfg.Templates[name], time.Now())
		require.NoError(t, buildErr)
		return msg
	}

	require.Equal(t, "nl", build("dutch").Lang)
	require.Empty(t, build("unset").Lang, "an unset lang stays empty so the renderer default applies")

	// And it survives all the way into the rendered document.
	r, err := mailrender.New(&mailrender.Options{DefaultLang: "en"})
	require.NoError(t, err)
	html, _, err := r.Render(build("dutch"))
	require.NoError(t, err)
	require.Contains(t, string(html), `lang="nl"`)

	html, _, err = r.Render(build("unset"))
	require.NoError(t, err)
	require.Contains(t, string(html), `lang="en"`, "unset must fall back to the operator default")
}

// TestParseRejectsMalformedLang pins that an operator typo fails at LOAD.
//
// A language tag is interpolated into an HTML attribute, so it is validated and
// refused rather than escaped — and refusing it during `rela validate` beats
// discovering it when a scheduled digest fails to send.
func TestParseRejectsMalformedLang(t *testing.T) {
	t.Parallel()

	for _, bad := range []string{`en" onload="x`, `<script>`, `en_US`} {
		_, err := mailtemplate.Parse([]byte(`mail_templates:
  digest:
    subject: Hello
    address_property: email
    lang: '`+bad+`'
`), model())
		require.Error(t, err, "malformed lang %q must be refused at load", bad)
		require.ErrorContains(t, err, "language tag")
	}
}
