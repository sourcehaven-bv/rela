package appbuild

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/affordances"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/mail"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/store/memstore"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// The core claim of scheduled-mail fan-out is that each message is rendered
// under ITS OWN recipient's ACL principal, with both row denial and field
// redaction applied. Nothing verified that against a real policy: the only test
// touching RunScheduledTemplate installs visibility.NopRedactor{} and no
// Declarative, so it exercises the rendering plumbing with the access control
// switched off (TKT-MESVDG / issue #1474, and PLAN-XMWT23 AC7, which promised
// exactly this test).
//
// The gap matters because the failure it would catch is the worst one this
// feature has: a regression in recipient scoping that mails one person another
// person's data. That is silent — the mail still sends, still looks right, and
// only the wrong recipient can tell.
//
// So this test denies one ROW and redacts one FIELD for a specific recipient
// and asserts both are absent from the rendered content, while a second
// recipient with a wider role still sees them. Asserting only the absence would
// pass against a renderer that emitted nothing at all.

// aclRelationLookup implements affordances.RelationLookup over the store.
type aclRelationLookup struct{ st store.Store }

func (l aclRelationLookup) OutgoingCounts(ctx context.Context, fromID string) map[string]int {
	counts := map[string]int{}
	for rel, err := range l.st.ListRelations(ctx, store.RelationQuery{
		From: fromID, Direction: store.DirectionOutgoing,
	}) {
		if err != nil || rel == nil {
			continue
		}
		counts[rel.Type]++
	}
	return counts
}

func (l aclRelationLookup) HasEdge(ctx context.Context, fromID, relType, toID string) bool {
	for rel, err := range l.st.ListRelations(ctx, store.RelationQuery{
		From: fromID, Direction: store.DirectionOutgoing,
	}) {
		if err != nil || rel == nil {
			continue
		}
		if rel.Type == relType && rel.To == toID {
			return true
		}
	}
	return false
}

// `secret` is visible only to the `lead` role; `public` tasks are readable by
// both roles, `internal` tasks only by lead. So alice (viewer) must lose one
// ROW entirely and one FIELD of the row she keeps; dana (lead) sees both.
const scheduledMailACL = `
roles:
  viewer:
    read:
      - person
    visible:
      task:
        - field: title
  lead:
    read:
      - person
      - task
    visible:
      task:
        - field: title
        - field: secret
assignments:
  alice: viewer
  dana: lead
`

func mustScheduledMailACL(t *testing.T, st store.Store, meta *metamodel.Metamodel) (*acl.Declarative, visibility.FieldRedactor) {
	t.Helper()
	var p acl.Policy
	require.NoError(t, yaml.Unmarshal([]byte(scheduledMailACL), &p))
	d, err := acl.NewDeclarative(&p, acl.NewStoreGraph(st), st)
	require.NoError(t, err)
	resolver, err := affordances.New(meta, aclRelationLookup{st}, d)
	require.NoError(t, err)
	redact, err := visibility.NewPolicyRedactor(resolver)
	require.NoError(t, err)
	return d, redact
}

