package apptest

import (
	"context"
	"errors"
	"net/http"
	"sync/atomic"

	"github.com/Sourcehaven-BV/rela/internal/ai"
)

// errRealNetwork is what a case's HTTP and AI calls fail with. A case runs
// offline; stubbing those calls is planned in TKT-HB313W5.
var errRealNetwork = errors.New("real HTTP/AI calls are not allowed in rela test")

// refusingTransport fails every request before any connection is made, and
// marks the case: a script that handles the error and carries on must not
// let the case pass.
type refusingTransport struct{ tried *atomic.Bool }

// RoundTrip refuses the request.
func (t refusingTransport) RoundTrip(*http.Request) (*http.Response, error) {
	t.tried.Store(true)
	return nil, errRealNetwork
}

// refusingAI fails every call with a network-kind [ai.Error], the only kind
// the Lua binding reports to the script, and marks the case as
// [refusingTransport] does.
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
