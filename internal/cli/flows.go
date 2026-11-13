package cli

import (
	"context"
	"encoding/json"
	stderrors "errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/flowcfg/transitions"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/output"
)

// FlowValidateCmd is `rela flow validate`: load flows.yaml as the server
// would, and report every problem.
type FlowValidateCmd struct{}

// FlowShowCmd is `rela flow show [name]`: list the flows, or render one as
// a Mermaid state diagram.
type FlowShowCmd struct {
	Name string `arg:"" optional:"" help:"Flow to render; a headless flow is schema.<enum type>. Without one, list the flows."`
}

// loadProjectFlows compiles flows.yaml for the project of svc, against its
// schema and acl.yaml, as boot does. It returns nil without an error when
// the project has no flows.yaml.
func loadProjectFlows(ctx context.Context, svc *readServices) (*flowcfg.Compiled, error) {
	var policy *acl.Policy
	data, err := svc.Config.Load(ctx, "acl.yaml")
	switch {
	case err == nil:
		if policy, err = acl.ParsePolicy(data, "acl.yaml"); err != nil {
			return nil, fmt.Errorf("acl.yaml: %w", err)
		}
		if err = appbuild.ValidateACLPolicy(policy, svc.Meta); err != nil {
			return nil, fmt.Errorf("acl.yaml invalid for this project: %w", err)
		}
	case !stderrors.Is(err, fs.ErrNotExist) && !os.IsNotExist(err):
		return nil, fmt.Errorf("read acl.yaml: %w", err)
	}
	return appbuild.LoadFlows(ctx, svc.Config, svc.Meta, policy)
}

// loadHeadlessFlows compiles the transitions of the project's schema into
// headless flows, as boot does.
func loadHeadlessFlows(svc *readServices) (*transitions.Transitions, error) {
	t, err := appbuild.CompileHeadlessFlows(svc.Meta, svc.Store)
	if err != nil {
		return nil, fmt.Errorf("schema transitions: %w", err)
	}
	return t, nil
}

// Run validates flows.yaml and the headless flows of the schema. It checks
// both before it returns, so one broken source does not hide the problems of
// the other.
func (c *FlowValidateCmd) Run(ctx context.Context, svc *readServices) error {
	headless, headlessErr := loadHeadlessFlows(svc)
	if headlessErr != nil {
		out.WriteError("%s", headlessErr)
	}
	fileErr := c.validateFile(ctx, svc)
	if headlessErr != nil {
		return stderrors.Join(headlessErr, fileErr)
	}
	if fileErr != nil {
		return fileErr
	}
	if n := len(headless.Headless()); n > 0 {
		out.WriteSuccess("Schema transitions are valid: %d headless flow(s).", n)
	}
	return nil
}

// validateFile loads flows.yaml and reports its problems.
func (c *FlowValidateCmd) validateFile(ctx context.Context, svc *readServices) error {
	flows, err := loadProjectFlows(ctx, svc)
	if err != nil {
		var loadErr *flowcfg.Error
		if stderrors.As(err, &loadErr) {
			for _, p := range loadErr.Problems {
				out.WriteError("%s", p)
			}
			return fmt.Errorf("%s: %d problem(s)", flowcfg.FileName, len(loadErr.Problems))
		}
		return err
	}
	if flows == nil {
		out.WriteSuccess("No %s found.", flowcfg.FileName)
		return nil
	}
	for _, w := range flows.Warnings() {
		out.WriteWarning("%s", w)
	}
	out.WriteSuccess("%s is valid: %d flow(s).", flowcfg.FileName, len(flows.Flows()))
	return nil
}

// The kinds of flow in `rela flow show` output.
const (
	flowKindFlow     = "flow"
	flowKindHeadless = "headless"
)

// flowSummary is one flow in `rela flow show` output. A flows.yaml flow has
// a subject and a version; a headless flow has the entity properties that
// follow it (uses) instead, because one enum type can serve several.
type flowSummary struct {
	Name    string   `json:"name"`
	Kind    string   `json:"kind"`
	Label   string   `json:"label"`
	Subject string   `json:"subject,omitempty"`
	Uses    []string `json:"uses,omitempty"`
	Version string   `json:"version,omitempty"`
	Initial string   `json:"initial"`
	States  []string `json:"states"`
	Mermaid string   `json:"mermaid,omitempty"`
}

