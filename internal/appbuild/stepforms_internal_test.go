package appbuild

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// stepFormSample compiles the flowcfg sample, whose collect state names the
// collect_documents form.
func stepFormSample(t *testing.T) (*flowcfg.Compiled, *metamodel.Metamodel) {
	t.Helper()
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("..", "flowcfg", "testdata", name))
		require.NoError(t, err)
		return data
	}
	meta, err := metamodel.Parse(read("schema.yaml"))
	require.NoError(t, err)
	policy, err := acl.ParsePolicy(read("acl.yaml"), "acl.yaml")
	require.NoError(t, err)
	flows, err := flowcfg.Parse(read("flows.yaml"), meta, policy)
	require.NoError(t, err)
	return flows, meta
}

func TestResolveStepForms(t *testing.T) {
	flows, meta := stepFormSample(t)
	field := func(prop string) dataentryconfig.FormField { return dataentryconfig.FormField{Property: prop} }
	yes := true
	tests := []struct {
		name string
		form *dataentryconfig.Form // nil: data-entry.yaml declares no collect_documents
		want []string
		err  string
	}{
		{name: "the form's fields", want: []string{"documents", "started"},
			form: &dataentryconfig.Form{EntityType: "person",
				Fields: []dataentryconfig.FormField{field("documents"), field("started")}}},
		{name: "hidden, blank and repeated fields are not sent", want: []string{"documents"},
			form: &dataentryconfig.Form{EntityType: "person", Fields: []dataentryconfig.FormField{
				field("documents"), {Property: "started", Hidden: true}, {}, field("documents")}}},
		{name: "no fields", want: []string{}, form: &dataentryconfig.Form{EntityType: "person"}},
		{name: "undeclared form", err: `declares no form "collect_documents"`},
		{name: "another type", err: `edits "task", not the flow's subject "person"`,
			form: &dataentryconfig.Form{EntityType: "task"}},
		{name: "wizard", err: "is a wizard",
			form: &dataentryconfig.Form{EntityType: "person", Steps: []dataentryconfig.FormStep{{Title: "one"}}}},
		{name: "relations", err: "edits relations",
			form: &dataentryconfig.Form{EntityType: "person",
				Relations: []dataentryconfig.FormRelation{{Relation: "about"}}}},
		{name: "body", err: "edits the body", form: &dataentryconfig.Form{EntityType: "person", Body: &yes}},
		{name: "undeclared property", err: `person has no property "documentz"`,
			form: &dataentryconfig.Form{EntityType: "person", Fields: []dataentryconfig.FormField{field("documentz")}}},
		{name: "derived property", err: "person.task_count is derived",
			form: &dataentryconfig.Form{EntityType: "person", Fields: []dataentryconfig.FormField{field("task_count")}}},
		{name: "reserved property", err: `"_x" is reserved`,
			form: &dataentryconfig.Form{EntityType: "person", Fields: []dataentryconfig.FormField{field("_x")}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			forms := map[string]dataentryconfig.Form{}
			if tc.form != nil {
				forms["collect_documents"] = *tc.form
			}
			got, err := resolveStepForms(flows, forms, meta)
			if tc.err != "" {
				require.ErrorContains(t, err, tc.err)
				require.ErrorContains(t, err, `flow "onboarding" state "collect"`)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got.fields("collect_documents"))
		})
	}
}

func TestStepForms_Fields(t *testing.T) {
	forms := stepForms{"f": {"a"}}
	require.Nil(t, forms.fields(""), "a state without a form accepts nothing")
	require.Nil(t, forms.fields("gone"), "a form data-entry.yaml no longer declares accepts nothing")
	got := forms.fields("f")
	got[0] = "b"
	require.Equal(t, []string{"a"}, forms["f"], "fields returns a copy")
}

