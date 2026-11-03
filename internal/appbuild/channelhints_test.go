//go:build !memorybackend && !sqlite

package appbuild_test

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/appbuild"
	"github.com/Sourcehaven-BV/rela/internal/appbuild/backendtest"
	"github.com/Sourcehaven-BV/rela/internal/channels"
	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/script"
)

// hintPolicy lets alice do everything, so she can post, rename and delete.
const hintPolicy = `roles:
  admin:
    read: ["*"]
    create: ["*"]
    update: ["*"]
    delete: ["*"]
assignments:
  alice: admin
`

// twoProcesses assembles the promote project twice over one backend, as
// two processes would open it: one project directory, and on postgres one
// schema. Each follows the other's hints, and has a live subscriber that
// has taken the follower's start resync.
func twoProcesses(t *testing.T) (a, b hintProcess) {
	t.Helper()
	root := t.TempDir()
	// Rename needs id_type manual.
	writeMetamodelBody(t, root, strings.Replace(promoteMetamodel, `id_prefix: "DOC-"
    id_type: sequential`, `id_prefix: "DOC-"
    id_type: manual`, 1))
	writePolicy(t, root, hintPolicy)
	require.NoError(t, os.WriteFile(filepath.Join(root, "realtime.yaml"), []byte(promoteRealtime), 0o600))
	opts := backendtest.Options(t)
	open := func() hintProcess {
		svc, err := appbuild.Discover(root, script.NewEngine(), opts...)
		require.NoError(t, err)
		// Subscribed before the follower starts, so the start resync
		// reaches it and is taken here, not mistaken for a hint later.
		acc := &liveAccumulator{sub: appbuild.ChannelService(svc).SubscribeLive()}
		ctx, cancel := context.WithCancel(context.Background())
		done := appbuild.FollowChannels(ctx, svc)
		t.Cleanup(func() {
			cancel()
			<-done
			acc.sub.Close()
			_ = svc.Close()
		})
		require.Eventually(t, func() bool { return acc.sub.TakeLive().Resync }, 15*time.Second, 10*time.Millisecond)
		return hintProcess{svc: svc, acc: acc}
	}
	// a creates the anchor before b opens, so b loads it from the backend.
	a = open()
	e := entity.New("", "doc")
	e.SetString("title", "One")
	_, err := a.svc.EntityManager().CreateEntity(promoteAs(t, a.svc, "alice"), e,
		entity.CreateOptions{ID: promoteRoom.Anchor})
	require.NoError(t, err)
	b = open()
	a.acc.sub.TakeLive() // a's own create
	return a, b
}

// hintProcess is one of twoProcesses.
type hintProcess struct {
	svc *appbuild.Services
	acc *liveAccumulator
}

// liveAccumulator gathers what a subscription takes until a condition
// holds.
type liveAccumulator struct {
	sub *realtime.Subscription
	got realtime.LiveChanges
}

func (l *liveAccumulator) take() {
	c := l.sub.TakeLive()
	l.got.Keys = append(l.got.Keys, c.Keys...)
	l.got.Durable = append(l.got.Durable, c.Durable...)
	l.got.Moved = append(l.got.Moved, c.Moved...)
	l.got.Addressed = append(l.got.Addressed, c.Addressed...)
	for r, ks := range c.Reads {
		if l.got.Reads == nil {
			l.got.Reads = map[channels.Reader][]channels.Key{}
		}
		l.got.Reads[r] = append(l.got.Reads[r], ks...)
	}
}

// waitFor takes until ok holds on what was gathered, then clears it.
func (l *liveAccumulator) waitFor(t *testing.T, what string, ok func(realtime.LiveChanges) bool) {
	t.Helper()
	require.Eventually(t, func() bool {
		l.take()
		return ok(l.got)
	}, 15*time.Second, 20*time.Millisecond, "%s never reached the other process; got %+v", what, l.got)
	l.got = realtime.LiveChanges{}
}

func anchoredAt(id string) func(channels.Key) bool {
	return func(k channels.Key) bool { return k.ID == id && k.Type == promoteRoom.Type }
}

// A post, a read marker, a rename and a delete in one process reach the
// live subscribers of another with the right kinds (AC 1 to 4).
func TestChannelHints_ReachOtherProcess(t *testing.T) {
	p, q := twoProcesses(t)
	a, acc := p.svc, q.acc
	alice := promoteAs(t, a, "alice")

	res, err := appbuild.ChannelService(a).Post(alice, promoteRoom, realtime.PostInput{
		Type: "proposal", Props: map[string]any{"summary": "hi"},
	})
	require.NoError(t, err)
	acc.waitFor(t, "a post", func(c realtime.LiveChanges) bool {
		return slices.ContainsFunc(c.Durable, anchoredAt("DOC-001"))
	})

	_, err = appbuild.ChannelService(a).MarkRead(alice, promoteRoom, &res.Message.Seq)
	require.NoError(t, err)
	acc.waitFor(t, "a read marker", func(c realtime.LiveChanges) bool {
		for r, ks := range c.Reads {
			if r.User == "alice" && slices.ContainsFunc(ks, anchoredAt("DOC-001")) {
				return true
			}
		}
		return false
	})

	_, err = a.EntityManager().RenameEntity(alice, "DOC-001", "DOC-100", entity.RenameOptions{})
	require.NoError(t, err)
	acc.waitFor(t, "a rename", func(c realtime.LiveChanges) bool {
		return slices.ContainsFunc(c.Moved, anchoredAt("DOC-001")) && slices.ContainsFunc(c.Moved, anchoredAt("DOC-100"))
	})

	_, err = a.EntityManager().DeleteEntity(alice, "DOC-100", false)
	require.NoError(t, err)
	acc.waitFor(t, "a delete", func(c realtime.LiveChanges) bool {
		return slices.ContainsFunc(c.Moved, anchoredAt("DOC-100"))
	})
}

// A re-assembly keeps the follower and the shared Local: hints flow both
// ways after the swap, and the retired assembly's endpoint is gone.
func TestChannelHints_SurviveReassembly(t *testing.T) {
	p, q := twoProcesses(t)
	a, b := p.svc, q.svc
	successor, err := a.Base().ForReassembly(a).Assemble(a.Store(), a.Searcher(), a.VisibleSearcher(), nil)
	require.NoError(t, err)
	defer successor.CloseAssembly()
	// The follower a's host started keeps running: a second one returns at
	// once.
	select {
	case <-appbuild.FollowChannels(context.Background(), successor):
	case <-time.After(5 * time.Second):
		t.Fatal("a second follower on the shared Local did not return at once")
	}
	a.CloseAssembly()

	// The successor shares a's Local, and so a's subscriber.
	fromB := p.acc
	_, err = appbuild.ChannelService(b).Post(promoteAs(t, b, "alice"), promoteRoom, realtime.PostInput{
		Type: "proposal", Props: map[string]any{"summary": "from b"},
	})
	require.NoError(t, err)
	fromB.waitFor(t, "b's post after the swap", func(c realtime.LiveChanges) bool {
		return slices.ContainsFunc(c.Durable, anchoredAt("DOC-001"))
	})

	toB := q.acc
	_, err = appbuild.ChannelService(successor).Post(promoteAs(t, successor, "alice"), promoteRoom,
		realtime.PostInput{Type: "proposal", Props: map[string]any{"summary": "from successor"}})
	require.NoError(t, err)
	toB.waitFor(t, "the successor's post", func(c realtime.LiveChanges) bool {
		return slices.ContainsFunc(c.Durable, anchoredAt("DOC-001"))
	})
}
