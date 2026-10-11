// Package mcpwire wires MCP tool groups to services from appbuild. It sits
// outside both: appbuild cannot import mcp (mcp's tests import appbuild), and
// mcp declares consumer-side seams rather than importing services.
package mcpwire

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/mcp"
	"github.com/Sourcehaven-BV/rela/internal/piles"
)

// Piles registers the pile tools over the piles service of svc. Every owner,
// privacy and push rule stays in [piles.Service]; this only maps types.
func Piles(svc *appbuild.Services) mcp.Option {
	f := pileFuncs(svc.Piles())
	return mcp.WithPiles(f, f)
}

func pileFuncs(svc *piles.Service) *mcp.PileFuncs {
	info := func(p piles.Pile) mcp.PileInfo {
		return mcp.PileInfo{ID: p.ID, Name: p.Name, Icon: p.Icon, Items: p.Refs()}
	}
	return &mcp.PileFuncs{
		List: func(ctx context.Context) ([]mcp.PileInfo, error) {
			ps, err := svc.List(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]mcp.PileInfo, len(ps))
			for i, p := range ps {
				out[i] = info(p)
			}
			return out, nil
		},
		Find: func(ctx context.Context, nameOrID string) (mcp.PileInfo, error) {
			get := svc.ByName
			if piles.ValidID(nameOrID) {
				get = svc.Get
			}
			p, err := get(ctx, nameOrID)
			if err != nil {
				return mcp.PileInfo{}, err
			}
			return info(p), nil
		},
		Push: func(ctx context.Context, p mcp.PilePush) (int, error) {
			return svc.Push(ctx, piles.PushRequest{Owner: p.Owner, Pile: p.Pile, Refs: p.Refs, Create: p.Create})
		},
		Remove: svc.Remove,
	}
}
