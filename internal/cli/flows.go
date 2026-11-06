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
	Name string `arg:"" optional:"" help:"Flow to render. Without one, list the flows."`
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

// Run validates flows.yaml.
func (c *FlowValidateCmd) Run(ctx context.Context, svc *readServices) error {
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
		out.WriteSuccess("No %s found; the project declares no flows.", flowcfg.FileName)
		return nil
	}
	for _, w := range flows.Warnings() {
		out.WriteWarning("%s", w)
	}
	out.WriteSuccess("%s is valid: %d flow(s).", flowcfg.FileName, len(flows.Flows()))
	return nil
}

// flowSummary is one flow in `rela flow show` output.
type flowSummary struct {
	Name    string   `json:"name"`
	Label   string   `json:"label"`
	Subject string   `json:"subject"`
	Version string   `json:"version"`
	Initial string   `json:"initial"`
	States  []string `json:"states"`
	Mermaid string   `json:"mermaid,omitempty"`
}

func summarize(f *flowcfg.Flow, withDiagram bool) flowSummary {
	s := flowSummary{Name: f.Name(), Label: f.Label(), Subject: f.Subject(), Version: f.Version(), Initial: f.Initial()}
	for _, st := range f.States() {
		s.States = append(s.States, st.Name)
	}
	if withDiagram {
		s.Mermaid = f.Mermaid()
	}
	return s
}

// Run lists the flows, or renders the named one.
func (c *FlowShowCmd) Run(ctx context.Context, svc *readServices) error {
	flows, err := loadProjectFlows(ctx, svc)
	if err != nil {
		return err
	}
	return c.write(os.Stdout, flows)
}

func (c *FlowShowCmd) write(w io.Writer, flows *flowcfg.Compiled) error {
	jsonOut := out.Format == output.FormatJSON
	if c.Name == "" {
		all := flows.Flows()
		list := make([]flowSummary, 0, len(all))
		for _, f := range all {
			list = append(list, summarize(f, false))
		}
		if jsonOut {
			return json.NewEncoder(w).Encode(list)
		}
		if len(list) == 0 {
			_, err := fmt.Fprintln(w, "No flows declared.")
			return err
		}
		for _, s := range list {
			if _, err := fmt.Fprintf(w, "%s\t%s\tsubject: %s\tversion: %s\tstates: %s\n", s.Name, s.Label, s.Subject,
				s.Version, strings.Join(s.States, ", ")); err != nil {
				return err
			}
		}
		return nil
	}
	f, ok := flows.Flow(c.Name)
	if !ok {
		return fmt.Errorf("no flow %q in %s", c.Name, flowcfg.FileName)
	}
	if jsonOut {
		return json.NewEncoder(w).Encode(summarize(f, true))
	}
	_, err := io.WriteString(w, f.Mermaid())
	return err
}
