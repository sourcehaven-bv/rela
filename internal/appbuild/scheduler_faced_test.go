package appbuild

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// TestScheduledForEachPrincipal_FacedUser pins that a user entity of a faced
// type maps to its principal. A principal is an entity, not a face, and a
// faced type has no zero-face row (DEC-NPZICR), which a zero-face read took
// for "not a user".
func TestScheduledForEachPrincipal_FacedUser(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	require.NoError(t, st.CreateEntity(t.Context(), &entity.Entity{ID: "U-1", Type: "person"}))
	require.NoError(t, st.CreateEntity(t.Context(), &entity.Entity{ID: "U-F", Type: "person", Face: "draft"}))
	require.NoError(t, st.CreateEntity(t.Context(), &entity.Entity{ID: "T-1", Type: "task"}))
	svc := &Services{store: st, aclPolicy: &acl.Policy{UserEntityType: "person"}}

	for id, want := range map[string]string{"U-1": "U-1", "U-F": "U-F", "T-1": "", "NOPE": ""} {
		got, err := svc.ScheduledForEachPrincipal(t.Context(), id)
		require.NoError(t, err)
		require.Equal(t, want, got, "principal for %s", id)
	}
}

// TestRunScheduledTemplate_FacedRecipientInDefaultWorld pins that scheduled
// mail addresses the recipient row the default world selects (TKT-KQXVF7
// D7). The default world resolves no face of a faced type until TKT-7IZHP0,
// so the recipient is skipped, never addressed from an arbitrary face.
func TestRunScheduledTemplate_FacedRecipientInDefaultWorld(t *testing.T) {
	t.Parallel()
	st := memstore.New()
	require.NoError(t, st.CreateEntity(t.Context(), &entity.Entity{
		ID: "P-F", Type: "person", Face: "draft",
		Properties: map[string]any{"email": "draft@example.test"},
	}))
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
`)},
		mail: &mailRuntime{config: &mail.Config{BaseURL: "https://rela.example"}, sender: sender},
	}

	require.NoError(t, svc.RunScheduledTemplate(t.Context(), "digest", "P-F"))
	require.Empty(t, sender.Messages())
}
