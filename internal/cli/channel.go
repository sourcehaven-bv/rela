package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"

	"github.com/Sourcehaven-BV/rela/internal/output"
	"github.com/Sourcehaven-BV/rela/internal/principal"
	"github.com/Sourcehaven-BV/rela/internal/realtime"
	"github.com/Sourcehaven-BV/rela/internal/store"
)

// ChannelCmd groups the realtime channel verbs.
//
// The CLI opens the project itself; it is not a client of a running server.
// Every call goes through the project's channel service, which applies
// acl.yaml for the CLI's principal as `rela lease` does. A signal lives only
// in the process that receives it, so the CLI posts log messages only.
type ChannelCmd struct {
	List   ChannelListCmd   `cmd:"" help:"List the channels on an entity that you may read."`
	Read   ChannelReadCmd   `cmd:"" help:"Read a channel: its state, what you may post, and a page of messages."`
	Post   ChannelPostCmd   `cmd:"" help:"Post a message to a channel."`
	Edit   ChannelEditCmd   `cmd:"" help:"Replace the props of your own message."`
	Remove ChannelRemoveCmd `cmd:"" help:"Withdraw your own message, or remove another as a moderator."`
	Create ChannelCreateCmd `cmd:"" help:"Create a named channel of a type with many channels per entity."`
	Watch  ChannelWatchCmd  `cmd:"" help:"Wait for new, edited or removed messages in a channel."`
}

// channelReader reads channels. The wiring site passes the project's
// *realtime.Service, which authorizes every call.
type channelReader interface {
	Enabled() bool
	ListAnchor(ctx context.Context, entityType, anchor string, state realtime.State) (realtime.AnchorChannels, error)
	Read(ctx context.Context, addr realtime.RawAddress, req realtime.ReadRequest) (realtime.ReadView, error)
}

// channelWriter writes messages.
type channelWriter interface {
	Post(ctx context.Context, addr realtime.RawAddress, in realtime.PostInput) (realtime.PostResult, error)
	Edit(ctx context.Context, addr realtime.RawAddress, seq int64, props map[string]any) (realtime.MessageView, error)
	Delete(ctx context.Context, addr realtime.RawAddress, seq int64, reason string) (realtime.MessageView, error)
}

// channelCreator creates named channels.
type channelCreator interface {
	Create(ctx context.Context, entityType, anchor, typ string, in realtime.CreateInput) (realtime.ChannelView, error)
}

// channelWaker wakes a watch when a channel in this process changes.
type channelWaker interface {
	Subscribe() *realtime.Subscription
}

// channelService is what the channel commands use, together.
type channelService interface {
	channelReader
	channelWriter
	channelCreator
	channelWaker
}

// channelServices is the binding the channel commands take.
//
// Nil Svc: accepted; the project has no channel service, and every command
// answers that channels are not enabled.
type channelServices struct {
	Svc channelService
}

const (
	// channelFailedText answers an unexpected channel error; the cause is
	// logged.
	channelFailedText = "channel request failed"
	// channelDefaultLimit and channelMaxLimit bound read --limit. The
	// service caps a page by its own limit too.
	channelDefaultLimit = 20
	channelMaxLimit     = 200
)

// channelTextError shows a channel error as its user text and keeps the
// error in its chain for errors.Is.
type channelTextError struct {
	text string
	err  error
}

func (e *channelTextError) Error() string { return e.text }
func (e *channelTextError) Unwrap() error { return e.err }

// channelStdin is where --props-json - reads; a variable so tests can feed
// it.
var channelStdin io.Reader = os.Stdin

// svc returns the channel service, or the "not enabled" answer when the
// project has none or realtime.yaml is not loaded.
func (c *channelServices) svc() (channelService, error) {
	if c == nil || c.Svc == nil || !c.Svc.Enabled() {
		return nil, &channelTextError{text: realtime.OffText, err: realtime.ErrRealtimeOff}
	}
	return c.Svc, nil
}

