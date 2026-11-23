package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"slices"

	"gopkg.in/yaml.v3"

	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/dataentryconfig"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// stepForms maps the name of a data-entry.yaml form that a flow state names
// in `form:` to the subject properties a step through it may send: the
// form's fields that are not hidden, in form order. The checks of a
// configuration use it; a running step reads its form through
// [stepFormSource].
//
// A step writes subject properties only, so a form a step uses has no
// relations, no wizard steps and no body. `required_when` and `visible_when`
// stay with the SPA, as they do for any form save; the server only bounds
// which properties a step may send.
type stepForms map[string][]string

// fields returns the properties a step through form name may send. A form
// the map does not know accepts none.
func (f stepForms) fields(name string) []string {
	if name == "" {
		return nil
	}
	return slices.Clone(f[name])
}

// errStepForm marks a step whose form cannot be used: data-entry.yaml no
// longer declares it, or no longer lets a step write through it. The step
// is refused rather than completed with nothing written.
var errStepForm = errors.New("the step's form cannot be used")

// stepFormSource reads the step forms from data-entry.yaml as it is when a
// step is shown or completed. So an edit of data-entry.yaml applies to the
// next step without a restart, and a run pinned to an older flow version
// finds the form its state names.
type stepFormSource struct {
	// cfg reads data-entry.yaml. Nil: accepted; then no form is declared.
	cfg  config.Loader
	meta *metamodel.Metamodel
}

// load reads the forms once, for one operation. A read or parse error is
// kept, and every form then reports it.
func (s stepFormSource) load(ctx context.Context) stepFormSet {
	if s.cfg == nil {
		return stepFormSet{meta: s.meta}
	}
	forms, err := loadForms(ctx, s.cfg)
	var rerr *formReadError
	if errors.As(err, &rerr) {
		// Every reader of the run sees this error, so it names the file
		// only; the cause, which may hold a server path, goes to the log.
		slog.Warn("flows: read step forms", "file", dataentryconfig.ConfigFile, "err", rerr.cause)
		err = fmt.Errorf("flows: %s cannot be read", dataentryconfig.ConfigFile)
	}
	return stepFormSet{forms: forms, meta: s.meta, err: err}
}

// formReadError is a failure to read data-entry.yaml, as opposed to a
// failure to parse it.
type formReadError struct{ cause error }

func (e *formReadError) Error() string {
	return fmt.Sprintf("flows: read %s: %v", dataentryconfig.ConfigFile, e.cause)
}

func (e *formReadError) Unwrap() error { return e.cause }

// stepFormSet is data-entry.yaml's forms as one operation read them.
type stepFormSet struct {
	forms map[string]dataentryconfig.Form
	meta  *metamodel.Metamodel
	err   error
}

// fields returns the properties a step through form name may send, on a
// subject of the flow subject type subject. A state without a form sends
// nothing. The error wraps [errStepForm].
func (s stepFormSet) fields(name, subject string) ([]string, error) {
	if name == "" {
		return nil, nil
	}
	if s.err != nil {
		return nil, fmt.Errorf("%w: %w", errStepForm, s.err)
	}
	form, ok := s.forms[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s declares no form %q", errStepForm, dataentryconfig.ConfigFile, name)
	}
	out, err := stepFormFields(name, form, subject, s.meta)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errStepForm, err)
	}
	return out, nil
}

// parseForms reads the forms of data-entry.yaml. Empty data declares none.
func parseForms(data []byte) (map[string]dataentryconfig.Form, error) {
	var doc struct {
		Forms map[string]dataentryconfig.Form `yaml:"forms"`
	}
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("flows: parse %s: %w", dataentryconfig.ConfigFile, err)
	}
	if doc.Forms == nil {
		doc.Forms = map[string]dataentryconfig.Form{}
	}
	return doc.Forms, nil
}

// loadForms reads the forms of data-entry.yaml through cfg. An absent file
// declares none.
func loadForms(ctx context.Context, cfg config.Loader) (map[string]dataentryconfig.Form, error) {
	data, err := cfg.Load(ctx, dataentryconfig.ConfigFile)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]dataentryconfig.Form{}, nil
	}
	if err != nil {
		return nil, &formReadError{cause: err}
	}
	return parseForms(data)
}

