package cli

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/alecthomas/kong"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Sourcehaven-BV/rela/internal/entity"
	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
)

// channelActionRealtime adds a proposals channel to the room fixture: a
// member may promote a proposal to an item, and only a lead (system:ci,
// through chat.lead) may close it.
const channelActionRealtime = channelCLIRealtime + `  proposals:
    attach_to: [item]
    roles:
      member: { from: attached }
      lead:   { permission: chat.lead }
    message_types:
      proposal:
        properties: { summary: { type: string, required: true } }
        post: [member]
        actions:
          accept: { roles: [member], promote_to: { type: item, map: { title: summary } } }
          close:  { roles: [lead] }
`

// TestChannelCLI_Action pins TKT-6912G8: rela channel action runs a
// message action through the service, with the caller's identity, and
// answers refusals with the service's text.
func TestChannelCLI_Action(t *testing.T) {
	p := newChannelProjectWith(t, channelActionRealtime)
	for range 2 {
		_, err := runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "proposals", "proposal",
			"-P", "summary=Ship it")
		require.NoError(t, err)
	}

	got, err := runChannel(t, p.svc, "alice", "", "channel", "action", p.item, "proposals", "1", "accept")
	require.NoError(t, err)
	assert.Regexp(t, `^#1 action accept: done -> ITEM-\d+\n$`, got)

	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "action", p.item, "proposals", "2", "accept")
	require.NoError(t, err)
	var ran actionJSON
	channelJSONOf(t, got, &ran)
	assert.Equal(t, int64(2), ran.Message.Seq)
	require.NotNil(t, ran.Message.Action)
	assert.Equal(t, "accept", ran.Message.Action.Name)
	require.NotNil(t, ran.Message.Action.Promoted)

	for range 2 {
		_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "proposals", "proposal", "-P", "summary=No")
		require.NoError(t, err)
	}
	got, err = runChannel(t, p.svc, "alice", "", "channel", "read", p.item, "proposals")
	require.NoError(t, err)
	assert.Contains(t, got, "  may: accept", "read lists the actions the reader may run")
	got, err = runChannel(t, p.svc, "system:ci", "", "channel", "action", p.item, "proposals", "3", "close")
	require.NoError(t, err, "the lead role may close")
	assert.Equal(t, "#3 action close: done\n", got)

	for _, tc := range []struct {
		name   string
		args   []string
		denial string // the service's refusal type
		usage  string // a refusal before the service is called
	}{
		{name: "already run", args: []string{"1", "accept"}, denial: realtime.TypeActionTaken},
		{name: "undeclared action", args: []string{"1", "nope"}, denial: realtime.TypeActionNotOffered},
		{name: "role without the action", args: []string{"4", "close"}, denial: realtime.TypeActionDenied},
		{name: "missing message", args: []string{"9", "accept"}, denial: realtime.TypeMessageNotFound},
		{name: "release without a pending action", args: []string{"1", "--release"},
			denial: realtime.TypeActionNotPending},
		{name: "seq 0", args: []string{"0", "accept"}, usage: "seq: at least 1"},
		{name: "name and release", args: []string{"1", "accept", "--release"},
			usage: "give an action name or --release, not both"},
		{name: "neither", args: []string{"1"}, usage: "give an action name, or --release"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := append([]string{"channel", "action", p.item, "proposals"}, tc.args...)
			_, err := runChannel(t, p.svc, "alice", "", args...)
			require.Error(t, err)
			if tc.usage != "" {
				assert.Equal(t, tc.usage, err.Error())
				return
			}
			var text *channelTextError
			require.ErrorAs(t, err, &text, "a refusal answers with the service's user text")
			var denial *realtime.DenialError
			require.ErrorAs(t, err, &denial)
			assert.Equal(t, tc.denial, denial.Type)
		})
	}
}

