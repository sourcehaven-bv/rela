package dataentry

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/acl"
	"github.com/Sourcehaven-BV/rela/internal/principal"
)

// TestServeConfigure_Gate pins who reaches the Configure API: nobody when it
// is not mounted, and with it mounted only a known principal holding
// config:edit under a loaded acl.yaml.
func TestServeConfigure_Gate(t *testing.T) {
	known := principal.VerifiedFrom("usr_1", principal.ToolDataEntry, principal.Claims{})
	cases := []struct {
		name      string
		mounted   bool
		declared  bool // acl.yaml loaded (Declarative) rather than NopACL
		principal principal.Principal
		holds     bool
		want      int
		canConfig bool
	}{
		{name: "not mounted", declared: true, principal: known, holds: true, want: http.StatusNotFound},
		{name: "no acl.yaml", mounted: true, principal: known, holds: true, want: http.StatusForbidden},
		{name: "no principal", mounted: true, declared: true, holds: true, want: http.StatusForbidden},
		{name: "unknown principal", mounted: true, declared: true, holds: true, want: http.StatusForbidden,
			principal: principal.Principal{User: principal.Unknown, Tool: principal.ToolDataEntry}},
		{name: "lacks permission", mounted: true, declared: true, principal: known, want: http.StatusForbidden},
		{name: "client token", mounted: true, declared: true, holds: true, want: http.StatusForbidden,
			principal: principal.VerifiedFrom("usr_1", principal.ToolDataEntry, principal.Claims{PrincipalType: "pat"})},
		{name: "interactive token", mounted: true, declared: true, holds: true, want: http.StatusOK, canConfig: true,
			principal: principal.VerifiedFrom("usr_1", principal.ToolDataEntry, principal.Claims{PrincipalType: "user"})},
		{name: "allowed", mounted: true, declared: true, principal: known, holds: true,
			want: http.StatusOK, canConfig: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			app := newTestAppV1(t)
			if tc.declared {
				d, err := acl.NewDeclarative(&acl.Policy{}, acl.NewStoreGraph(app.store), app.store)
				require.NoError(t, err)
				app.acl = d
			}
			reached := false
			if tc.mounted {
				SetConfigure(app, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
					reached = true
					w.WriteHeader(http.StatusOK)
				}))
			}
			ctx := withReadGate(context.Background(), fakeGate{holdsPermission: tc.holds})
			if !tc.principal.IsZero() {
				ctx = principal.With(ctx, tc.principal)
			}

			req := httptest.NewRequest(http.MethodGet, ConfigurePath, http.NoBody).WithContext(ctx)
			rec := httptest.NewRecorder()
			serveConfigure(app)(rec, req)
			require.Equal(t, tc.want, rec.Code, rec.Body.String())
			require.Equal(t, tc.want == http.StatusOK, reached)

			require.Equal(t, tc.canConfig, decodeMe(t, meAs(ctx, t, app)).CanConfigure)
		})
	}
}

// TestEventBroker_Close pins what Retire relies on: close ends every open
// stream after it drains what was buffered, and a later subscriber gets a
// stream that is already closed.
func TestEventBroker_Close(t *testing.T) {
	b := newEventBroker()
	ch := b.subscribe()
	b.broadcast(sseConfigChanged)
	b.close()

	ev, ok := <-ch
	require.True(t, ok, "the buffered event is still delivered")
	require.Equal(t, sseConfigChanged, ev.Name)
	_, ok = <-ch
	require.False(t, ok, "the stream ends after the buffer drains")

	_, ok = <-b.subscribe()
	require.False(t, ok, "subscribing after close returns a closed channel")

	b.unsubscribe(ch) // must not panic after close
	b.broadcast("refresh")
}