// channelFailed answers a failed channel call. An anchor miss gets the text
// `rela show` gives for the same id, so a channel command tells a hidden
// entity from a missing one no better than a read does. A refusal gets its
// [realtime.UserText], with [realtime.NoPrincipalHint] for no_principal.
// Anything else is logged with the caller's user and tool, never message
// content, and answered generically.
func channelFailed(ctx context.Context, op, id string, err error) error {
	if errors.Is(err, realtime.ErrAnchorNotFound) {
		return classifyReadError(id, store.ErrNotFound)
	}
	if text, ok := realtime.UserText(err); ok {
		if realtime.IsNoPrincipal(err) {
			text += realtime.NoPrincipalHint
		}
		return &channelTextError{text: printable(text), err: err}
	}
	p := principal.From(ctx)
	slog.Warn("cli: channel operation failed", "op", op, "user", p.User, "tool", p.Tool, "entity", id, "err", err)
	return errors.New(channelFailedText)
}

// channelAddr is the channel address every command but list takes.
type channelAddr struct {
	ID   string `arg:"" help:"Entity ID, or ID@face for a channel type that anchors to a face."`
	Type string `arg:"" help:"Channel type, from rela channel list."`
	Name string `help:"Channel name; only for a type with many channels."`
}

// raw is the address for the service. It names no entity type: the
// anchor's own type decides.
func (a channelAddr) raw() realtime.RawAddress {
	return realtime.RawAddress{Anchor: strings.TrimSpace(a.ID), Type: a.Type, Name: a.Name}
}

// channelProps are the props flags of post and edit.
type channelProps struct {
	Props     []string `short:"P" name:"prop" sep:"none" placeholder:"KEY=VALUE" help:"A message property (repeatable). The value is read as the property's declared type; commas are kept."`
	PropsJSON string   `name:"props-json" placeholder:"JSON|@FILE|-" help:"All message properties as one JSON object: inline, from a file (@path) or from stdin (-)."`
}

// set reports whether any props flag was given.
func (p channelProps) set() bool { return len(p.Props) > 0 || p.PropsJSON != "" }

// collect builds the props object. --props-json is taken as is; -P values
// are coerced by the declared types in schema (nil reads every value as a
// string). The service validates the result.
func (p channelProps) collect(schema map[string]string) (map[string]any, error) {
	if len(p.Props) > 0 && p.PropsJSON != "" {
		return nil, errors.New("use -P or --props-json, not both")
	}
	if p.PropsJSON != "" {
		return readPropsJSON(p.PropsJSON)
	}
	props := make(map[string]any, len(p.Props))
	for _, kv := range p.Props {
		key, value, ok := strings.Cut(kv, "=")
		if !ok || key == "" {
			return nil, fmt.Errorf("-P %q: want key=value", kv)
		}
		if _, dup := props[key]; dup {
			return nil, fmt.Errorf("-P %s: given twice", key)
		}
		v, err := coerceProp(schema[key], value)
		if err != nil {
			return nil, fmt.Errorf("-P %s: %w", key, err)
		}
		props[key] = v
	}
	return props, nil
}

// coerceProp reads value as a property of the JSON Schema type typ.
func coerceProp(typ, value string) (any, error) {
	switch typ {
	case "integer":
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return nil, errors.New("want a whole number")
		}
		return n, nil
	case "boolean":
		b, err := strconv.ParseBool(value)
		if err != nil {
			return nil, errors.New("want true or false")
		}
		return b, nil
	}
	return value, nil
}

// maxPropsJSON bounds what --props-json reads from a file or stdin. The
// service limits a message far below this.
const maxPropsJSON = 1 << 20