// TestChannelCLI_Session pins TKT-6912G8: rela channel session shows the
// session, who it waits for and what the reader may post to move it.
func TestChannelCLI_Session(t *testing.T) {
	p := newChannelProject(t)
	got, err := runChannel(t, p.svc, "alice", "", "channel", "session", p.item, "incident")
	require.NoError(t, err)
	assert.Equal(t, p.item+" incident (open)\nsession: detected (Detected), generation 0, working\n"+
		"may post: contained\n", got)

	got, err = runChannel(t, p.svc, "alice", "", "-o", "json", "channel", "session", p.item, "incident")
	require.NoError(t, err)
	var read realtime.SessionRead
	channelJSONOf(t, got, &read)
	require.NotNil(t, read.Session)
	assert.Equal(t, "detected", read.Session.State)
	assert.Equal(t, []string{"contained"}, read.Transitions)

	_, err = runChannel(t, p.svc, "alice", "", "channel", "post", p.item, "incident", "contained")
	require.NoError(t, err)
	got, err = runChannel(t, p.svc, "alice", "", "channel", "session", p.item, "incident")
	require.NoError(t, err)
	assert.Contains(t, got, "session: contained (Contained), generation 1, final, completed\n")
	assert.Contains(t, got, "may post: nothing that moves the session\n")

	_, err = runChannel(t, p.svc, "alice", "", "channel", "session", p.item, "room")
	var text *channelTextError
	require.ErrorAs(t, err, &text, "a type without a session")
	var denial *realtime.DenialError
	require.ErrorAs(t, err, &denial)
	assert.Equal(t, realtime.TypeNoSession, denial.Type)
}

// fakeActor answers RunAction and ReleaseAction with fixed results, for
// output the real service only gives in states a test cannot reach
// cheaply: a release past [realtimelimits.PendingRelease], and warnings.
type fakeActor struct {
	channelService
	result   realtime.ActionResult
	released realtime.MessageView
	calls    []string
}

func (f *fakeActor) Enabled() bool { return true }

func (f *fakeActor) RunAction(_ context.Context, _ realtime.RawAddress, seq int64, name string) (realtime.ActionResult, error) {
	f.calls = append(f.calls, fmt.Sprintf("run %d %s", seq, name))
	return f.result, nil
}

func (f *fakeActor) ReleaseAction(_ context.Context, _ realtime.RawAddress, seq int64) (realtime.MessageView, error) {
	f.calls = append(f.calls, fmt.Sprintf("release %d", seq))
	return f.released, nil
}

// TestChannelCLI_ActionOutput pins the routing of rela channel action and
// the output the real service gives only in states hard to reach: a
// release, and a promote with soft validation warnings.
func TestChannelCLI_ActionOutput(t *testing.T) {
	fake := &fakeActor{
		result: realtime.ActionResult{
			Message: realtime.MessageView{Seq: 4, Action: &realtime.ActionView{
				Name: "accept", Status: "done", Promoted: &realtime.RefValue{ID: "ITEM-7"},
			}},
			Warnings: []entity.Warning{
				{Code: "missing", Path: "owner", Detail: "owner is \x1b[31mrequired"},
				{Code: "bare"},
			},
		},
		released: realtime.MessageView{Seq: 4},
	}
	run := func(args ...string) string {
		t.Helper()
		got, err := runChannelOn(t, &channelServices{Svc: fake}, append([]string{"channel", "action", "ITEM-1", "room"}, args...)...)
		require.NoError(t, err)
		return got
	}

	got := run("4", "accept")
	assert.Equal(t, "#4 action accept: done -> ITEM-7\nwarning: missing owner: owner is \uFFFD[31mrequired\nwarning: bare\n", got)

	var ran actionJSON
	channelJSONOf(t, run("-o", "json", "4", "accept"), &ran)
	assert.Equal(t, fake.result.Warnings, ran.Warnings)

	assert.Equal(t, "#4 released\n", run("4", "--release"))
	var released actionJSON
	channelJSONOf(t, run("-o", "json", "4", "--release"), &released)
	assert.Equal(t, int64(4), released.Message.Seq)
	assert.Nil(t, released.Message.Action)

	assert.Equal(t, []string{"run 4 accept", "run 4 accept", "release 4", "release 4"}, fake.calls)
}

// runChannelOn runs a channel command as alice against cs, for a fake
// service. Like runChannel it swaps the package globals, so a test that
// uses it must not call t.Parallel.
func runChannelOn(t *testing.T, cs *channelServices, args ...string) (string, error) {
	t.Helper()
	var cli CLI
	parser, err := kong.New(&cli, kong.Name("rela"), kong.Vars{"version": Version})
	require.NoError(t, err)
	ktx, err := parser.Parse(args)
	if err != nil {
		return "", err
	}
	buf := new(bytes.Buffer)
	prevOut := out
	out = output.NewWithWriter(buf, output.Format(cli.Output))
	t.Cleanup(func() { out = prevOut })
	ctx := principal.With(context.Background(), principal.Principal{User: "alice", Tool: principal.ToolCLI})
	ktx.BindTo(ctx, (*context.Context)(nil))
	ktx.Bind(out, cs)
	err = ktx.Run()
	return buf.String(), err
}
