package records

import (
	"context"
	"fmt"
)

// Reader is the part of a [Store] that [Each] and [Copy] read from.
type Reader interface {
	Owners(ctx context.Context, typ string) ([]string, error)
	List(ctx context.Context, q ListQuery) (Page, error)
}

// Appender is the part of a [Store] that [Copy] writes to.
type Appender interface {
	Append(ctx context.Context, r AppendRequest) error
}

// Each calls fn for every row of every type in types, read raw from src:
// type by type in the given order, then owner by owner, then in (TS, ID)
// order. It reads no access rules, so it is for an operator export or copy,
// never for a request.
//
// Only the listed types are read. The Store contract cannot list the types
// it holds, so rows of a type the caller does not name are not visited.
func Each(ctx context.Context, src Reader, types []string, fn func(Record) error) error {
	for _, typ := range types {
		owners, err := src.Owners(ctx, typ)
		if err != nil {
			return fmt.Errorf("records: owners of %s: %w", typ, err)
		}
		for _, owner := range owners {
			q := ListQuery{Type: typ, Owner: owner, Limit: MaxPage}
			for {
				page, err := src.List(ctx, q)
				if err != nil {
					return fmt.Errorf("records: list %s of %s: %w", typ, owner, err)
				}
				for _, rec := range page.Records {
					if err := fn(rec); err != nil {
						return err
					}
				}
				if page.Next == "" {
					break
				}
				q.After = page.Next
			}
		}
	}
	return nil
}

// CopyOptions tunes [Copy].
type CopyOptions struct {
	// ClaimGroups claims every ledger group in dst as its first row is
	// written. A backend that keeps its claims apart from the rows (the
	// SQL backends) needs this, or a copied transaction id could be used
	// again. The file backend derives its claims from the rows, and
	// claiming there costs a scan of the type per group.
	ClaimGroups bool
}

// Copy appends every row of the listed types from src to dst unchanged, in
// batches of [MaxBatch], and returns how many rows it wrote. Ids, times and
// attribution are kept. It reads raw, as [Each] does.
//
// A batch that fails stops the copy; earlier batches stay written unless
// dst is bound to a transaction the caller rolls back.
func Copy(ctx context.Context, src Reader, dst Appender, types []string, opts CopyOptions) (int, error) {
	claimed := map[GroupKey]bool{}
	var batch AppendRequest
	n := 0
	flush := func() error {
		if len(batch.Records) == 0 {
			return nil
		}
		if err := dst.Append(ctx, batch); err != nil {
			return fmt.Errorf("records: append: %w", err)
		}
		n += len(batch.Records)
		batch = AppendRequest{}
		return nil
	}
	err := Each(ctx, src, types, func(rec Record) error {
		if opts.ClaimGroups && rec.Group != "" {
			key := GroupKey{Type: rec.Type, Group: rec.Group}
			if !claimed[key] {
				claimed[key] = true
				batch.Groups = append(batch.Groups, key)
			}
		}
		batch.Records = append(batch.Records, rec)
		if len(batch.Records) >= MaxBatch {
			return flush()
		}
		return nil
	})
	if err != nil {
		return n, err
	}
	return n, flush()
}