// readPropsJSON reads --props-json: inline JSON, @file or - for stdin.
func readPropsJSON(arg string) (map[string]any, error) {
	data := []byte(arg)
	switch {
	case arg == "-":
		b, err := io.ReadAll(io.LimitReader(channelStdin, maxPropsJSON+1))
		if err != nil {
			return nil, fmt.Errorf("--props-json: read stdin: %w", err)
		}
		data = b
	case strings.HasPrefix(arg, "@"):
		f, err := os.Open(arg[1:])
		if err != nil {
			return nil, fmt.Errorf("--props-json: %w", err)
		}
		defer f.Close()
		b, err := io.ReadAll(io.LimitReader(f, maxPropsJSON+1))
		if err != nil {
			return nil, fmt.Errorf("--props-json: %w", err)
		}
		data = b
	}
	if len(data) > maxPropsJSON {
		return nil, fmt.Errorf("--props-json: larger than %d bytes", maxPropsJSON)
	}
	var props map[string]any
	if err := json.Unmarshal(data, &props); err != nil || props == nil {
		return nil, errors.New("--props-json: want a JSON object")
	}
	return props, nil
}

// declaredTypes returns the declared property types of message type typ
// in view, as JSON Schema type names. -P needs them to coerce. A type the
// read does not list yields no types, and the service refuses the write.
func declaredTypes(view realtime.ReadView, typ string) map[string]string {
	for _, mt := range view.MessageTypes {
		if mt.Name != typ {
			continue
		}
		types := make(map[string]string, len(mt.Props.Properties))
		for name, p := range mt.Props.Properties {
			types[name] = p.Type
		}
		return types
	}
	return map[string]string{}
}

// messageProps collects the props of a post or edit. Only -P needs the
// declared types; it gets them from one read of the page req selects, with
// the schemas, and typeOf names the message type on that page. The read is
// one a caller who may write can make anyway.
func messageProps(
	ctx context.Context, svc channelReader, addr realtime.RawAddress, p channelProps,
	req realtime.ReadRequest, typeOf func(realtime.ReadView) string,
) (map[string]any, error) {
	if len(p.Props) == 0 || p.PropsJSON != "" {
		return p.collect(nil)
	}
	req.Schemas, req.LogOnly = true, true
	view, err := svc.Read(ctx, addr, req)
	if err != nil {
		return nil, channelFailed(ctx, "read", addr.Anchor, err)
	}
	return p.collect(declaredTypes(view, typeOf(view)))
}

// ChannelListCmd lists the channels on an entity.
type ChannelListCmd struct {
	ID    string `arg:"" help:"Entity ID, or ID@face to include the channels of that face."`
	State string `help:"Only channels in this state." enum:",open,closed,archived" default:""`
}

// Run lists the channels.
func (c *ChannelListCmd) Run(ctx context.Context, cs *channelServices) error {
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	id := strings.TrimSpace(c.ID)
	got, err := svc.ListAnchor(ctx, "", id, realtime.State(c.State))
	if err != nil {
		return channelFailed(ctx, "list", id, err)
	}
	if out.Format == output.FormatJSON {
		return writeChannelJSON(struct {
			Notice string `json:"notice"`
			realtime.AnchorChannels
		}{realtime.UntrustedNotice, got})
	}
	return renderChannelList(out.Out, got)
}

// ChannelReadCmd reads a channel.
type ChannelReadCmd struct {
	channelAddr
	Limit    int    `help:"Page size, 1 to 200." default:"20"`
	Before   *int64 `help:"Messages before this seq."`
	After    *int64 `help:"Messages after this seq, oldest first."`
	AfterRev *int64 `name:"after-rev" help:"Messages changed after this rev, ordered by rev; 0 is valid."`
	Schemas  bool   `help:"Show each message type's properties."`
}

// Run reads the channel.
func (c *ChannelReadCmd) Run(ctx context.Context, cs *channelServices) error {
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	req, err := c.request()
	if err != nil {
		return err
	}
	addr := c.raw()
	view, err := svc.Read(ctx, addr, req)
	if err != nil {
		return channelFailed(ctx, "read", addr.Anchor, err)
	}
	if out.Format == output.FormatJSON {
		return writeChannelJSON(readJSON{Notice: realtime.UntrustedNotice, ReadView: view})
	}
	return renderRead(out.Out, view)
}