func TestCheckFlowDataEntry(t *testing.T) {
	flows, meta := stepFormSample(t)
	ctx := context.Background()
	form := "forms:\n  collect_documents:\n    entity_type: person\n    fields:\n      - property: documents\n"

	require.NoError(t, CheckFlowDataEntry(ctx, flows, mapLoader{dataentryconfig.ConfigFile: form}, meta))
	require.ErrorContains(t, CheckFlowDataEntry(ctx, flows, mapLoader{}, meta),
		`declares no form "collect_documents"`, "an absent data-entry.yaml declares no form")
	require.ErrorContains(t, CheckFlowDataEntry(ctx, flows, mapLoader{dataentryconfig.ConfigFile: "forms: ["}, meta),
		"parse data-entry.yaml")
	require.NoError(t, CheckFlowDataEntry(ctx, nil, mapLoader{}, meta), "no flows name nothing")
}

// TestStepFormSet_Fields pins that a step resolves its form from
// data-entry.yaml directly, so a form that only an older flow version names
// still serves the runs pinned to it (RR-707GV2F), and that a form that
// cannot be used is an error, never an empty step (RR-DWWR4SB).
func TestStepFormSet_Fields(t *testing.T) {
	_, meta := stepFormSample(t)
	set := stepFormSet{meta: meta, forms: map[string]dataentryconfig.Form{
		"old_form": {EntityType: "person", Fields: []dataentryconfig.FormField{{Property: "documents"}}},
	}}
	got, err := set.fields("old_form", "person")
	require.NoError(t, err)
	require.Equal(t, []string{"documents"}, got, "a form no current flow names")

	got, err = set.fields("", "person")
	require.NoError(t, err)
	require.Empty(t, got, "a state without a form sends nothing")

	set.forms["alias_form"] = dataentryconfig.Form{EntityType: "pers",
		Fields: []dataentryconfig.FormField{{Property: "documents"}}}
	got, err = set.fields("alias_form", "person")
	require.NoError(t, err, "a form naming the subject type by its alias")
	require.Equal(t, []string{"documents"}, got)

	_, err = set.fields("gone", "person")
	require.ErrorIs(t, err, errStepForm)
	_, err = set.fields("old_form", "task")
	require.ErrorIs(t, err, errStepForm)
	_, err = stepFormSource{meta: meta}.load(context.Background()).fields("old_form", "person")
	require.ErrorIs(t, err, errStepForm, "no data-entry.yaml declares no form")
}

func TestCheckFlowsData_StepForms(t *testing.T) {
	read := func(name string) []byte {
		data, err := os.ReadFile(filepath.Join("..", "flowcfg", "testdata", name))
		require.NoError(t, err)
		return data
	}
	meta, err := metamodel.Parse(read("schema.yaml"))
	require.NoError(t, err)
	policy, err := acl.ParsePolicy(read("acl.yaml"), "acl.yaml")
	require.NoError(t, err)
	flows := read("flows.yaml")

	require.NoError(t, CheckFlowsData(flows, read("data-entry.yaml"), meta, policy))

	err = CheckFlowsData(flows, []byte("forms: {}\n"), meta, policy)
	require.ErrorContains(t, err, `flows.yaml: flow "onboarding" state "collect"`)
	require.ErrorContains(t, err, `declares no form "collect_documents"`)

	wrongType := []byte("forms:\n  collect_documents:\n    entity_type: task\n    fields: [{property: title}]\n")
	require.ErrorContains(t, CheckFlowsData(flows, wrongType, meta, policy), `edits "task"`)

	require.NoError(t, CheckFlowsData(flows, []byte("forms: [\n"), meta, policy),
		"data-entry.yaml's own check reports a parse error")
}

// failingLoader fails every read with an error that names a server path.
type failingLoader struct{}

func (failingLoader) Load(context.Context, string) ([]byte, error) {
	return nil, errors.New("open /srv/secret/project/data-entry.yaml: permission denied")
}

func (failingLoader) List(context.Context, string) ([]string, error) { return nil, nil }

// TestStepFormSource_ReadErrorNamesNoPath pins that a run reader sees a
// fixed message when data-entry.yaml cannot be read, not the raw cause.
func TestStepFormSource_ReadErrorNamesNoPath(t *testing.T) {
	_, meta := stepFormSample(t)
	_, err := stepFormSource{cfg: failingLoader{}, meta: meta}.load(context.Background()).
		fields("collect_documents", "person")
	require.ErrorIs(t, err, errStepForm)
	require.NotContains(t, err.Error(), "/srv/secret")
	require.Contains(t, err.Error(), "cannot be read")
}
