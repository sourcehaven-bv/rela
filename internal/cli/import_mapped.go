package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/Sourcehaven-BV/rela/internal/mapimport"
)

// runMapped imports c.File through the mapping c.Mapping. The plan is
// computed from raw rows and gated per row with the visible reader; the
// writes go through the entity manager as the CLI principal.
func (c *ImportCmd) runMapped(ctx context.Context, w *writeServices) error {
	if w.LuaWriteDeps.VisibleReader == nil {
		return errors.New("import: no read gate is wired for this project")
	}
	if !mapimport.ValidName(c.Mapping) {
		return fmt.Errorf("invalid import name %q: use lowercase letters, digits, - and _", c.Mapping)
	}
	src, err := w.Config.Load(ctx, mapimport.FilePath(c.Mapping))
	if err != nil {
		return fmt.Errorf("read %s: %w", mapimport.FilePath(c.Mapping), err)
	}
	m, err := mapimport.Parse(c.Mapping, src, w.Meta)
	if err != nil {
		return err
	}
	lim := m.Limits()
	data, err := readImportFile(w, c.File, lim.MaxBytes)
	if err != nil {
		return err
	}
	format := mapimport.Format(c.Format)
	if format == "" {
		format = m.Format
	}
	if format == "" {
		format = mapimport.DetectFormat(c.File)
	}
	if format == "" {
		return fmt.Errorf("cannot tell the format of %s; set --format or format: in the mapping", c.File)
	}
	root, err := mapimport.Decode(data, format, lim)
	if err != nil {
		return fmt.Errorf("%s: %w", c.File, err)
	}
	plan, err := mapimport.Build(ctx, mapimport.Deps{
		Meta: w.Meta, Store: w.Store, Gate: w.LuaWriteDeps.VisibleReader, Files: w.Config,
	}, m, root)
	if err != nil {
		return err
	}
	var res *mapimport.Result
	if !c.DryRun && plan.HasChanges() && (len(plan.Errors) == 0 || c.SkipErrors) {
		res, err = mapimport.Apply(ctx, w.EntityManager, plan, mapimport.ApplyOptions{AllowErrors: c.SkipErrors})
		if err != nil {
			// A cancelled run has written part of the plan; say how much.
			printImportResult(res)
			return err
		}
	}
	if outputFormat == "json" {
		enc := json.NewEncoder(out.Out)
		enc.SetIndent("", "  ")
		if err := enc.Encode(struct {
			Plan   *mapimport.Plan   `json:"plan"`
			Result *mapimport.Result `json:"result,omitempty"`
		}{plan, res}); err != nil {
			return err
		}
	} else {
		printImportPlan(plan, c.DryRun)
		printImportResult(res)
	}
	switch {
	case len(plan.Errors) > 0 && !c.SkipErrors:
		if c.DryRun {
			return fmt.Errorf("import plan has %d record errors", len(plan.Errors))
		}
		return fmt.Errorf("import plan has %d record errors; nothing was written (use --skip-errors to apply the rest)",
			len(plan.Errors))
	case res != nil && len(res.Failures) > 0:
		return fmt.Errorf("import: %d writes failed", len(res.Failures))
	}
	return nil
}

// readImportFile reads at most limit bytes of path, and fails on a larger file
// without reading the rest.
func readImportFile(w *writeServices, path string, limit int) ([]byte, error) {
	f, err := w.FS.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, int64(limit)+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	if len(data) > limit {
		return nil, fmt.Errorf("%s is larger than %d bytes; raise max_bytes in the mapping", path, limit)
	}
	return data, nil
}

func printImportPlan(p *mapimport.Plan, dryRun bool) {
	verb := "Will"
	if dryRun {
		verb = "Would"
	}
	for _, c := range p.Creates {
		out.WriteInfo("+ %s %s", c.Type, entityLabel(c))
		for _, pc := range c.Properties {
			out.WriteInfo("    %s: %s", pc.Name, showValue(pc.New))
		}
	}
	for _, c := range p.Updates {
		out.WriteInfo("~ %s %s", c.Type, entityLabel(c))
		for _, pc := range c.Properties {
			if pc.Cleared {
				out.WriteInfo("    %s: %s -> (removed)", pc.Name, showValue(pc.Old))
				continue
			}
			out.WriteInfo("    %s: %s -> %s", pc.Name, showValue(pc.Old), showValue(pc.New))
		}
		if c.ContentChanged {
			out.WriteInfo("    (content)")
		}
	}
	for _, r := range p.RelationRemoves {
		out.WriteInfo("- %s --%s--> %s", r.From, r.Type, r.To)
	}
	for _, r := range p.RelationAdds {
		out.WriteInfo("+ %s --%s--> %s", r.From, r.Type, r.To)
	}
	for _, r := range p.Stale {
		out.WriteWarning("stale (kept): %s --%s--> %s", r.From, r.Type, r.To)
	}
	for _, m := range p.Missing {
		out.WriteWarning("missing from the source (kept): %s %s (key %s)", m.Type, m.ID, m.Key)
	}
	for _, e := range p.Errors {
		out.WriteError("%s: %s: %s", e.Mapping, e.Location, e.Message)
	}
	out.WriteInfo("%s create %d, update %d, add %d and remove %d relations; %d unchanged, %d skipped by the hook",
		verb, len(p.Creates), len(p.Updates), len(p.RelationAdds), len(p.RelationRemoves), p.Unchanged, p.Skipped)
}

func entityLabel(c mapimport.EntityChange) string {
	if c.ID == "" || c.ID == c.Key {
		return c.Key
	}
	return c.ID + " (" + c.Key + ")"
}

func showValue(v any) string {
	s := fmt.Sprint(v)
	if m, ok := v.(map[string]any); ok {
		if id, has := m["id"]; has {
			s = fmt.Sprint(id)
		}
	}
	const maxShown = 80
	if r := []rune(s); len(r) > maxShown {
		return string(r[:maxShown]) + "..."
	}
	return fmt.Sprintf("%q", s)
}

func printImportResult(res *mapimport.Result) {
	if res == nil {
		return
	}
	out.WriteSuccess("Created %d, updated %d, added %d and removed %d relations",
		res.Created, res.Updated, res.RelationsAdded, res.RelationsRemoved)
	for _, f := range res.Failures {
		out.WriteError("%s: %s", f.What, f.Error)
	}
}
