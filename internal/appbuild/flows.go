package appbuild

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
)

// LoadFlows reads flows.yaml through r and compiles it against meta and
// policy. It returns (nil, nil) when the file is absent. The caller reports
// the warnings. The policy must have passed [ValidateACLPolicy], which
// expands abstract type names: a visible: grant on an unexpanded abstract
// type would not match the subject type, and the field checks would pass.
//
// Nil: meta is rejected; policy is accepted (no acl.yaml).
func LoadFlows(ctx context.Context, r configReader, meta *metamodel.Metamodel,
	policy *acl.Policy,
) (*flowcfg.Compiled, error) {
	if meta == nil {
		return nil, errors.New("flows: LoadFlows requires a metamodel")
	}
	data, err := r.Load(ctx, flowcfg.FileName)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) || os.IsNotExist(err) {
			return nil, nil //nolint:nilnil // (nil, nil) is "no flows.yaml": no flows
		}
		return nil, fmt.Errorf("read %s: %w", flowcfg.FileName, err)
	}
	return flowcfg.Parse(data, meta, policy)
}

// CheckFlowsData compiles the `flows:` section of flows.yaml content against
// meta and policy, as a server build does, and returns the compile error.
// It also checks each state's step form against the forms of dataEntry,
// data-entry.yaml's content. The error text starts with the file name.
// Both need not be on disk yet, such as a Configure draft; nil, or a file
// without `flows:`, passes.
//
// Nil: meta is rejected; policy is accepted (no acl.yaml).
func CheckFlowsData(data, dataEntry []byte, meta *metamodel.Metamodel, policy *acl.Policy) error {
	if meta == nil {
		return errors.New("flows: CheckFlowsData requires a metamodel")
	}
	m, err := withFlowRuns(data, meta)
	if err != nil {
		return fmt.Errorf("%s: %w", flowcfg.FileName, err)
	}
	compiled, err := compileFlows(data, m, policy)
	if err != nil || !namesForm(compiled) {
		return err
	}
	forms, err := parseForms(dataEntry)
	if err != nil {
		return nil //nolint:nilerr // data-entry.yaml's own check reports a parse error
	}
	if _, err := resolveStepForms(compiled, forms, m); err != nil {
		return fmt.Errorf("%s: %w", flowcfg.FileName, err)
	}
	return nil
}
