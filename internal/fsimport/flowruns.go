package fsimport

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
	"github.com/Sourcehaven-BV/rela/internal/flowrun"
	"github.com/Sourcehaven-BV/rela/internal/flowrun/kvflowrun"
	"github.com/Sourcehaven-BV/rela/internal/metamodel"
	"github.com/Sourcehaven-BV/rela/internal/storage"
)

// errNoRunTarget fails an import of unfinished flow runs into a target
// that has no run store to take them.
var errNoRunTarget = errors.New("this import cannot copy flow runs")

// withSourceFlows adds the `_flow-run` type to meta when the source's
// flows.yaml declares flows, as appbuild does at boot, so the run headers
// and their edges are read and copied like any other row.
func withSourceFlows(ro storage.FS, root string, meta *metamodel.Metamodel) (*metamodel.Metamodel, bool, error) {
	data, err := ro.ReadFile(filepath.Join(root, flowcfg.FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return meta, false, nil
	}
	if err != nil {
		return nil, false, fmt.Errorf("source %s: %w", flowcfg.FileName, err)
	}
	if !flowcfg.DeclaresFlows(data) {
		return meta, false, nil
	}
	m, err := metamodel.WithFlowRuns(meta)
	if err != nil {
		return nil, false, fmt.Errorf("source schema: %w", err)
	}
	return m, true, nil
}

// sourceRuns opens the source's run store: the KV document under .rela/
// the file tier keeps. The source is read-only and its fingerprint guards
// the import, so no lock is taken.
func sourceRuns(src *source) (flowrun.Store, error) {
	return kvflowrun.New(src.state, noLock{})
}

type noLock struct{}

func (noLock) Lock(context.Context) (func(), error) { return func() {}, nil }

// copyFlowRuns copies every unfinished run with its definition. A run's id,
// subject, state and pending mark are kept; its revision starts again at 1.
func (c *copier) copyFlowRuns(ctx context.Context) error {
	if c.src.runs == nil {
		return nil
	}
	var runs []flowrun.Run
	q := flowrun.Query{Limit: flowrun.MaxPage}
	for {
		page, err := c.src.runs.List(ctx, q)
		if err != nil {
			return fmt.Errorf("read flow runs: %w", err)
		}
		runs = append(runs, page...)
		if len(page) < q.Limit {
			break
		}
		q.After = page[len(page)-1].ID
	}
	if len(runs) == 0 {
		return nil
	}
	if !c.src.flows {
		// Without flows the headers are not read either, so the runs
		// cannot be copied; say why rather than drop them.
		c.rep.Errors = append(c.rep.Errors, fmt.Sprintf(
			"the source has %d unfinished flow run(s) but its %s declares no flows; restore it first",
			len(runs), flowcfg.FileName))
		return nil
	}
	if c.dst.FlowRuns == nil {
		c.rep.Errors = append(c.rep.Errors, fmt.Sprintf(
			"%v: the source has %d unfinished flow run(s); finish or cancel them first (rela flow runs)",
			errNoRunTarget, len(runs)))
		return nil
	}
	for _, r := range runs {
		def, err := c.src.runs.Definition(ctx, r.Flow, r.Version)
		if err != nil {
			c.rep.Errors = append(c.rep.Errors, fmt.Sprintf("flow run %s: definition: %v", r.ID, err))
			continue
		}
		_, err = c.dst.FlowRuns.Create(ctx, flowrun.NewRun{
			ID: r.ID, Flow: r.Flow, Version: r.Version, Definition: def,
			Subject: r.Subject, Singleton: r.Singleton, At: r.CreatedAt,
			Instance: r.Instance, Pending: r.Pending,
		})
		if err != nil {
			c.rep.Errors = append(c.rep.Errors, fmt.Sprintf("flow run %s: %v", r.ID, err))
			continue
		}
		c.rep.FlowRuns++
	}
	return nil
}
