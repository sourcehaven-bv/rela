package reconclient

import (
	"context"
	"errors"

	"github.com/Sourcehaven-BV/rela/internal/store"
	"github.com/Sourcehaven-BV/rela/internal/visibility"
)

// Visible returns x as the [Local] side of a run.
//
// Nil: rejected; there is no local side without it.
func Visible(x *visibility.Elements) (Local, error) {
	if x == nil {
		return nil, errors.New("reconclient: local element reader is required")
	}
	return visibleLocal{x: x}, nil
}

type visibleLocal struct{ x *visibility.Elements }

func (v visibleLocal) Sum(ctx context.Context, types visibility.ElementTypes, r store.ElementRange) (Sum, error) {
	rg, err := v.x.Fingerprint(ctx, types, r)
	if err != nil {
		return Sum{}, err
	}
	return Sum{Count: rg.Count, Fingerprint: rg.Sum.String()}, nil
}

func (v visibleLocal) List(
	ctx context.Context, types visibility.ElementTypes, r store.ElementRange, limit int,
) (Page, error) {
	page, err := v.x.List(ctx, types, r, limit)
	if err != nil {
		return Page{}, err
	}
	rows := make([]Row, len(page.Elements))
	for i, e := range page.Elements {
		rows[i] = Row{Address: e.Address, Hash: e.Hash}
	}
	return Page{Rows: rows, Last: page.Last, More: page.More}, nil
}
