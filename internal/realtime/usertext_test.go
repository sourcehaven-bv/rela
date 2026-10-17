package realtime

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"maps"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/realtimecfg"
)

func TestUserText(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		err  error
		want string
		ok   bool
	}{
		{"denial", unknownChannelType("room"), `unknown_channel_type: no channel type "room" here`, true},
		{
			"rule", forbidden(TypeReadDenied, "reading needs a role", "channel_types.room.read"),
			"channel_read_denied: reading needs a role (rule: channel_types.room.read)", true,
		},
		{"retry", rateLimited(1500 * time.Millisecond), "rate_limited: too many requests; try again later; retry in 2s", true},
		{"wrapped", fmt.Errorf("x: %w", channelNotFound()), "channel_not_found: channel not found", true},
		{"no principal", noPrincipal(), "no_principal: the request carries no identity the ACL can evaluate" +
			noPrincipalHint, true},
		{"off", ErrRealtimeOff, OffText, true},
		{"anchor", ErrAnchorNotFound, "", false},
		{"other", errors.New("disk on fire"), "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, ok := UserText(tc.err)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

// denialConstructors builds one denial from every function of the package
// that returns a *DenialError.
func denialConstructors() map[string]*DenialError {
	ct := realtimecfg.ChannelType{Name: "room"}
	mt := &realtimecfg.MessageType{Name: "text", Post: []string{"member"}}
	return map[string]*DenialError{
		"deny":               deny(400, TypeInvalid, "bad"),
		"forbidden":          forbidden(TypePostDenied, "no", ""),
		"rateLimited":        rateLimited(0),
		"unknownChannelType": unknownChannelType("x"),
		"channelNotFound":    channelNotFound(),
		"messageNotFound":    messageNotFound(),
		"invalid":            invalid("bad %s", "value"),
		"invalidRef":         invalidRef(),
		"typeConflict":       typeConflict("room"),
		"signalsNotKept":     signalsNotKept("typing"),
		"configChanged":      configChanged(),
		"noPrincipal":        noPrincipal(),
		"readDenied":         readDenied(ct),
		"postDenied":         postDenied(ct, mt),
		"gateDenied":         gateDenied(TypeCreateDenied, "creating", "r", realtimecfg.Gate{}, nil),
	}
}

// TestDenialConstructors_HaveTitles pins that every denial the package
// builds renders a type and a title, and that the table above lists every
// such function.
func TestDenialConstructors_HaveTitles(t *testing.T) {
	t.Parallel()
	table := denialConstructors()
	for name, d := range table {
		require.NotEmpty(t, d.Type, name)
		require.NotEmpty(t, d.Title, name)
		text, ok := UserText(d)
		require.True(t, ok, name)
		require.True(t, strings.HasPrefix(text, d.Type+": "), name)
	}

	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	var found []string
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, e.Name(), nil, 0)
		require.NoError(t, err)
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
				continue
			}
			star, ok := fn.Type.Results.List[0].Type.(*ast.StarExpr)
			if !ok {
				continue
			}
			if id, isIdent := star.X.(*ast.Ident); isIdent && id.Name == "DenialError" {
				found = append(found, fn.Name.Name)
			}
		}
	}
	slices.Sort(found)
	require.Equal(t, found, slices.Sorted(maps.Keys(table)))
}
