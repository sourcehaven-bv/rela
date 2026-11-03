//go:build postgres

package appbuild_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// BenchmarkChannelPost measures a post on postgres with channel hints sent
// over NOTIFY, and with no hint endpoint, whose sends fail at once.
func BenchmarkChannelPost(b *testing.B) {
	for _, tc := range []struct {
		name  string
		hints bool
	}{{"hints", true}, {"no-hints", false}} {
		b.Run(tc.name, func(b *testing.B) {
			root := b.TempDir()
			for name, body := range map[string]string{
				"schema.yaml": promoteMetamodel, "acl.yaml": benchPolicy(), "realtime.yaml": promoteRealtime,
			} {
				if err := os.WriteFile(filepath.Join(root, name), []byte(body), 0o600); err != nil {
					b.Fatal(err)
				}
			}
			svc, err := appbuild.Discover(root, script.NewEngine(), backendtest.Options(b)...)
			if err != nil {
				b.Fatal(err)
			}
			b.Cleanup(func() { _ = svc.Close() })
			if !tc.hints {
				appbuild.DetachChannelHintsForTest(svc)
			}
			// Posts rotate over benchPosters users, so the post rate limit
			// per user is not reached.
			ctxs := make([]context.Context, benchPosters)
			for i := range ctxs {
				p := principal.Principal{User: fmt.Sprintf("u%d", i), Tool: principal.ToolDataEntry}
				ctxs[i] = boundAs(b, svc, p) //nolint:fatcontext // one bound ctx per poster, not nested
			}
			ctx := ctxs[0]
			e := entity.New("", "doc")
			e.SetString("title", "One")
			if _, err := svc.EntityManager().CreateEntity(ctx, e, entity.CreateOptions{}); err != nil {
				b.Fatal(err)
			}
			cs := appbuild.ChannelService(svc)
			in := realtime.PostInput{Type: "proposal", Props: map[string]any{"summary": "bench"}}
			n := 0
			for b.Loop() {
				n++
				if _, err := cs.Post(ctxs[n%benchPosters], promoteRoom, in); err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// benchPosters is how many users the benchmark posts as.
const benchPosters = 100

// benchPolicy grants admin to u0 .. u(benchPosters-1).
func benchPolicy() string {
	var sb strings.Builder
	sb.WriteString("roles:\n  admin:\n    read: [\"*\"]\n    create: [\"*\"]\nassignments:\n")
	for i := range benchPosters {
		fmt.Fprintf(&sb, "  u%d: admin\n", i)
	}
	return sb.String()
}
