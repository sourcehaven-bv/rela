// Package docscli is the command layer for the standalone `rela-docs` binary:
// it resolves a Markdown manual's rela Lua islands (field tables, lifecycles,
// relation graphs, role matrices, and live `screenshot{}` captures) against a
// project's metamodel + acl.yaml.
//
// It lives outside internal/cli on purpose. The screenshot{} island drives a
// headless Chrome via internal/docscapture, which links chromedp (+ cdproto,
// gobwas — ~15 MB). Keeping that dependency here, reachable only from
// cmd/rela-docs, is what keeps it OUT of the default `rela` / `rela-server`
// binaries (CI asserts the isolation). The core doc engine (internal/docs)
// stays browser-free; the concrete Capturer is injected via the build-tagged
// NewCapturer seam in this package.
package docscli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/docs"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/project"
)

// Project is the minimal read-only view docs build needs: the resolved
// metamodel and the project paths (to locate acl.yaml and the on-disk schema
// the capture server copies). appbuild.Services satisfies this via its Meta()
// and Paths() accessors — the binary supplies it at the wiring site.
type Project interface {
	Meta() *metamodel.Metamodel
	Paths() *project.Context
}

// Root is the kong-parsed root for the rela-docs binary.
type Root struct {
	Build BuildCmd `cmd:"" help:"Render a manual (Markdown with rela Lua islands) to resolved Markdown."`
}

// BuildCmd is `rela-docs build <manual.md>`. It resolves the manual's Lua
// islands against the project's metamodel + acl.yaml (and a seeded in-memory
// graph the manual populates) and writes the resulting Markdown.
type BuildCmd struct {
	Manual string `arg:"" help:"Path to the manual (Markdown with rela Lua islands)."`
	Output string `name:"out" help:"Write resolved Markdown here (default: stdout)."`
	Strict bool   `help:"Fail the build if any island resolves to nothing."`
}

// newCapturer resolves the browser capturer. It defaults to the build-tagged
// NewCapturer (fsstore: a real capturer; postgres: fail-loud), and is a var so
// tests can inject a stub or force the "no browser" path deterministically,
// independent of whether Chrome is installed on the host.
var newCapturer = NewCapturer

// outputDir is the directory screenshot{} PNGs are written into: the output
// file's directory, or the current directory when writing to stdout.
func outputDir(out string) string {
	if out == "" {
		return "."
	}
	return filepath.Dir(out)
}

// Run resolves the manual and writes the output.
func (c *BuildCmd) Run(ctx context.Context, proj Project) error {
	// The manual is an operator-supplied path (a document, not a project
	// script), typically outside the project root — read it directly, same
	// trust boundary as `pandoc in.md -o out`.
	src, err := os.ReadFile(c.Manual)
	if err != nil {
		return fmt.Errorf("read manual: %w", err)
	}

	// acl.yaml is optional; roles_matrix degrades to a note when absent.
	var policy *acl.Policy
	policyPath := filepath.Join(proj.Paths().Root, "acl.yaml")
	if p, perr := acl.LoadPolicy(policyPath); perr == nil {
		policy = p
	} else if !errors.Is(perr, os.ErrNotExist) {
		return fmt.Errorf("load acl.yaml: %w", perr)
	}

	opts := docs.Options{
		Meta:       proj.Meta(),
		Policy:     policy,
		Strict:     c.Strict,
		ProjectDir: proj.Paths().Root,
		OutDir:     outputDir(c.Output),
	}
	// Wire a browser capturer for screenshot{} islands. If no browser is
	// available (or this build can't host the capture server — see the build-
	// tagged NewCapturer), leave it nil but keep the specific reason: a manual
	// WITHOUT screenshot{} still builds; one WITH it fails loud with the
	// actionable message. No graceful degradation.
	if capturer, capErr := newCapturer(); capErr == nil {
		opts.Capturer = capturer
	} else {
		opts.CapturerErr = capErr.Error()
	}

	rendered, err := docs.Build(ctx, string(src), opts)
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
	// filepath.Dir never returns "" (a bare filename yields "."), so the
	// MkdirAll is unconditional; on a bare name it no-ops on the cwd.
	if mkErr := os.MkdirAll(filepath.Dir(c.Output), 0o755); mkErr != nil {
		return fmt.Errorf("create output dir: %w", mkErr)
	}
	if err := os.WriteFile(c.Output, []byte(rendered), 0o644); err != nil {
		return fmt.Errorf("write output: %w", err)
	}
	return nil
}