// request builds the read request from the flags.
func (c *ChannelReadCmd) request() (realtime.ReadRequest, error) {
	req := realtime.ReadRequest{Schemas: c.Schemas, Actions: true, LogOnly: true}
	if c.Limit < 1 || c.Limit > channelMaxLimit {
		return req, fmt.Errorf("--limit: 1 to %d", channelMaxLimit)
	}
	req.Limit = c.Limit
	cursors := 0
	for _, f := range []struct {
		name string
		v    *int64
		dst  *int64
	}{{"--before", c.Before, &req.BeforeSeq}, {"--after", c.After, &req.AfterSeq}} {
		if f.v == nil {
			continue
		}
		if *f.v < 1 {
			return req, fmt.Errorf("%s: a seq of at least 1", f.name)
		}
		*f.dst = *f.v
		cursors++
	}
	if c.AfterRev != nil {
		if *c.AfterRev < 0 {
			return req, errors.New("--after-rev: a rev of at least 0")
		}
		req.AfterRev = c.AfterRev
		cursors++
	}
	if cursors > 1 {
		return req, errors.New("name at most one of --before, --after and --after-rev")
	}
	return req, nil
}

// readJSON is the JSON document of read and of each watch change.
type readJSON struct {
	Notice string `json:"notice"`
	realtime.ReadView
}

// messageJSON is the JSON document of post, edit and remove.
type messageJSON struct {
	Notice  string               `json:"notice"`
	Message realtime.MessageView `json:"message"`
	// Session is the channel's session after a post, for a channel type
	// with a session.
	Session *realtime.SessionView `json:"session,omitempty"`
	// Outputs is the outputs mark of the move a post caused.
	Outputs *realtime.OutputsView `json:"outputs,omitempty"`
}

// ChannelPostCmd posts a message.
type ChannelPostCmd struct {
	channelAddr
	MessageType string `arg:"" name:"message-type" help:"Message type, from rela channel read."`
	channelProps
	ReplyTo int64 `name:"reply-to" help:"Seq of the top-level message to reply to."`
}

// Run posts the message.
func (c *ChannelPostCmd) Run(ctx context.Context, cs *channelServices) error {
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	if c.ReplyTo < 0 {
		return errors.New("--reply-to: a seq of at least 1")
	}
	addr := c.raw()
	props, err := messageProps(ctx, svc, addr, c.channelProps, realtime.ReadRequest{Limit: 1},
		func(realtime.ReadView) string { return c.MessageType })
	if err != nil {
		return err
	}
	got, err := svc.Post(ctx, addr, realtime.PostInput{
		Type: c.MessageType, Props: props, ReplyTo: c.ReplyTo, LogOnly: true,
	})
	if err != nil {
		return channelFailed(ctx, "post", addr.Anchor, err)
	}
	if got.Message == nil { // coverage-ignore: defensive: a log-only post always answers its message
		return errors.New(channelFailedText)
	}
	return writeMessage(*got.Message, "posted", got.Session, outputsLine{got.Outputs, addr.Anchor})
}

// ChannelEditCmd edits a message.
type ChannelEditCmd struct {
	channelAddr
	Seq int64 `arg:"" help:"Message seq."`
	channelProps
}

