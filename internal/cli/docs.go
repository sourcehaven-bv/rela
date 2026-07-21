package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/docs"
)

// DocsCmd is `rela docs` — the doc-language build tool.
type DocsCmd struct {
	Build DocsBuildCmd `cmd:"" help:"Render a manual (Markdown with rela Lua islands) to resolved Markdown."`
}

// DocsBuildCmd is `rela docs build <manual.md>`. It resolves the manual's Lua
// islands against the project's metamodel + acl.yaml (and a seeded in-memory
// graph the manual populates) and writes the resulting Markdown.
type DocsBuildCmd struct {
	Manual string `arg:"" help:"Path to the manual (Markdown with rela Lua islands)."`
	Output string `name:"out" help:"Write resolved Markdown here (default: stdout)."`
	Strict bool   `help:"Fail the build if any island resolves to nothing."`
}

// Run resolves the manual and writes the output.
func (c *DocsBuildCmd) Run(ctx context.Context, svc *readServices) error {
	// The manual is an operator-supplied path (a document, not a project
	// script), typically outside the project root — read it directly, same
	// trust boundary as `pandoc in.md -o out`.
	src, err := os.ReadFile(c.Manual)
	if err != nil {
		return fmt.Errorf("read manual: %w", err)
	}

	// acl.yaml is optional; roles_matrix degrades to a note when absent.
	var policy *acl.Policy
	policyPath := filepath.Join(svc.Paths.Root, "acl.yaml")
	if p, perr := acl.LoadPolicy(policyPath); perr == nil {
		policy = p
	} else if !errors.Is(perr, os.ErrNotExist) {
		return fmt.Errorf("load acl.yaml: %w", perr)
	}

	rendered, err := docs.Build(ctx, string(src), docs.Options{
		Meta:   svc.Meta,
		Policy: policy,
		Strict: c.Strict,
	})
	if err != nil {
		return err
	}

	if c.Output == "" {
		fmt.Print(rendered)
		return nil
	}
	if info, statErr := os.Stat(c.Output); statErr == nil && info.IsDir() {
		return fmt.Errorf("--output %q is a directory", c.Output)
	}
	if dir := filepath.Dir(c.Output); dir != "" {
		if mkErr := os.MkdirAll(dir, 0o755); mkErr != nil {
			return fmt.Errorf("create output dir: %w", mkErr)
		}
	}
	if err := os.WriteFile(c.Output, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
