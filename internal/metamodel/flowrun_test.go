package metamodel

import (
	"slices"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/entity"
)

const flowRunTestSchema = `
version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties:
      title:
        type: string
  note:
    label: Note
    id_prefix: "NOTE-"
    faces: {draft: {}, published: {}}
    properties:
      title: {type: string}
relations:
  blocks:
    label: blocks
    from: [task]
    to: [task]
`

// TestWithFlowRuns_NoRelations pins that a schema without `relations:`
// gets the run relations instead of a panic.
func TestWithFlowRuns_NoRelations(t *testing.T) {
	base, err := Parse([]byte(`
version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties: {title: {type: string}}
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, err := WithFlowRuns(base)
	if err != nil {
		t.Fatalf("WithFlowRuns: %v", err)
	}
	if _, ok := got.Relations[entity.CarriesRelation]; !ok {
		t.Fatalf("%s not declared", entity.CarriesRelation)
	}
}

func TestWithFlowRuns(t *testing.T) {
	base, err := Parse([]byte(flowRunTestSchema))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	got, err := WithFlowRuns(base)
	if err != nil {
		t.Fatalf("WithFlowRuns: %v", err)
	}

	t.Run("adds the type and both relations", func(t *testing.T) {
		if !got.HasFlowRuns() {
			t.Fatal("HasFlowRuns = false")
		}
		def, ok := got.Entities[entity.FlowRunType]
		if !ok {
			t.Fatalf("%s not declared", entity.FlowRunType)
		}
		if def.Properties[FlowRunPropStatus].Type != PropertyTypeEnum {
			t.Errorf("status type = %q", def.Properties[FlowRunPropStatus].Type)
		}
		for _, rel := range []string{entity.CarriesRelation, entity.ProducedRelation} {
			r, ok := got.Relations[rel]
			if !ok {
				t.Fatalf("%s not declared", rel)
			}
			if !slices.Equal(r.From, []string{entity.FlowRunType}) {
				t.Errorf("%s from = %v", rel, r.From)
			}
			// Faceless operator types only: never the faced note, never
			// the run type itself (RR-1QKRQPB).
			if !slices.Equal(r.To, []string{"task"}) {
				t.Errorf("%s to = %v, want [task]", rel, r.To)
			}
		}
	})

	t.Run("leaves the input unchanged", func(t *testing.T) {
		if base.HasFlowRuns() {
			t.Error("input HasFlowRuns = true")
		}
		if _, ok := base.Entities[entity.FlowRunType]; ok {
			t.Error("input gained the run type")
		}
		if _, ok := base.Relations[entity.CarriesRelation]; ok {
			t.Error("input gained _carries")
		}
	})

	t.Run("twice is an error", func(t *testing.T) {
		if _, err := WithFlowRuns(got); err == nil {
			t.Fatal("want error")
		}
	})

	t.Run("nil is an error", func(t *testing.T) {
		if _, err := WithFlowRuns(nil); err == nil {
			t.Fatal("want error")
		}
	})
}

// A conversion between faceless types needs no mapping for the managed
// relations: a schema cannot name them (critical review finding on
// TKT-BC5G8RP).
func TestWithFlowRuns_Conversions(t *testing.T) {
	base, err := Parse([]byte(`
version: "1.0"
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties: {title: {type: string}}
  issue:
    label: Issue
    id_prefix: "ISS-"
    properties: {title: {type: string}}
relations: {}
conversions:
  task-to-issue:
    from: task
    to: issue
    fields:
      title: "{{source.title}}"
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	if _, err := WithFlowRuns(base); err != nil {
		t.Fatalf("WithFlowRuns: %v", err)
	}
}

// WithFlowRuns validates a copy; a published snapshot may be read
// concurrently, so validate must not write through to it. Run with -race.
func TestWithFlowRuns_ConcurrentReaders(t *testing.T) {
	base, err := Parse([]byte(`
version: "1.0"
types:
  code:
    description: Code
    validations:
      - pattern: "^[A-Z]+$"
        error: upper case
entities:
  task:
    label: Task
    id_prefix: "TASK-"
    properties: {title: {type: string}, code: {type: code}}
relations: {}
comments:
  enabled: true
  on: [task]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for range 100 {
			for _, ct := range base.Types {
				for i := range ct.Validations {
					_ = ct.Validations[i].Compiled()
				}
			}
			_ = slices.Clone(base.Comments.On)
		}
	}()
	for range 10 {
		if _, err := WithFlowRuns(base); err != nil {
			t.Fatalf("WithFlowRuns: %v", err)
		}
	}
	<-done
}

func TestWithFlowRuns_Refusals(t *testing.T) {
	cases := []struct {
		name, schema, want string
	}{
		{
			name: "id prefix collision",
			schema: `
version: "1.0"
entities:
  runbook:
    label: Runbook
    id_prefix: "run-"
    properties:
      title: {type: string}
`,
			want: "id prefix",
		},
		{
			name: "only faced types",
			schema: `
version: "1.0"
entities:
  note:
    label: Note
    id_prefix: "NOTE-"
    faces: {draft: {}, published: {}}
    properties:
      title: {type: string}
`,
			want: "no faceless entity type",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			m, err := Parse([]byte(tc.schema))
			if err != nil {
				t.Fatalf("Parse: %v", err)
			}
			_, err = WithFlowRuns(m)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want it to mention %q", err, tc.want)
			}
		})
	}
}

// An operator schema that declares a managed name still fails: the
// exemption belongs to WithFlowRuns alone.
func TestParse_ManagedNamesStayReserved(t *testing.T) {
	for _, schema := range []string{
		`
version: "1.0"
entities:
  _flow-run:
    label: Run
`,
		`
version: "1.0"
entities:
  task:
    label: Task
relations:
  _carries:
    label: carries
    from: [task]
    to: [task]
`,
	} {
		if _, err := Parse([]byte(schema)); err == nil || !strings.Contains(err.Error(), "reserved") {
			t.Errorf("Parse = %v, want a reserved-name error", err)
		}
	}
}

// `comments: on: ["*"]` covers the operator's types, not a flow run.
func TestWithFlowRuns_CommentWildcardSkipsRuns(t *testing.T) {
	base, err := Parse([]byte(flowRunTestSchema + `comments:
  enabled: true
  on: ["*"]
`))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	m, err := WithFlowRuns(base)
	if err != nil {
		t.Fatalf("WithFlowRuns: %v", err)
	}
	p := NewCommentPolicy(m)
	if !p.Commentable("task") || p.Commentable(entity.FlowRunType) {
		t.Errorf("task %t, run %t; want only task commentable", p.Commentable("task"), p.Commentable(entity.FlowRunType))
	}
}
