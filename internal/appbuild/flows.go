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