func TestRunScheduledTemplate_RedactsPerRecipientACL(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	st := memstore.New()

	// `viewer` grants read on person but NOT on task, so alice loses every task
	// ROW. dana holds `lead`, which reads task and sees the `secret` field.
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
		ID: "alice", Type: "person", Properties: map[string]any{"email": "alice@example.test"},
	}))
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
		ID: "dana", Type: "person", Properties: map[string]any{"email": "dana@example.test"},
	}))
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
		ID: "T-1", Type: "task",
		Properties: map[string]any{"title": "Quarterly review", "secret": "salary-band-4"},
	}))

	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"person": {Properties: map[string]metamodel.PropertyDef{
			"email": {Type: metamodel.PropertyTypeString},
		}},
		"task": {Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: metamodel.PropertyTypeString},
			"secret": {Type: metamodel.PropertyTypeString},
		}},
	}}

	d, redact := mustScheduledMailACL(t, st, meta)
	sender := mail.NewMemorySender(4)
	svc := &Services{
		store: st, meta: meta,
		aclDeclarative: d,
		fieldRedactor:  redact,
		cfgLoader: staticConfig{"mail-templates.yaml": []byte(`mail_templates:
  digest:
    subject: Daily digest
    address_property: email
    sections:
      - entity_type: task
        columns: [title, secret]
`), "schedules.yaml": []byte(`tasks:
  - name: digest
    template: digest
    every: day
    for_each: {entity_type: person}
`)},
		mail: &mailRuntime{config: &mail.Config{BaseURL: "https://rela.example"}, sender: sender},
	}

	send := func(t *testing.T, user string) mail.Message {
		t.Helper()
		pctx := principal.With(ctx, principal.Principal{User: user, Tool: principal.ToolScheduler})
		require.NoError(t, svc.RunScheduledTemplate(pctx, "digest", user))
		msgs := sender.Messages()
		require.NotEmpty(t, msgs)
		return msgs[len(msgs)-1]
	}

	// The POSITIVE half first. Without it, the absence assertions below would
	// pass against a renderer that emitted nothing at all — which is the way a
	// redaction test most often silently stops testing anything.
	lead := send(t, "dana")
	require.Contains(t, string(lead.Text), "Quarterly review",
		"lead reads task, so the row must be present")
	require.Contains(t, string(lead.Text), "salary-band-4",
		"lead is granted the secret field, so it must be present")

	// Now the recipient whose policy denies the row outright.
	viewer := send(t, "alice")
	require.Equal(t, "alice@example.test", viewer.To[0].Email,
		"precondition: this is alice's message, not dana's")
	// RenderedFor is the attribution the fan-out uses to say WHOSE ACL the
	// content was rendered under. If it drifts from the addressee, an audit of
	// "who was this rendered for" answers a different question than "who
	// received it" — the two must agree or neither can be trusted.
	require.Equal(t, "alice", viewer.RenderedFor,
		"the message must be attributed to the recipient it was addressed to")
	require.NotContains(t, string(viewer.Text), "Quarterly review",
		"viewer has no read on task, so the ROW must not appear")
	require.NotContains(t, string(viewer.Text), "salary-band-4",
		"the secret field must not leak through the denied row")
}

// The field-level half, isolated: a recipient who CAN read the row must still
// lose a field their role does not grant. Separated from the row-denial case
// because a row denial hides a field for free — a single test would not
// distinguish "the field was redacted" from "the whole row was gone".
func TestRunScheduledTemplate_RedactsFieldOnAVisibleRow(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	st := memstore.New()

	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
		ID: "carol", Type: "person", Properties: map[string]any{"email": "carol@example.test"},
	}))
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
		ID: "T-1", Type: "task",
		Properties: map[string]any{"title": "Quarterly review", "secret": "salary-band-4"},
	}))

	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"person": {Properties: map[string]metamodel.PropertyDef{
			"email": {Type: metamodel.PropertyTypeString},
		}},
		"task": {Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: metamodel.PropertyTypeString},
			"secret": {Type: metamodel.PropertyTypeString},
		}},
	}}

	// carol reads task (so the row survives) but is granted only `title`.
	const policyYAML = `
roles:
  reporter:
    read:
      - person
      - task
    visible:
      task:
        - field: title
assignments:
  carol: reporter
`
	var p acl.Policy
	require.NoError(t, yaml.Unmarshal([]byte(policyYAML), &p))
	d, err := acl.NewDeclarative(&p, acl.NewStoreGraph(st), st)
	require.NoError(t, err)
	resolver, err := affordances.New(meta, aclRelationLookup{st}, d)
	require.NoError(t, err)
	redact, err := visibility.NewPolicyRedactor(resolver)
	require.NoError(t, err)

	sender := mail.NewMemorySender(4)
	svc := &Services{
		store: st, meta: meta,
		aclDeclarative: d,
		fieldRedactor:  redact,
		cfgLoader: staticConfig{"mail-templates.yaml": []byte(`mail_templates:
  digest:
    subject: Daily digest
    address_property: email
    sections:
      - entity_type: task
        columns: [title, secret]
`), "schedules.yaml": []byte(`tasks:
  - name: digest
    template: digest
    every: day
    for_each: {entity_type: person}
`)},
		mail: &mailRuntime{config: &mail.Config{BaseURL: "https://rela.example"}, sender: sender},
	}

	pctx := principal.With(ctx, principal.Principal{User: "carol", Tool: principal.ToolScheduler})
	require.NoError(t, svc.RunScheduledTemplate(pctx, "digest", "carol"))
	msgs := sender.Messages()
	require.Len(t, msgs, 1)

	// The row IS present — that is what makes this the field case rather than
	// a second copy of the row-denial test.
	require.Contains(t, string(msgs[0].Text), "Quarterly review",
		"reporter reads task, so the row must survive")
	require.NotContains(t, string(msgs[0].Text), "salary-band-4",
		"the secret field is not granted to reporter and must be redacted")
}

