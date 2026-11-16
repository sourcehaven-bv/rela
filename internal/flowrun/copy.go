package flowrun

import (
	"context"
	"fmt"
)

// CopyRun recreates run r of src in dst with its definition and its whole
// state: instance, pending mark, timeline, claim and parallel branches. Its
// id, subject and create time are kept; its revision starts again.
//
// [Store.Create] takes only the starting state, so a run with branches or a
// claim is written in a second step. When that step fails, the created run
// is deleted again.
func CopyRun(ctx context.Context, src, dst Store, r Run) error {
	def, err := src.Definition(ctx, r.Flow, r.Version)
	if err != nil {
		return fmt.Errorf("flow run %s: definition: %w", r.ID, err)
	}
	created, err := dst.Create(ctx, NewRun{
		ID: r.ID, Flow: r.Flow, Version: r.Version, Definition: def,
		Subject: r.Subject, Singleton: r.Singleton, At: r.CreatedAt,
		Instance: r.Instance, Pending: r.Pending, Log: r.Log,
	})
	if err != nil {
		return fmt.Errorf("flow run %s: %w", r.ID, err)
	}
	if len(r.Branches) == 0 && r.Claim == nil {
		return nil
	}
	if _, err := dst.Update(ctx, r.ID, created.Rev, r.State, r.UpdatedAt); err != nil {
		// Never leave the run half copied: it would run from its start
		// state without its branches or claim.
		if delErr := dst.Delete(context.WithoutCancel(ctx), r.ID, created.Rev); delErr != nil {
			return fmt.Errorf("flow run %s: state: %w; removing the partial copy also failed: %w",
				r.ID, err, delErr)
		}
		return fmt.Errorf("flow run %s: state: %w", r.ID, err)
	}
	return nil
}

// ListAll returns every run in s, in id order.
func ListAll(ctx context.Context, s Store) ([]Run, error) {
	var runs []Run
	q := Query{Limit: MaxPage}
	for {
		page, err := s.List(ctx, q)
		if err != nil {
			return nil, err
		}
		runs = append(runs, page...)
		if len(page) < q.Limit {
			return runs, nil
		}
		q.After = page[len(page)-1].ID
	}
}

// CopyRuns copies every run of src into dst with [CopyRun] and reports how
// many it copied. It stops at the first failure.
func CopyRuns(ctx context.Context, src, dst Store) (int, error) {
	runs, err := ListAll(ctx, src)
	if err != nil {
		return 0, fmt.Errorf("read flow runs: %w", err)
	}
	for i, r := range runs {
		if err := CopyRun(ctx, src, dst, r); err != nil {
			return i, err
		}
	}
	return len(runs), nil
}
