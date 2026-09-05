package appbuild

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

type staticConfig map[string][]byte

func (c staticConfig) Load(_ context.Context, name string) ([]byte, error) { return c[name], nil }

// List reports no directory-shaped config: these tests exercise the
// mail-template path, which reads named files only.
func (c staticConfig) List(_ context.Context, _ string) ([]string, error) { return nil, nil }

func TestRunScheduledTemplateSendsRenderedRecipientMessage(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	recipient := &entity.Entity{
		ID: "P-1", Type: "person", Properties: map[string]any{"email": "Alice <alice@example.test>"},
	}
	task := &entity.Entity{
		ID: "T-1", Type: "task", Properties: map[string]any{"title": "Visible task"},
	}
	require.NoError(t, st.CreateEntity(t.Context(), recipient))
	require.NoError(t, st.CreateEntity(t.Context(), &entity.Entity{
		ID: "P-2", Type: "person", Properties: map[string]any{"email": ""},
	}))
	require.NoError(t, st.CreateEntity(t.Context(), task))

	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"person": {Properties: map[string]metamodel.PropertyDef{"email": {Type: metamodel.PropertyTypeString}}},
		"task":   {Properties: map[string]metamodel.PropertyDef{"title": {Type: metamodel.PropertyTypeString}}},
	}}
	sender := mail.NewMemorySender(4)
	svc := &Services{
		store: st, meta: meta, fieldRedactor: visibility.NopRedactor{},
		cfgLoader: staticConfig{"mail-templates.yaml": []byte(`mail_templates:
  digest:
    subject: Daily digest
    address_property: email
    sections:
      - entity_type: task
        columns: [title]
`), "schedules.yaml": []byte(`tasks:
  - name: digest
    template: digest
    every: day
    for_each: {entity_type: person}
`)},
		mail: &mailRuntime{config: &mail.Config{BaseURL: "https://rela.example"}, sender: sender},
	}

	require.NoError(t, svc.RunScheduledTemplate(t.Context(), "digest", recipient.ID))
	messages := sender.Messages()
	require.Len(t, messages, 1)
	require.Equal(t, recipient.ID, messages[0].RenderedFor)
	require.Equal(t, "alice@example.test", messages[0].To[0].Email)
	require.Contains(t, string(messages[0].Text), task.Properties["title"])
	require.ErrorContains(t, svc.ValidateScheduledMailRecipients(t.Context()), "P-2")
}