// require_visible_content (TKT-WRKFQL) suppresses a send whose every section is
// empty AFTER recipient-scoped ACL filtering — the case that otherwise mails
// somebody a message reading only "Nothing to show.".
//
// The whole matrix runs against ONE fixture so the cases differ only in the
// recipient's policy and the opt-in flag. alice (viewer) cannot read task at
// all, dana (lead) reads every row: same graph, same template, opposite
// outcomes. Asserting suppression alone would pass against a build that never
// sends anything, so the visible cases are asserted alongside it.
func TestRunScheduledTemplate_RequireVisibleContent(t *testing.T) {
	t.Parallel()

	newSvc := func(t *testing.T, requireVisible bool) (*Services, *mail.MemorySender) {
		t.Helper()
		ctx := t.Context()
		st := memstore.New()
		require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
			ID: "alice", Type: "person", Properties: map[string]any{"email": "alice@example.test"},
		}))
		require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
			ID: "dana", Type: "person", Properties: map[string]any{"email": "dana@example.test"},
		}))
		require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
			ID: "T-1", Type: "task",
			Properties: map[string]any{"title": "Quarterly review", "secret": "salary-band-4"},
		}))

		meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
			"person": {Properties: map[string]metamodel.PropertyDef{
				"email": {Type: metamodel.PropertyTypeString},
			}},
			"task": {Properties: map[string]metamodel.PropertyDef{
				"title":  {Type: metamodel.PropertyTypeString},
				"secret": {Type: metamodel.PropertyTypeString},
			}},
		}}

		templates := `mail_templates:
  digest:
    subject: Daily digest
    address_property: email
`
		if requireVisible {
			templates += "    require_visible_content: true\n"
		}
		templates += `    sections:
      - entity_type: task
        columns: [title]
`
		d, redact := mustScheduledMailACL(t, st, meta)
		sender := mail.NewMemorySender(4)
		return &Services{
			store: st, meta: meta,
			aclDeclarative: d,
			fieldRedactor:  redact,
			cfgLoader: staticConfig{"mail-templates.yaml": []byte(templates), "schedules.yaml": []byte(`tasks:
  - name: digest
    template: digest
    every: day
    for_each: {entity_type: person}
`)},
			mail: &mailRuntime{config: &mail.Config{BaseURL: "https://rela.example"}, sender: sender},
		}, sender
	}

	run := func(t *testing.T, svc *Services, user string) {
		t.Helper()
		pctx := principal.With(t.Context(), principal.Principal{User: user, Tool: principal.ToolScheduler})
		require.NoError(t, svc.RunScheduledTemplate(pctx, "digest", user))
	}

	t.Run("fully hidden is suppressed when opted in", func(t *testing.T) {
		t.Parallel()
		svc, sender := newSvc(t, true)
		run(t, svc, "alice")
		require.Empty(t, sender.Messages(),
			"alice cannot read any task row, so there is nothing to mail her")
	})

	t.Run("fully visible still sends when opted in", func(t *testing.T) {
		t.Parallel()
		svc, sender := newSvc(t, true)
		run(t, svc, "dana")
		msgs := sender.Messages()
		require.Len(t, msgs, 1, "dana reads the row, so opting in must not suppress her mail")
		require.Contains(t, string(msgs[0].Text), "Quarterly review")
	})

	// AC 3: the default must be byte-for-byte today's behavior, including the
	// placeholder this feature exists to avoid. Without this case a build that
	// suppressed unconditionally would still look correct above.
	t.Run("fully hidden still sends when opted out", func(t *testing.T) {
		t.Parallel()
		svc, sender := newSvc(t, false)
		run(t, svc, "alice")
		msgs := sender.Messages()
		require.Len(t, msgs, 1, "without the opt-in, an empty digest sends exactly as before")
		require.Contains(t, string(msgs[0].Text), "Nothing to show.")
	})
}