// resolveStepForms checks the `form:` of every state of flows against forms
// and returns the properties each named form lets a step send. It refuses a
// form data-entry.yaml does not declare, one for another type than the
// flow's subject, and one a step cannot write through (see [stepForms]).
func resolveStepForms(flows *flowcfg.Compiled, forms map[string]dataentryconfig.Form,
	meta *metamodel.Metamodel) (stepForms, error) {
	out := stepForms{}
	if flows == nil {
		return out, nil
	}
	for _, f := range flows.Flows() {
		for _, st := range f.States() {
			name := st.Form()
			if name == "" {
				continue
			}
			form, ok := forms[name]
			if !ok {
				return nil, fmt.Errorf("flow %q state %q: %s declares no form %q", f.Name(), st.Name,
					dataentryconfig.ConfigFile, name)
			}
			fields, err := stepFormFields(name, form, f.Subject(), meta)
			if err != nil {
				return nil, fmt.Errorf("flow %q state %q: %w", f.Name(), st.Name, err)
			}
			out[name] = fields
		}
	}
	return out, nil
}

// stepFormFields returns the properties a step may send through form name,
// on a subject of type subject, or why the form cannot serve a step. A type
// alias names the same type as its canonical name.
func stepFormFields(name string, form dataentryconfig.Form, subject string,
	meta *metamodel.Metamodel) ([]string, error) {
	switch {
	case meta.ResolveAlias(form.EntityType) != meta.ResolveAlias(subject):
		return nil, fmt.Errorf("form %q edits %q, not the flow's subject %q", name, form.EntityType, subject)
	case len(form.Steps) > 0:
		return nil, fmt.Errorf("form %q is a wizard; a step form has one page", name)
	case len(form.Relations) > 0:
		return nil, fmt.Errorf("form %q edits relations; a step writes subject properties only", name)
	case form.Body != nil && *form.Body:
		return nil, fmt.Errorf("form %q edits the body; a step writes subject properties only", name)
	}
	def, ok := meta.GetEntityDef(subject)
	if !ok {
		return nil, fmt.Errorf("form %q: the schema declares no type %q", name, subject)
	}
	out := make([]string, 0, len(form.Fields))
	for _, fd := range form.Fields {
		prop := fd.Property
		if prop == "" || fd.Hidden || slices.Contains(out, prop) {
			continue
		}
		pd, ok := def.Properties[prop]
		switch {
		case metamodel.IsReservedName(prop):
			return nil, fmt.Errorf("form %q: %q is reserved; a step cannot write it", name, prop)
		case !ok:
			return nil, fmt.Errorf("form %q: %s has no property %q", name, subject, prop)
		case pd.Computed != "" || pd.Rollup != nil:
			return nil, fmt.Errorf("form %q: %s.%s is derived; a step cannot write it", name, subject, prop)
		}
		out = append(out, prop)
	}
	return out, nil
}

// checkFlowForms resolves the step forms of flows against data-entry.yaml,
// read through cfg. It reads the file only when some state names a form.
func checkFlowForms(ctx context.Context, flows *flowcfg.Compiled, cfg config.Loader,
	meta *metamodel.Metamodel) error {
	if !namesForm(flows) {
		return nil
	}
	forms, err := loadForms(ctx, cfg)
	if err != nil {
		return err
	}
	_, err = resolveStepForms(flows, forms, meta)
	return err
}

// namesForm reports whether some state of flows names a step form.
func namesForm(flows *flowcfg.Compiled) bool {
	if flows == nil {
		return false
	}
	for _, f := range flows.Flows() {
		for _, st := range f.States() {
			if st.Form() != "" {
				return true
			}
		}
	}
	return false
}

// CheckFlowDataEntry checks what the flows of flows name in data-entry.yaml,
// read through cfg: the actions of job steps and the forms of steps. The
// server refuses to build the flow engine on the same errors, so `rela flow
// validate` reports them before an upgrade or a deploy.
func CheckFlowDataEntry(ctx context.Context, flows *flowcfg.Compiled, cfg config.Loader,
	meta *metamodel.Metamodel) error {
	if err := checkFlowActions(ctx, flows, cfg); err != nil {
		return err
	}
	return checkFlowForms(ctx, flows, cfg, meta)
}
