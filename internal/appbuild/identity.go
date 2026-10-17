package appbuild

import (
	"context"
	"errors"
	"log/slog"

	"github.com/Sourcehaven-BV/rela/internal/acl"
)

// IdentityBinder returns the request-edge step for svc. Every transport whose
// principal is already a graph identity takes this one shape at wiring: the
// CLI, rela-docs, the scheduler and stdio MCP.
//
// The returned function resolves the principal stamped on ctx and attaches
// its acl.Request (see [acl.BindRequest]). It binds against
// [Services.ACLDeclarative], read once here, so the Request belongs to the
// same evaluator as svc's gated readers and manager. With no declarative
// policy it returns ctx unchanged.
//
// Nil: rejected with an error; there are no Services to bind against.
func IdentityBinder(svc *Services) (func(context.Context) (context.Context, error), error) {
	if svc == nil {
		return nil, errors.New("appbuild: IdentityBinder: services are nil")
	}
	return identityBinderFor(svc.ACLDeclarative()), nil
}

// MustIdentityBinder is [IdentityBinder] for tests: it panics on nil.
func MustIdentityBinder(svc *Services) func(context.Context) (context.Context, error) {
	bind, err := IdentityBinder(svc)
	if err != nil {
		panic(err)
	}
	return bind
}

// identityBinderFor is [IdentityBinder] over a policy already in hand. It is
// the one place in appbuild that resolves an identity, so the background job
// runner and the MCP watch rebind share the edge step with every transport.
// Calling it again on a ctx that already carries a Request replaces that
// Request with a fresh one, which is what a long-lived reader needs to see
// role changes.
//
// Nil: accepted — a nil d means no policy is configured; the binder then
// returns ctx unchanged.
func identityBinderFor(d *acl.Declarative) func(context.Context) (context.Context, error) {
	return func(ctx context.Context) (context.Context, error) {
		return acl.BindRequest(ctx, d)
	}
}

// BindOrWarn runs bind on ctx for a command-line entry point. A failure (for
// example an unstamped OS user under an enforcing acl.yaml) is logged and ctx
// is returned unchanged: commands that touch no gated data still run, and
// every gated read or write denies on its own.
func BindOrWarn(ctx context.Context, bind func(context.Context) (context.Context, error)) context.Context {
	bound, err := bind(ctx)
	if err != nil {
		slog.Warn("acl: could not resolve the principal; gated reads and writes will be denied", "err", err)
		return ctx
	}
	return bound
}