// Partial visibility must SEND: the recipient has content, just less of it.
// This is the case a naive "any section empty" reading would get wrong, and it
// is separated from the matrix above because it needs a second section that is
// empty for everyone.
func TestRunScheduledTemplate_RequireVisibleContentSendsOnPartialVisibility(t *testing.T) {
	t.Parallel()
	ctx := t.Context()
	st := memstore.New()
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
		ID: "dana", Type: "person", Properties: map[string]any{"email": "dana@example.test"},
	}))
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
		ID: "T-1", Type: "task",
		Properties: map[string]any{"title": "Quarterly review", "secret": "salary-band-4"},
	}))

	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"person": {Properties: map[string]metamodel.PropertyDef{
			"email": {Type: metamodel.PropertyTypeString},
		}},
		"task": {Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: metamodel.PropertyTypeString},
			"secret": {Type: metamodel.PropertyTypeString},
		}},
		// A type with no instances: the first section is empty for everyone,
		// which is what makes this a PARTIAL rather than a total-emptiness case.
		"note": {Properties: map[string]metamodel.PropertyDef{
			"title": {Type: metamodel.PropertyTypeString},
		}},
	}}

	// A local policy rather than the shared scheduledMailACL: this case needs a
	// `note` grant, and widening the shared role would change what the row- and
	// field-redaction tests above are asserting.
	const partialPolicy = `
roles:
  lead:
    read:
      - person
      - task
      - note
    visible:
      task:
        - field: title
      note:
        - field: title
assignments:
  dana: lead
`
	var p acl.Policy
	require.NoError(t, yaml.Unmarshal([]byte(partialPolicy), &p))
	d, err := acl.NewDeclarative(&p, acl.NewStoreGraph(st), st)
	require.NoError(t, err)
	resolver, err := affordances.New(meta, aclRelationLookup{st}, d)
	require.NoError(t, err)
	redact, err := visibility.NewPolicyRedactor(resolver)
	require.NoError(t, err)

	sender := mail.NewMemorySender(4)
	svc := &Services{
		store: st, meta: meta,
		aclDeclarative: d,
		fieldRedactor:  redact,
		cfgLoader: staticConfig{"mail-templates.yaml": []byte(`mail_templates:
  digest:
    subject: Daily digest
    address_property: email
    require_visible_content: true
    sections:
      - entity_type: note
        columns: [title]
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

	pctx := principal.With(ctx, principal.Principal{User: "dana", Tool: principal.ToolScheduler})
	require.NoError(t, svc.RunScheduledTemplate(pctx, "digest", "dana"))
	msgs := sender.Messages()
	require.Len(t, msgs, 1, "one empty section must not suppress a message the other section fills")
	require.Contains(t, string(msgs[0].Text), "Quarterly review")
}

// AC 4: a suppressed send must be diagnosable by the operator WITHOUT
// disclosing what was filtered. The recipient could not see those entities, so
// the log line that explains their absence must not name them either — the log
// is a plausible sink for exactly the leak the ACL just prevented.
func TestRunScheduledTemplate_SuppressionLogDoesNotDiscloseHiddenContent(t *testing.T) {
	// No t.Parallel: this swaps the process-wide default logger, which would
	// capture (or lose) output from any test running beside it.
	ctx := t.Context()
	st := memstore.New()
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
		ID: "alice", Type: "person", Properties: map[string]any{"email": "alice@example.test"},
	}))
	require.NoError(t, st.CreateEntity(ctx, &entity.Entity{
		ID: "T-1", Type: "task",
		Properties: map[string]any{"title": "Quarterly review", "secret": "salary-band-4"},
	}))

	meta := &metamodel.Metamodel{Entities: map[string]metamodel.EntityDef{
		"person": {Properties: map[string]metamodel.PropertyDef{
			"email": {Type: metamodel.PropertyTypeString},
		}},
		"task": {Properties: map[string]metamodel.PropertyDef{
			"title":  {Type: metamodel.PropertyTypeString},
			"secret": {Type: metamodel.PropertyTypeString},
		}},
	}}

	d, redact := mustScheduledMailACL(t, st, meta)
	sender := mail.NewMemorySender(4)
	svc := &Services{
		store: st, meta: meta,
		aclDeclarative: d,
		fieldRedactor:  redact,
		cfgLoader: staticConfig{"mail-templates.yaml": []byte(`mail_templates:
  digest:
    subject: Daily digest
    address_property: email
    require_visible_content: true
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

	var logged bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(restore) })

	pctx := principal.With(ctx, principal.Principal{User: "alice", Tool: principal.ToolScheduler})
	require.NoError(t, svc.RunScheduledTemplate(pctx, "digest", "alice"))
	require.Empty(t, sender.Messages(), "precondition: this run must have been suppressed")

	out := logged.String()
	// The positive half: without it, the absence assertions below would pass
	// against a build that logged nothing at all.
	require.Contains(t, out, "no visible content", "the operator needs to know a send was skipped")
	require.Contains(t, out, "digest", "the log must name the template")
	require.Contains(t, out, "alice", "the log must name the recipient")

	require.NotContains(t, out, "Quarterly review",
		"the hidden entity's title must not reach the log")
	require.NotContains(t, out, "salary-band-4",
		"the redacted field value must not reach the log")
	require.NotContains(t, out, "T-1",
		"even the hidden entity's ID must not reach the log")
}
