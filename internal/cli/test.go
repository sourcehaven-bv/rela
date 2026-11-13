package cli

import (
	"context"
	stderrors "errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	"github.com/Sourcehaven-BV/rela/internal/apptest"
	relaerrors "github.com/Sourcehaven-BV/rela/internal/errors"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/project"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// TestCmd runs the project's `*.test.lua` files, each case on its own
// in-memory world. It opens no store, so it runs beside a server that holds
// the project's database.
type TestCmd struct {
	Paths  []string `arg:"" optional:"" help:"Test files or directories (default: tests/)."`
	Filter string   `name:"run" help:"Run only the cases whose name matches this regular expression."`
	Seed   *uint64  `help:"Seed for generated values and clocks; the run prints the seed it used."`

	RequireTests bool `name:"require-tests" help:"Fail when no test file is found."`
}

// Run runs the tests and exits 1 when a case fails.
func (c *TestCmd) Run(ctx context.Context) error {
	start := projectPath
	if start == "" {
		start = os.Getenv("RELA_PROJECT")
	}
	pctx, err := project.Discover(start, storage.NewSafeFS(storage.NewOsFS()))
	if err != nil {
		return relaerrors.WrapDiscoverError(err)
	}
	if !verbose {
		// Each case builds a world, which logs at info and repeats the same
		// config warnings; that drowns the report. Show each warning once.
		prev := slog.Default()
		level := &slog.HandlerOptions{Level: slog.LevelWarn}
		slog.SetDefault(slog.New(&onceHandler{Handler: slog.NewTextHandler(os.Stderr, level), seen: &sync.Map{}}))
		defer slog.SetDefault(prev)
	}
	paths, err := resolveTestPaths(pctx.Root, c.Paths)
	if err != nil {
		return err
	}
	opts := apptest.Options{Root: pctx.Root, Paths: paths, Run: c.Filter}
	if c.Seed != nil {
		opts.Seed, opts.SeedSet = *c.Seed, true
	}
	rep, err := apptest.Run(ctx, opts)
	if stderrors.Is(err, apptest.ErrNoTests) {
		where := apptest.TestsDir + "/"
		if len(c.Paths) > 0 {
			where = strings.Join(c.Paths, ", ")
		}
		if c.RequireTests {
			return fmt.Errorf("no test files found in %s", where)
		}
		if out.Format == output.FormatJSON {
			return (&apptest.Report{Cases: []apptest.CaseResult{}}).WriteJSON(out.Out)
		}
		if !quiet {
			fmt.Fprintf(out.Out, "no test files found (looked in %s)\n", where)
		}
		return nil
	}
	if err != nil {
		return err
	}
	if out.Format == output.FormatJSON {
		err = rep.WriteJSON(out.Out)
	} else {
		err = rep.WriteText(out.Out)
	}
	if err != nil {
		return err
	}
	if !rep.OK() {
		return relaerrors.NewExitError(1)
	}
	return nil
}

// resolveTestPaths reads each relative path that names an existing file or
// directory from the working directory, and returns it relative to root,
// symlinks resolved on both sides. Any other path is kept as given, and the
// runner reads it relative to root, so `rela --project p test
// tests/x.test.lua` also works from outside the project.
func resolveTestPaths(root string, paths []string) ([]string, error) {
	out := slices.Clone(paths)
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	for i, p := range paths {
		if filepath.IsAbs(p) {
			continue
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return nil, err
		}
		if abs, err = filepath.EvalSymlinks(abs); err != nil {
			continue // not under the working directory; root-relative
		}
		if rel, err := filepath.Rel(realRoot, abs); err == nil && filepath.IsLocal(rel) {
			out[i] = rel
		} else {
			out[i] = abs // outside the project; the runner refuses it by name
		}
	}
	return out, nil
}

// onceHandler drops a record whose level and message it has already passed
// on, so a warning every test world logs is shown once per run.
type onceHandler struct {
	slog.Handler
	seen *sync.Map
}

// Handle passes r on the first time its level and message are seen.
func (h *onceHandler) Handle(ctx context.Context, r slog.Record) error {
	if _, dup := h.seen.LoadOrStore(r.Level.String()+"\x00"+r.Message, true); dup {
		return nil
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs keeps the dedup across derived loggers.
func (h *onceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &onceHandler{Handler: h.Handler.WithAttrs(attrs), seen: h.seen}
}

// WithGroup keeps the dedup across derived loggers.
func (h *onceHandler) WithGroup(name string) slog.Handler {
	return &onceHandler{Handler: h.Handler.WithGroup(name), seen: h.seen}
}