// Run edits the message. Edit replaces the props, so it needs at least one
// props flag.
func (c *ChannelEditCmd) Run(ctx context.Context, cs *channelServices) error {
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	if c.Seq < 1 {
		return errors.New("seq: at least 1")
	}
	if !c.set() {
		return errors.New("edit replaces the message props: give -P or --props-json")
	}
	addr := c.raw()
	// The page that ends at the message gives its type and every schema.
	props, err := messageProps(ctx, svc, addr, c.channelProps, realtime.ReadRequest{BeforeSeq: c.Seq + 1, Limit: 1},
		func(view realtime.ReadView) string { return messageType(view, c.Seq) })
	if err != nil {
		return err
	}
	got, err := svc.Edit(ctx, addr, c.Seq, props)
	if err != nil {
		return channelFailed(ctx, "edit", addr.Anchor, err)
	}
	return writeMessage(got, "edited", nil, outputsLine{})
}

// messageType returns the type of message seq in view, so edit can coerce
// -P by it. A message the read does not return yields "", and the service
// answers for the edit.
func messageType(view realtime.ReadView, seq int64) string {
	for _, m := range view.Messages {
		if m.Seq == seq {
			return m.Type
		}
	}
	return ""
}

// ChannelRemoveCmd removes a message.
type ChannelRemoveCmd struct {
	channelAddr
	Seq    int64  `arg:"" help:"Message seq."`
	Reason string `help:"Why; shown to moderators. Some channels require it for a moderator removal."`
}

// Run removes the message.
func (c *ChannelRemoveCmd) Run(ctx context.Context, cs *channelServices) error {
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	if c.Seq < 1 {
		return errors.New("seq: at least 1")
	}
	addr := c.raw()
	got, err := svc.Delete(ctx, addr, c.Seq, c.Reason)
	if err != nil {
		return channelFailed(ctx, "remove", addr.Anchor, err)
	}
	return writeMessage(got, "removed", nil, outputsLine{})
}

// ChannelCreateCmd creates a named channel.
type ChannelCreateCmd struct {
	ID    string `arg:"" help:"Entity ID, or ID@face for a channel type that anchors to a face."`
	Type  string `arg:"" help:"Channel type, from rela channel list."`
	Title string `help:"Channel title."`
}

// Run creates the channel.
func (c *ChannelCreateCmd) Run(ctx context.Context, cs *channelServices) error {
	svc, err := cs.svc()
	if err != nil {
		return err
	}
	id := strings.TrimSpace(c.ID)
	got, err := svc.Create(ctx, "", id, c.Type, realtime.CreateInput{Title: c.Title})
	if err != nil {
		return channelFailed(ctx, "create", id, err)
	}
	if out.Format == output.FormatJSON {
		return writeChannelJSON(struct {
			Notice  string               `json:"notice"`
			Channel realtime.ChannelView `json:"channel"`
		}{realtime.UntrustedNotice, got})
	}
	return renderCreated(out.Out, got)
}

// outputsLine is the outputs mark of a post's move, and the anchor its
// sets wrote.
type outputsLine struct {
	view   *realtime.OutputsView
	anchor string
}

// writeMessage prints the result of post, edit or remove.
func writeMessage(m realtime.MessageView, verb string, session *realtime.SessionView, outputs outputsLine) error {
	if out.Format == output.FormatJSON {
		return writeChannelJSON(messageJSON{
			Notice: realtime.UntrustedNotice, Message: m, Session: session, Outputs: outputs.view,
		})
	}
	if _, err := fmt.Fprintf(out.Out, "#%d %s\n", m.Seq, verb); err != nil {
		return err
	}
	if session != nil {
		if _, err := fmt.Fprintln(out.Out, sessionText(session)); err != nil {
			return err
		}
	}
	if outputs.view == nil {
		return nil
	}
	_, err := fmt.Fprintln(out.Out, outputsText(outputs.view, outputs.anchor))
	return err
}

// writeChannelJSON prints v as indented JSON.
func writeChannelJSON(v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeJSONLine(data)
}

// writeJSONLine writes one encoded JSON document and a newline to out,
// made safe for a terminal by [terminalSafeJSON].
func writeJSONLine(data []byte) error {
	_, err := out.Out.Write(append(terminalSafeJSON(data), '\n'))
	return err
}
