package apptest

import (
	"context"
	"errors"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/ai"
)

// errRealNetwork is what a case's AI calls fail with. A case runs offline;
// its HTTP calls go to its stubs ([stubTransport]).
var errRealNetwork = errors.New("real AI calls are not allowed in rela test")

// refusingAI fails every call with a network-kind [ai.Error], the only kind
// the Lua binding reports to the script, and marks the case: a script that
// handles the error and carries on must not let the case pass.
type refusingAI struct{ tried *atomic.Bool }

// Chat refuses the call.
func (a refusingAI) Chat(context.Context, ai.ChatRequest) (*ai.ChatResponse, error) {
	a.tried.Store(true)
	return nil, &ai.Error{Kind: ai.ErrNetwork, Message: errRealNetwork.Error()}
}

// Embed refuses the call.
func (a refusingAI) Embed(context.Context, ai.EmbedRequest) (*ai.EmbedResponse, error) {
	a.tried.Store(true)
	return nil, &ai.Error{Kind: ai.ErrNetwork, Message: errRealNetwork.Error()}
}
