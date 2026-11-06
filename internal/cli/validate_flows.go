package cli

import (
	"context"
	stderrors "errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/config"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/projectsetup"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// reportFlowsValidation loads flows.yaml when the project has one,
// compiled against the schema the earlier check loaded and acl.yaml, as
// `rela flow validate` does. Each problem is a ✗ line. It needs a schema
// that loaded, and returns hasErrors, set when the check failed.
func reportFlowsValidation(ctx context.Context, result *projectsetup.ValidateResult, hasErrors bool) bool {
	if result.Metamodel == nil {
		return hasErrors
	}
	root := result.ProjectRoot
	// #nosec G703 -- path is <discovered project root>/flows.yaml.
	if _, err := os.Stat(filepath.Join(root, flowcfg.FileName)); err != nil {
		return hasErrors
	}
	if !quiet {
		fmt.Printf("Validating %s...\n", flowcfg.FileName)
	}
	policy, err := acl.LoadPolicy(filepath.Join(root, "acl.yaml"))
	if err != nil && !stderrors.Is(err, fs.ErrNotExist) {
		fmt.Printf("  ✗ %s: acl.yaml: %v\n", flowcfg.FileName, err)
		return true
	}
	// Expand abstract type names, as the server does. Without the
	// expansion the visible: checks would miss grants, so a policy that
	// does not validate skips this check; the acl.yaml check reports it.
	if policy != nil && appbuild.ValidateACLPolicy(policy, result.Metamodel) != nil {
		fmt.Printf("  ⚠ %s: skipped, acl.yaml does not validate\n", flowcfg.FileName)
		return hasErrors
	}
	loader := config.NewFSLoader(storage.NewSafeFS(storage.NewOsFS()), root)
	flows, err := appbuild.LoadFlows(ctx, loader, result.Metamodel, policy)
	var loadErr *flowcfg.Error
	switch {
	case stderrors.As(err, &loadErr):
		for _, p := range loadErr.Problems {
			fmt.Printf("  ✗ %s: %s\n", flowcfg.FileName, p)
		}
		return true
	case err != nil:
		fmt.Printf("  ✗ %v\n", err)
		return true
	}
	for _, w := range flows.Warnings() {
		fmt.Printf("  ⚠ %s: %s\n", flowcfg.FileName, w)
	}
	if !quiet {
		fmt.Printf("  ✓ %s is valid\n", flowcfg.FileName)
	}
	return hasErrors
}
