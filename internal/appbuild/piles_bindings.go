package appbuild

import (
	"context"

	"github.com/Sourcehaven-BV/rela/internal/autocascade"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/lua"
	"github.com/Sourcehaven-BV/rela/internal/piles"
)

// The adapters below hand the piles service to the surfaces that declare
// their own consumer-side piles seams (lua, autocascade; mcp is adapted in
// internal/mcpwire, because mcp tests import this package). Each one only
// maps types: the owner, privacy and push rules all stay in [piles.Service].

// luaPiles adapts svc to the Lua rela.piles capability.
func luaPiles(svc *piles.Service) *lua.PileFuncs {
	summary := func(p piles.Pile) lua.PileSummary {
		return lua.PileSummary{ID: p.ID, Name: p.Name, Icon: p.Icon, Items: p.Refs()}
	}
	return &lua.PileFuncs{
		List: func(ctx context.Context) ([]lua.PileSummary, error) {
			ps, err := svc.List(ctx)
			if err != nil {
				return nil, err
			}
			out := make([]lua.PileSummary, len(ps))
			for i, p := range ps {
				out[i] = summary(p)
			}
			return out, nil
		},
		Find: func(ctx context.Context, name string) (lua.PileSummary, error) {
			p, err := svc.ByName(ctx, name)
			if err != nil {
				return lua.PileSummary{}, err
			}
			return summary(p), nil
		},
		Push: func(ctx context.Context, p lua.PilePush) (int, error) {
			return svc.Push(ctx, piles.PushRequest{Owner: p.Owner, Pile: p.Pile, Refs: p.Refs, Create: p.Create})
		},
		Remove: svc.Remove,
	}
}

// pileReaderFor and pileWriterFor return the Lua piles capability, or a
// genuinely nil interface when svc is nil: a nil *lua.PileFuncs in the
// interface field would pass the binding's nil check and then panic.
func pileReaderFor(svc *piles.Service) lua.PileReader {
	if svc == nil {
		return nil
	}
	return luaPiles(svc)
}

func pileWriterFor(svc *piles.Service) lua.PileWriter {
	if svc == nil {
		return nil
	}
	return luaPiles(svc)
}

// cascadePiles adapts svc to the automation add_to_pile action.
func cascadePiles(svc *piles.Service) autocascade.PilePusher {
	return autocascade.PilePushFunc(func(ctx context.Context, r autocascade.PileRequest) error {
		_, err := svc.Push(ctx, piles.PushRequest{
			Owner: r.Owner, Pile: r.Pile, Refs: []entity.Ref{r.Ref}, Create: r.Create,
		})
		return err
	})
}

// LuaPiles is the Lua rela.piles capability over the piles service of s, for
// surfaces that build their own [lua.WriteDeps] (the data-entry App).
func LuaPiles(s *Services) *lua.PileFuncs { return luaPiles(s.piles) }