func summarize(f *flowcfg.Flow, withDiagram bool) flowSummary {
	s := flowSummary{
		Name: f.Name(), Kind: flowKindFlow, Label: f.Label(), Subject: f.Subject(), Version: f.Version(),
		Initial: f.Initial(),
	}
	for _, st := range f.States() {
		s.States = append(s.States, st.Name)
	}
	if withDiagram {
		s.Mermaid = f.Mermaid()
	}
	return s
}

func summarizeHeadless(t *transitions.Transitions, f *transitions.HeadlessFlow, withDiagram bool) flowSummary {
	s := flowSummary{
		Name: transitions.HeadlessPrefix + f.Name(), Kind: flowKindHeadless, Label: f.Name(),
		Uses: t.Uses(f.Name()), Initial: f.Initial(), States: f.States(),
	}
	if withDiagram {
		s.Mermaid = f.Mermaid()
	}
	return s
}

// Run lists the flows, or renders the named one.
func (c *FlowShowCmd) Run(ctx context.Context, svc *readServices) error {
	return c.run(ctx, svc, os.Stdout)
}

// run writes to w. The headless flows load on their own, so a text list
// still shows them when flows.yaml does not load, and then returns its
// error. A JSON list writes nothing in that case: a script that reads
// stdout must not take a partial list for the whole one.
func (c *FlowShowCmd) run(ctx context.Context, svc *readServices, w io.Writer) error {
	headless, err := loadHeadlessFlows(svc)
	if err != nil {
		return err
	}
	if strings.HasPrefix(c.Name, transitions.HeadlessPrefix) {
		return c.write(w, nil, headless)
	}
	flows, flowsErr := loadProjectFlows(ctx, svc)
	if flowsErr != nil && (c.Name != "" || out.Format == output.FormatJSON) {
		return flowsErr
	}
	if err := c.write(w, flows, headless); err != nil {
		return err
	}
	return flowsErr
}

func (c *FlowShowCmd) write(w io.Writer, flows *flowcfg.Compiled, headless *transitions.Transitions) error {
	jsonOut := out.Format == output.FormatJSON
	if c.Name == "" {
		return writeFlowList(w, flows, headless, jsonOut)
	}
	s, ok := c.find(flows, headless)
	if !ok {
		if _, isHeadless := headless.HeadlessFlow(c.Name); isHeadless {
			return fmt.Errorf("no flow %q in %s; the schema transitions of %s are flow %q",
				c.Name, flowcfg.FileName, c.Name, transitions.HeadlessPrefix+c.Name)
		}
		return fmt.Errorf("no flow %q in %s or in the schema transitions", c.Name, flowcfg.FileName)
	}
	if jsonOut {
		return json.NewEncoder(w).Encode(s)
	}
	_, err := io.WriteString(w, s.Mermaid)
	return err
}

// find looks the name up among the headless flows when it carries
// [transitions.HeadlessPrefix], and among the flows of flows.yaml otherwise.
func (c *FlowShowCmd) find(flows *flowcfg.Compiled, headless *transitions.Transitions) (flowSummary, bool) {
	if name, ok := strings.CutPrefix(c.Name, transitions.HeadlessPrefix); ok {
		f, found := headless.HeadlessFlow(name)
		if !found {
			return flowSummary{}, false
		}
		return summarizeHeadless(headless, f, true), true
	}
	f, ok := flows.Flow(c.Name)
	if !ok {
		return flowSummary{}, false
	}
	return summarize(f, true), true
}

func writeFlowList(w io.Writer, flows *flowcfg.Compiled, headless *transitions.Transitions, jsonOut bool) error {
	all, hl := flows.Flows(), headless.Headless()
	list := make([]flowSummary, 0, len(all)+len(hl))
	for _, f := range all {
		list = append(list, summarize(f, false))
	}
	for _, f := range hl {
		list = append(list, summarizeHeadless(headless, f, false))
	}
	if jsonOut {
		return json.NewEncoder(w).Encode(list)
	}
	if len(list) == 0 {
		_, err := fmt.Fprintln(w, "No flows declared.")
		return err
	}
	for _, s := range list {
		var err error
		if s.Kind == flowKindHeadless {
			_, err = fmt.Fprintf(w, "%s\theadless\tuses: %s\tinitial: %s\tstates: %s\n", s.Name,
				strings.Join(s.Uses, ", "), s.Initial, strings.Join(s.States, ", "))
		} else {
			_, err = fmt.Fprintf(w, "%s\t%s\tsubject: %s\tversion: %s\tstates: %s\n", s.Name, s.Label, s.Subject,
				s.Version, strings.Join(s.States, ", "))
		}
		if err != nil {
			return err
		}
	}
	return nil
}
