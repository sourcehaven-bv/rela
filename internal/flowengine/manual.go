package flowengine

import (
	"context"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/flowcfg"
)

// ManualStarts returns the `on: manual` starts of f whose `when:` holds
// for the subject at ref, read raw like every other guard. A start
// without `when:` always holds; one whose `when:` fails to evaluate does
// not, so a broken condition offers no start. A function rather than a
// method: Engine is at its method cap.
func ManualStarts(ctx context.Context, e *Engine, f *flowcfg.Flow, ref entity.Ref) ([]flowcfg.Start, error) {
	var (
		out  []flowcfg.Start
		subj *entity.Entity
	)
	for _, s := range f.Starts() {
		if s.On != flowcfg.StartManual {
			continue
		}
		if s.When == nil {
			out = append(out, s)
			continue
		}
		if subj == nil {
			var err error
			if subj, err = e.d.Subjects.Subject(ctx, ref); err != nil {
				return nil, err
			}
		}
		ok, err := e.g.holds(ctx, s.When, subj)
		if err != nil {
			// The error can quote a property value of the subject, so it
			// stays at debug level.
			slog.WarnContext(ctx, "flowengine: a manual start condition failed", "flow", f.Name(),
				"subject", subj.ID, "when", s.When.Source())
			slog.DebugContext(ctx, "flowengine: manual start condition error", "flow", f.Name(),
				"subject", subj.ID, "error", err)
			continue
		}
		if ok {
			out = append(out, s)
		}
	}
	return out, nil
}
